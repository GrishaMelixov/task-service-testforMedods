package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	infrastructurepostgres "example.com/taskservice/internal/infrastructure/postgres"
	postgresrepo "example.com/taskservice/internal/repository/postgres"
	transporthttp "example.com/taskservice/internal/transport/http"
	swaggerdocs "example.com/taskservice/internal/transport/http/docs"
	httphandlers "example.com/taskservice/internal/transport/http/handlers"
	scheduleusecase "example.com/taskservice/internal/usecase/schedule"
	"example.com/taskservice/internal/usecase/task"
	"example.com/taskservice/internal/worker/generator"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := loadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := infrastructurepostgres.Open(ctx, cfg.DatabaseDSN)
	if err != nil {
		logger.Error("open postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// ── repositories ──────────────────────────────────────────────────────────
	taskRepo := postgresrepo.New(pool)
	scheduleRepo := postgresrepo.NewScheduleRepository(pool)

	// ── use cases ─────────────────────────────────────────────────────────────
	taskUsecase := task.NewService(taskRepo)
	scheduleUsecase := scheduleusecase.NewService(scheduleRepo)

	// ── transport ─────────────────────────────────────────────────────────────
	taskHandler := httphandlers.NewTaskHandler(taskUsecase)
	scheduleHandler := httphandlers.NewScheduleHandler(scheduleUsecase)
	docsHandler := swaggerdocs.NewHandler()
	router := transporthttp.NewRouter(taskHandler, scheduleHandler, docsHandler)

	// ── background worker ─────────────────────────────────────────────────────
	var wg sync.WaitGroup

	if !cfg.GeneratorDisabled {
		gen := generator.New(generator.Config{
			Schedules: scheduleRepo,
			Tasks:     taskRepo,
			Clock:     time.Now,
			Location:  cfg.Timezone,
			Horizon:   time.Duration(cfg.HorizonDays) * 24 * time.Hour,
			TickEvery: cfg.GeneratorInterval,
			Log:       logger,
		})

		wg.Add(1)
		go func() {
			defer wg.Done()
			gen.Run(ctx)
		}()
	}

	// ── HTTP server ───────────────────────────────────────────────────────────
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown http server", "error", err)
		}
	}()

	logger.Info("http server started", "addr", cfg.HTTPAddr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("listen and serve", "error", err)
		os.Exit(1)
	}

	// Wait for the generator to finish its current cycle before exiting.
	wg.Wait()
}

type config struct {
	HTTPAddr          string
	DatabaseDSN       string
	Timezone          *time.Location
	HorizonDays       int
	GeneratorInterval time.Duration
	GeneratorDisabled bool
}

func loadConfig() config {
	cfg := config{
		HTTPAddr:    envOrDefault("HTTP_ADDR", ":8080"),
		DatabaseDSN: envOrDefault("DATABASE_DSN", "postgres://postgres:postgres@localhost:5432/taskservice?sslmode=disable"),
	}

	if cfg.DatabaseDSN == "" {
		panic(fmt.Errorf("DATABASE_DSN is required"))
	}

	// Timezone used for computing day boundaries in the generator.
	tzName := envOrDefault("TASKSERVICE_TIMEZONE", "Europe/Moscow")
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		panic(fmt.Errorf("TASKSERVICE_TIMEZONE %q is not a valid IANA timezone: %w", tzName, err))
	}
	cfg.Timezone = loc

	// How many days ahead to materialise tasks.
	cfg.HorizonDays = 30
	if v := os.Getenv("GENERATOR_HORIZON_DAYS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			panic(fmt.Errorf("GENERATOR_HORIZON_DAYS must be a positive integer, got %q", v))
		}
		cfg.HorizonDays = n
	}

	// How often the generator runs.
	cfg.GeneratorInterval = time.Hour
	if v := os.Getenv("GENERATOR_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			panic(fmt.Errorf("GENERATOR_INTERVAL must be a positive duration (e.g. 1h), got %q", v))
		}
		cfg.GeneratorInterval = d
	}

	// Escape-hatch to disable the generator for local debugging.
	cfg.GeneratorDisabled = os.Getenv("GENERATOR_DISABLED") == "true"

	return cfg
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
