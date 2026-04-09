package schedule

import "errors"

// ErrNotFound is returned by repositories when a schedule lookup fails.
var ErrNotFound = errors.New("schedule not found")

// ErrInvalidSchedule is a sentinel for whole-schedule validation failures
// (missing fields, inconsistent dates, unknown kind). Wrap it with %w.
var ErrInvalidSchedule = errors.New("invalid schedule")

// ErrInvalidParams is a sentinel for kind-specific parameter validation failures.
// Wrap it with %w for context.
var ErrInvalidParams = errors.New("invalid schedule params")

// ErrInvalidTimezone is returned when a schedule's Timezone cannot be resolved
// by time.LoadLocation.
var ErrInvalidTimezone = errors.New("invalid timezone")
