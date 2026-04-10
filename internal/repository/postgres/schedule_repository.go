package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	scheduledomain "example.com/taskservice/internal/domain/schedule"
	taskdomain "example.com/taskservice/internal/domain/task"
)

type ScheduleRepository struct {
	pool *pgxpool.Pool
}

func NewScheduleRepository(pool *pgxpool.Pool) *ScheduleRepository {
	return &ScheduleRepository{pool: pool}
}

func (r *ScheduleRepository) Create(ctx context.Context, s *scheduledomain.Schedule) (*scheduledomain.Schedule, error) {
	const query = `
		INSERT INTO schedules
			(title, description, default_status, kind, params, start_date, end_date, timezone, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, title, description, default_status, kind, params, start_date, end_date, timezone, active, created_at, updated_at
	`
	row := r.pool.QueryRow(ctx, query,
		s.Title, s.Description, string(s.DefaultStatus), string(s.Kind),
		[]byte(s.Params),
		s.StartDate, s.EndDate,
		s.Timezone, s.Active,
		s.CreatedAt, s.UpdatedAt,
	)
	return scanSchedule(row)
}

func (r *ScheduleRepository) GetByID(ctx context.Context, id int64) (*scheduledomain.Schedule, error) {
	const query = `
		SELECT id, title, description, default_status, kind, params, start_date, end_date, timezone, active, created_at, updated_at
		FROM schedules WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, id)
	s, err := scanSchedule(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, scheduledomain.ErrNotFound
		}
		return nil, err
	}
	return s, nil
}

func (r *ScheduleRepository) Update(ctx context.Context, s *scheduledomain.Schedule) (*scheduledomain.Schedule, error) {
	const query = `
		UPDATE schedules
		SET title = $1, description = $2, default_status = $3, kind = $4, params = $5,
		    start_date = $6, end_date = $7, timezone = $8, active = $9, updated_at = $10
		WHERE id = $11
		RETURNING id, title, description, default_status, kind, params, start_date, end_date, timezone, active, created_at, updated_at
	`
	row := r.pool.QueryRow(ctx, query,
		s.Title, s.Description, string(s.DefaultStatus), string(s.Kind),
		[]byte(s.Params),
		s.StartDate, s.EndDate,
		s.Timezone, s.Active, s.UpdatedAt,
		s.ID,
	)
	updated, err := scanSchedule(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, scheduledomain.ErrNotFound
		}
		return nil, err
	}
	return updated, nil
}

func (r *ScheduleRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM schedules WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return scheduledomain.ErrNotFound
	}
	return nil
}

func (r *ScheduleRepository) List(ctx context.Context) ([]scheduledomain.Schedule, error) {
	const query = `
		SELECT id, title, description, default_status, kind, params, start_date, end_date, timezone, active, created_at, updated_at
		FROM schedules ORDER BY id DESC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []scheduledomain.Schedule
	for rows.Next() {
		s, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func (r *ScheduleRepository) ListActive(ctx context.Context) ([]scheduledomain.Schedule, error) {
	const query = `
		SELECT id, title, description, default_status, kind, params, start_date, end_date, timezone, active, created_at, updated_at
		FROM schedules WHERE active = TRUE ORDER BY id
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []scheduledomain.Schedule
	for rows.Next() {
		s, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func (r *ScheduleRepository) SetActive(ctx context.Context, id int64, active bool) error {
	result, err := r.pool.Exec(ctx,
		`UPDATE schedules SET active = $1, updated_at = $2 WHERE id = $3`,
		active, time.Now().UTC(), id,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return scheduledomain.ErrNotFound
	}
	return nil
}

type scheduleScanner interface {
	Scan(dest ...any) error
}

func scanSchedule(sc scheduleScanner) (*scheduledomain.Schedule, error) {
	var (
		s             scheduledomain.Schedule
		defaultStatus string
		kind          string
		params        []byte
		endDate       *time.Time
	)

	if err := sc.Scan(
		&s.ID,
		&s.Title,
		&s.Description,
		&defaultStatus,
		&kind,
		&params,
		&s.StartDate,
		&endDate,
		&s.Timezone,
		&s.Active,
		&s.CreatedAt,
		&s.UpdatedAt,
	); err != nil {
		return nil, err
	}

	s.DefaultStatus = taskdomain.Status(defaultStatus)
	s.Kind = scheduledomain.Kind(kind)
	s.Params = json.RawMessage(params)
	s.EndDate = endDate

	return &s, nil
}
