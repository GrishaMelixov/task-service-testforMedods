package schedule

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	scheduledomain "example.com/taskservice/internal/domain/schedule"
	taskdomain "example.com/taskservice/internal/domain/task"
)

// Service implements the schedule Usecase.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService constructs a Service with the given repository.
func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*scheduledomain.Schedule, error) {
	sched, err := buildSchedule(input.Title, input.Description, input.DefaultStatus,
		input.Kind, input.RawParams, input.StartDate, input.EndDate, input.Timezone)
	if err != nil {
		return nil, err
	}

	now := s.now()
	sched.Active = true
	sched.CreatedAt = now
	sched.UpdatedAt = now

	return s.repo.Create(ctx, sched)
}

func (s *Service) GetByID(ctx context.Context, id int64) (*scheduledomain.Schedule, error) {
	if err := requirePositiveID(id); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*scheduledomain.Schedule, error) {
	if err := requirePositiveID(id); err != nil {
		return nil, err
	}

	sched, err := buildSchedule(input.Title, input.Description, input.DefaultStatus,
		input.Kind, input.RawParams, input.StartDate, input.EndDate, input.Timezone)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	sched.ID = id
	sched.Active = existing.Active // preserve active state on update
	sched.CreatedAt = existing.CreatedAt
	sched.UpdatedAt = s.now()

	return s.repo.Update(ctx, sched)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := requirePositiveID(id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]scheduledomain.Schedule, error) {
	return s.repo.List(ctx)
}

func (s *Service) Activate(ctx context.Context, id int64) error {
	if err := requirePositiveID(id); err != nil {
		return err
	}
	return s.repo.SetActive(ctx, id, true)
}

func (s *Service) Deactivate(ctx context.Context, id int64) error {
	if err := requirePositiveID(id); err != nil {
		return err
	}
	return s.repo.SetActive(ctx, id, false)
}

// buildSchedule constructs and validates a Schedule from raw input.
func buildSchedule(
	title, description string,
	defaultStatus taskdomain.Status,
	kind scheduledomain.Kind,
	rawParams []byte,
	startDate time.Time,
	endDate *time.Time,
	timezone string,
) (*scheduledomain.Schedule, error) {
	if timezone == "" {
		timezone = scheduledomain.DefaultTimezone
	}

	sched := &scheduledomain.Schedule{
		Title:         strings.TrimSpace(title),
		Description:   description,
		DefaultStatus: defaultStatus,
		Kind:          kind,
		Params:        rawParams,
		StartDate:     startDate,
		EndDate:       endDate,
		Timezone:      timezone,
	}

	if err := scheduledomain.Validate(sched); err != nil {
		if errors.Is(err, scheduledomain.ErrInvalidSchedule) || errors.Is(err, scheduledomain.ErrInvalidParams) {
			return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
		return nil, err
	}

	return sched, nil
}

func requirePositiveID(id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}
	return nil
}
