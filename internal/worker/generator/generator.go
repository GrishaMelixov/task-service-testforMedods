// Package generator contains the background worker that materialises Schedule
// occurrences into Task rows on a configurable tick cadence.
//
// Design notes:
//   - The worker fires once immediately on startup to catch up the horizon from
//     "today" (not a back-fill: missed dates before today are not generated).
//   - Idempotency is guaranteed by the DB UNIQUE INDEX on (schedule_id, due_date)
//     combined with INSERT … ON CONFLICT DO NOTHING in the repository.
//   - A single per-cycle error log is emitted for any schedule that fails; the
//     cycle continues for the remaining schedules so a single bad schedule cannot
//     starve the rest.
//   - The worker depends on two small interfaces (ScheduleLister, TaskMaterialiser)
//     rather than the full repository types, making it straightforward to test
//     with fakes without spinning up a database.
package generator

import (
	"context"
	"log/slog"
	"time"

	scheduledomain "example.com/taskservice/internal/domain/schedule"
)

// ScheduleLister is the subset of the schedule repository used by the generator.
type ScheduleLister interface {
	ListActive(ctx context.Context) ([]scheduledomain.Schedule, error)
}

// TaskMaterialiser is the subset of the task repository used by the generator.
type TaskMaterialiser interface {
	// CreateFromSchedule inserts a task for the given schedule and due date.
	// It returns (nil, nil) when the task already exists (ON CONFLICT DO NOTHING).
	CreateFromSchedule(ctx context.Context, s *scheduledomain.Schedule, dueDate time.Time) (any, error)
}

// Generator materialises schedule occurrences into tasks on a periodic basis.
type Generator struct {
	schedules ScheduleLister
	tasks     TaskMaterialiser
	clock     func() time.Time
	loc       *time.Location
	horizon   time.Duration
	tickEvery time.Duration
	log       *slog.Logger
}

// Config holds the constructor arguments for a Generator.
type Config struct {
	Schedules ScheduleLister
	Tasks     TaskMaterialiser
	// Clock is used to determine "now"; defaults to time.Now if nil.
	Clock func() time.Time
	// Location is the clinic timezone used to compute day boundaries.
	Location  *time.Location
	Horizon   time.Duration // how far ahead to materialise; default 30 days
	TickEvery time.Duration // cycle cadence; default 1 hour
	Log       *slog.Logger
}

// New constructs a Generator with the given config, applying sensible defaults.
func New(cfg Config) *Generator {
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	if cfg.Location == nil {
		cfg.Location = time.UTC
	}
	if cfg.Horizon == 0 {
		cfg.Horizon = 30 * 24 * time.Hour
	}
	if cfg.TickEvery == 0 {
		cfg.TickEvery = time.Hour
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	return &Generator{
		schedules: cfg.Schedules,
		tasks:     cfg.Tasks,
		clock:     cfg.Clock,
		loc:       cfg.Location,
		horizon:   cfg.Horizon,
		tickEvery: cfg.TickEvery,
		log:       cfg.Log,
	}
}

// Run starts the materialisation loop. It blocks until ctx is cancelled.
// Call it in a separate goroutine and include its sync.WaitGroup counter in
// the application shutdown sequence.
func (g *Generator) Run(ctx context.Context) {
	g.log.Info("generator started",
		"horizon_days", int(g.horizon.Hours()/24),
		"interval", g.tickEvery.String(),
		"timezone", g.loc.String(),
	)

	// Immediate catch-up on startup.
	g.generateOnce(ctx)

	ticker := time.NewTicker(g.tickEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			g.log.Info("generator stopped")
			return
		case <-ticker.C:
			g.generateOnce(ctx)
		}
	}
}

// generateOnce runs a single materialisation cycle: fetch active schedules,
// compute occurrences for [today, today+horizon], insert missing tasks.
func (g *Generator) generateOnce(ctx context.Context) {
	start := time.Now()
	now := g.clock().In(g.loc)
	from := midnight(now)
	to := from.Add(g.horizon)

	schedules, err := g.schedules.ListActive(ctx)
	if err != nil {
		g.log.Error("generator: list active schedules", "err", err)
		return
	}

	var created, skipped, failed int
	for i := range schedules {
		s := &schedules[i]
		c, sk, f := g.processSchedule(ctx, s, from, to)
		created += c
		skipped += sk
		failed += f
	}

	g.log.Info("generator: cycle done",
		"schedules", len(schedules),
		"created", created,
		"skipped", skipped,
		"failed", failed,
		"duration", time.Since(start).Round(time.Millisecond).String(),
	)
}

// processSchedule materialises the occurrences of a single schedule.
// Returns (created, skipped, failed) counts.
func (g *Generator) processSchedule(ctx context.Context, s *scheduledomain.Schedule, from, to time.Time) (int, int, int) {
	dates, err := scheduledomain.Occurrences(*s, from, to)
	if err != nil {
		g.log.Error("generator: compute occurrences",
			"schedule_id", s.ID,
			"schedule_title", s.Title,
			"err", err,
		)
		return 0, 0, 1
	}

	created, skipped, failed := 0, 0, 0
	for _, d := range dates {
		task, err := g.tasks.CreateFromSchedule(ctx, s, d)
		if err != nil {
			g.log.Error("generator: create task",
				"schedule_id", s.ID,
				"due_date", d.Format("2006-01-02"),
				"err", err,
			)
			failed++
			continue
		}
		if task == nil {
			skipped++ // ON CONFLICT DO NOTHING
		} else {
			created++
		}
	}

	return created, skipped, failed
}

// midnight returns the time at midnight (00:00:00) of the given time in its location.
func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
