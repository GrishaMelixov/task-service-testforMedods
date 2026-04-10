package schedule_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	scheduledomain "example.com/taskservice/internal/domain/schedule"
	taskdomain "example.com/taskservice/internal/domain/task"
	scheduleusecase "example.com/taskservice/internal/usecase/schedule"
)

// fakeRepo is a simple in-memory schedule repository for testing.
type fakeRepo struct {
	schedules map[int64]*scheduledomain.Schedule
	nextID    int64
	failNext  error // if set, every method returns this error once
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{schedules: make(map[int64]*scheduledomain.Schedule)}
}

func (r *fakeRepo) mayFail() error {
	if r.failNext != nil {
		err := r.failNext
		r.failNext = nil
		return err
	}
	return nil
}

func (r *fakeRepo) Create(_ context.Context, s *scheduledomain.Schedule) (*scheduledomain.Schedule, error) {
	if err := r.mayFail(); err != nil {
		return nil, err
	}
	r.nextID++
	cp := *s
	cp.ID = r.nextID
	r.schedules[cp.ID] = &cp
	return &cp, nil
}

func (r *fakeRepo) GetByID(_ context.Context, id int64) (*scheduledomain.Schedule, error) {
	if err := r.mayFail(); err != nil {
		return nil, err
	}
	s, ok := r.schedules[id]
	if !ok {
		return nil, scheduledomain.ErrNotFound
	}
	cp := *s
	return &cp, nil
}

func (r *fakeRepo) Update(_ context.Context, s *scheduledomain.Schedule) (*scheduledomain.Schedule, error) {
	if err := r.mayFail(); err != nil {
		return nil, err
	}
	if _, ok := r.schedules[s.ID]; !ok {
		return nil, scheduledomain.ErrNotFound
	}
	cp := *s
	r.schedules[cp.ID] = &cp
	return &cp, nil
}

func (r *fakeRepo) Delete(_ context.Context, id int64) error {
	if err := r.mayFail(); err != nil {
		return err
	}
	if _, ok := r.schedules[id]; !ok {
		return scheduledomain.ErrNotFound
	}
	delete(r.schedules, id)
	return nil
}

func (r *fakeRepo) List(_ context.Context) ([]scheduledomain.Schedule, error) {
	if err := r.mayFail(); err != nil {
		return nil, err
	}
	out := make([]scheduledomain.Schedule, 0, len(r.schedules))
	for _, s := range r.schedules {
		out = append(out, *s)
	}
	return out, nil
}

func (r *fakeRepo) SetActive(_ context.Context, id int64, active bool) error {
	if err := r.mayFail(); err != nil {
		return err
	}
	s, ok := r.schedules[id]
	if !ok {
		return scheduledomain.ErrNotFound
	}
	s.Active = active
	return nil
}

// helpers

func jsonRaw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func mkDate(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func validInput() scheduleusecase.CreateInput {
	return scheduleusecase.CreateInput{
		Title:         "Обход пациентов",
		DefaultStatus: taskdomain.StatusNew,
		Kind:          scheduledomain.KindDailyEveryN,
		RawParams:     jsonRaw(map[string]any{"n": 1}),
		StartDate:     mkDate(2026, 4, 10),
		Timezone:      "Europe/Moscow",
	}
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestService_Create(t *testing.T) {
	t.Parallel()

	t.Run("creates and returns schedule", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		svc := scheduleusecase.NewService(repo)

		s, err := svc.Create(context.Background(), validInput())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.ID == 0 {
			t.Fatal("expected non-zero ID")
		}
		if !s.Active {
			t.Error("new schedule should be active")
		}
		if s.Title != "Обход пациентов" {
			t.Errorf("title = %q", s.Title)
		}
	})

	t.Run("empty title returns ErrInvalidInput", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		svc := scheduleusecase.NewService(repo)

		input := validInput()
		input.Title = ""
		_, err := svc.Create(context.Background(), input)
		if !errors.Is(err, scheduleusecase.ErrInvalidInput) {
			t.Fatalf("want ErrInvalidInput, got %v", err)
		}
	})

	t.Run("invalid kind returns ErrInvalidInput", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		svc := scheduleusecase.NewService(repo)

		input := validInput()
		input.Kind = "hourly"
		_, err := svc.Create(context.Background(), input)
		if !errors.Is(err, scheduleusecase.ErrInvalidInput) {
			t.Fatalf("want ErrInvalidInput, got %v", err)
		}
	})

	t.Run("invalid params returns ErrInvalidInput", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		svc := scheduleusecase.NewService(repo)

		input := validInput()
		input.RawParams = jsonRaw(map[string]any{"n": -5})
		_, err := svc.Create(context.Background(), input)
		if !errors.Is(err, scheduleusecase.ErrInvalidInput) {
			t.Fatalf("want ErrInvalidInput, got %v", err)
		}
	})

	t.Run("empty timezone defaults to Europe/Moscow", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		svc := scheduleusecase.NewService(repo)

		input := validInput()
		input.Timezone = ""
		s, err := svc.Create(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Timezone != scheduledomain.DefaultTimezone {
			t.Errorf("timezone = %q, want %q", s.Timezone, scheduledomain.DefaultTimezone)
		}
	})
}

