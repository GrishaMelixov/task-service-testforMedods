package task

import "time"

type Status string

const (
	StatusNew        Status = "new"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

type Task struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// ScheduleID links the task to its recurrence template.
	// Nil for one-off tasks created directly via the API.
	ScheduleID *int64 `json:"schedule_id,omitempty"`
	// DueDate is the calendar date this recurrence instance represents.
	// Nil for one-off tasks.
	DueDate *time.Time `json:"due_date,omitempty"`
}

// ListFilter carries optional predicates for task list queries.
// Zero-value (nil) fields are ignored by the repository.
type ListFilter struct {
	ScheduleID *int64
	From       *time.Time // due_date >= From (date-only comparison)
	To         *time.Time // due_date <= To
	Status     *Status
}

func (s Status) Valid() bool {
	switch s {
	case StatusNew, StatusInProgress, StatusDone:
		return true
	default:
		return false
	}
}
