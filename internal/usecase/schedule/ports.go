package schedule

import (
	"context"
	"time"

	scheduledomain "example.com/taskservice/internal/domain/schedule"
	taskdomain "example.com/taskservice/internal/domain/task"
)

// Repository is the persistence interface required by the schedule usecase.
// Kept narrow to only what the service actually needs.
type Repository interface {
	Create(ctx context.Context, s *scheduledomain.Schedule) (*scheduledomain.Schedule, error)
	GetByID(ctx context.Context, id int64) (*scheduledomain.Schedule, error)
	Update(ctx context.Context, s *scheduledomain.Schedule) (*scheduledomain.Schedule, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]scheduledomain.Schedule, error)
	SetActive(ctx context.Context, id int64, active bool) error
}

// Usecase is the schedule service interface consumed by the HTTP handler.
type Usecase interface {
	Create(ctx context.Context, input CreateInput) (*scheduledomain.Schedule, error)
	GetByID(ctx context.Context, id int64) (*scheduledomain.Schedule, error)
	Update(ctx context.Context, id int64, input UpdateInput) (*scheduledomain.Schedule, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]scheduledomain.Schedule, error)
	Activate(ctx context.Context, id int64) error
	Deactivate(ctx context.Context, id int64) error
}

// CreateInput carries the validated data for creating a new schedule.
type CreateInput struct {
	Title         string
	Description   string
	DefaultStatus taskdomain.Status
	Kind          scheduledomain.Kind
	RawParams     []byte // validated, normalised JSON
	StartDate     time.Time
	EndDate       *time.Time
	Timezone      string
}

// UpdateInput carries the validated data for replacing a schedule.
type UpdateInput struct {
	Title         string
	Description   string
	DefaultStatus taskdomain.Status
	Kind          scheduledomain.Kind
	RawParams     []byte
	StartDate     time.Time
	EndDate       *time.Time
	Timezone      string
}