func TestService_GetByID(t *testing.T) {
	t.Parallel()

	t.Run("returns existing schedule", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		svc := scheduleusecase.NewService(repo)

		created, _ := svc.Create(context.Background(), validInput())
		got, err := svc.GetByID(context.Background(), created.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != created.ID {
			t.Errorf("got id=%d, want %d", got.ID, created.ID)
		}
	})

	t.Run("non-positive id returns ErrInvalidInput", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		svc := scheduleusecase.NewService(repo)

		_, err := svc.GetByID(context.Background(), 0)
		if !errors.Is(err, scheduleusecase.ErrInvalidInput) {
			t.Fatalf("want ErrInvalidInput, got %v", err)
		}
	})

	t.Run("unknown id returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		svc := scheduleusecase.NewService(repo)

		_, err := svc.GetByID(context.Background(), 999)
		if !errors.Is(err, scheduledomain.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

func TestService_Update(t *testing.T) {
	t.Parallel()

	t.Run("updates and preserves active state", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		svc := scheduleusecase.NewService(repo)

		created, _ := svc.Create(context.Background(), validInput())
		_ = svc.Deactivate(context.Background(), created.ID)

		updateInput := scheduleusecase.UpdateInput{
			Title:         "Updated title",
			DefaultStatus: taskdomain.StatusNew,
			Kind:          scheduledomain.KindDailyEveryN,
			RawParams:     jsonRaw(map[string]any{"n": 2}),
			StartDate:     mkDate(2026, 4, 10),
			Timezone:      "Europe/Moscow",
		}
		updated, err := svc.Update(context.Background(), created.ID, updateInput)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Title != "Updated title" {
			t.Errorf("title not updated")
		}
		// Active should remain false after update.
		if updated.Active {
			t.Error("update must not re-activate a deactivated schedule")
		}
	})
}

func TestService_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes existing schedule", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		svc := scheduleusecase.NewService(repo)

		created, _ := svc.Create(context.Background(), validInput())
		if err := svc.Delete(context.Background(), created.ID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err := svc.GetByID(context.Background(), created.ID)
		if !errors.Is(err, scheduledomain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound after delete, got %v", err)
		}
	})
}

func TestService_ActivateDeactivate(t *testing.T) {
	t.Parallel()

	t.Run("deactivate then activate — idempotent", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		svc := scheduleusecase.NewService(repo)

		created, _ := svc.Create(context.Background(), validInput())

		// Deactivate twice — must not error.
		if err := svc.Deactivate(context.Background(), created.ID); err != nil {
			t.Fatalf("deactivate: %v", err)
		}
		if err := svc.Deactivate(context.Background(), created.ID); err != nil {
			t.Fatalf("second deactivate: %v", err)
		}

		// Re-activate.
		if err := svc.Activate(context.Background(), created.ID); err != nil {
			t.Fatalf("activate: %v", err)
		}

		s, _ := svc.GetByID(context.Background(), created.ID)
		if !s.Active {
			t.Error("expected active=true after Activate")
		}
	})

	t.Run("unknown id returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		svc := scheduleusecase.NewService(repo)

		if err := svc.Activate(context.Background(), 999); !errors.Is(err, scheduledomain.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}
