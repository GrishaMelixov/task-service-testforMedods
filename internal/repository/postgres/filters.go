package postgres

import "time"

// TaskFilter contains optional predicates for the task listing query.
// Zero-value fields are ignored (no filtering on that column).
type TaskFilter struct {
	ScheduleID *int64
	From       *time.Time // due_date >= From
	To         *time.Time // due_date <= To
	Status     *string
}
