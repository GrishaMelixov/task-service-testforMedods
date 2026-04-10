package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	scheduledomain "example.com/taskservice/internal/domain/schedule"
	taskdomain "example.com/taskservice/internal/domain/task"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		INSERT INTO tasks (title, description, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, title, description, status, schedule_id, due_date, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query, task.Title, task.Description, task.Status, task.CreatedAt, task.UpdatedAt)
	created, err := scanTask(row)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, schedule_id, due_date, created_at, updated_at
		FROM tasks
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	found, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return found, nil
}

func (r *Repository) Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		UPDATE tasks
		SET title = $1,
			description = $2,
			status = $3,
			updated_at = $4
		WHERE id = $5
		RETURNING id, title, description, status, schedule_id, due_date, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query, task.Title, task.Description, task.Status, task.UpdatedAt, task.ID)
	updated, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return updated, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM tasks WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}

	return nil
}

// List returns all tasks ordered by id DESC. Kept for backward-compatibility
// with the existing Usecase interface; new callers should use ListByFilter.
func (r *Repository) List(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, schedule_id, due_date, created_at, updated_at
		FROM tasks
		ORDER BY id DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]taskdomain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, *task)
	}

	return tasks, rows.Err()
}

// ListByFilter returns tasks matching the given predicates, ordered by due_date
// ASC (nulls last), then id DESC. All filter fields are optional.
func (r *Repository) ListByFilter(ctx context.Context, f TaskFilter) ([]taskdomain.Task, error) {
	query := `
		SELECT id, title, description, status, schedule_id, due_date, created_at, updated_at
		FROM tasks
		WHERE TRUE
	`
	args := make([]any, 0, 4)
	i := 1

	if f.ScheduleID != nil {
		query += fmt.Sprintf(" AND schedule_id = $%d", i)
		args = append(args, *f.ScheduleID)
		i++
	}
	if f.From != nil {
		query += fmt.Sprintf(" AND due_date >= $%d", i)
		args = append(args, f.From.Format("2006-01-02"))
		i++
	}
	if f.To != nil {
		query += fmt.Sprintf(" AND due_date <= $%d", i)
		args = append(args, f.To.Format("2006-01-02"))
		i++
	}
	if f.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", i)
		args = append(args, *f.Status)
		i++
	}

	query += ` ORDER BY due_date ASC NULLS LAST, id DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]taskdomain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *task)
	}

	return tasks, rows.Err()
}

// CreateFromSchedule inserts a task derived from a schedule occurrence.
// It is idempotent: if a task for this (schedule_id, due_date) already exists
// the existing row is returned unchanged (ON CONFLICT DO NOTHING).
// Returns (nil, nil) when the task already existed — callers should treat this
// as a no-op, not an error.
func (r *Repository) CreateFromSchedule(ctx context.Context, s *scheduledomain.Schedule, dueDate time.Time) (*taskdomain.Task, error) {
	const query = `
		INSERT INTO tasks (title, description, status, schedule_id, due_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (schedule_id, due_date) WHERE schedule_id IS NOT NULL DO NOTHING
		RETURNING id, title, description, status, schedule_id, due_date, created_at, updated_at
	`

	now := time.Now().UTC()
	dateOnly := dueDate.Format("2006-01-02")

	row := r.pool.QueryRow(ctx, query,
		s.Title, s.Description, string(s.DefaultStatus),
		s.ID, dateOnly,
		now, now,
	)

	task, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// ON CONFLICT DO NOTHING — task already existed.
			return nil, nil
		}
		return nil, err
	}

	return task, nil
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner taskScanner) (*taskdomain.Task, error) {
	var (
		task       taskdomain.Task
		status     string
		scheduleID *int64
		dueDate    *time.Time
	)

	if err := scanner.Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&status,
		&scheduleID,
		&dueDate,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		return nil, err
	}

	task.Status = taskdomain.Status(status)
	task.ScheduleID = scheduleID
	task.DueDate = dueDate

	return &task, nil
}
