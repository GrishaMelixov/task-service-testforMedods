// Package schedule contains the recurring-task domain: the Schedule entity,
// its kinds and parameters, validation rules and a pure Occurrences function
// that answers "which dates does this schedule fall on inside a given window".
//
// The package has no dependencies on persistence, HTTP or the clock; it only
// uses the standard library. All other layers (usecase, repository, worker)
// depend on this package, never the other way around.
package schedule

import (
	"encoding/json"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

// Kind enumerates the supported recurrence strategies.
type Kind string

const (
	// KindDailyEveryN fires every N days, anchored on StartDate.
	KindDailyEveryN Kind = "daily_every_n"
	// KindMonthlyDays fires on the listed days of every month (1..30).
	KindMonthlyDays Kind = "monthly_days"
	// KindSpecificDates fires only on the explicit list of calendar dates.
	KindSpecificDates Kind = "specific_dates"
	// KindEvenOdd fires on even-only or odd-only days of each month.
	KindEvenOdd Kind = "even_odd"
)

// Valid reports whether k is a recognised recurrence kind.
func (k Kind) Valid() bool {
	switch k {
	case KindDailyEveryN, KindMonthlyDays, KindSpecificDates, KindEvenOdd:
		return true
	default:
		return false
	}
}

// DefaultTimezone is used when a schedule is created without an explicit timezone.
// The service is intended for a single clinic operating in Moscow time; a per-schedule
// override is still allowed through the Timezone field.
const DefaultTimezone = "Europe/Moscow"

// Parity distinguishes even and odd days of the month for KindEvenOdd.
type Parity string

const (
	ParityEven Parity = "even"
	ParityOdd  Parity = "odd"
)

// Valid reports whether p is a recognised parity value.
func (p Parity) Valid() bool {
	return p == ParityEven || p == ParityOdd
}

// Schedule is a recurrence template. Concrete Task instances are materialised
// from a Schedule by the generator worker.
type Schedule struct {
	ID            int64
	Title         string
	Description   string
	DefaultStatus taskdomain.Status

	Kind   Kind
	Params json.RawMessage // typed per Kind; see package validate.

	// StartDate is the first date at which the schedule can fire (inclusive,
	// interpreted in Timezone). For KindDailyEveryN it also acts as the anchor
	// of the "every N days" sequence.
	StartDate time.Time
	// EndDate is an optional upper bound (inclusive, interpreted in Timezone).
	// A nil value means "no end".
	EndDate *time.Time
	// Timezone is an IANA zone identifier. Empty is treated as DefaultTimezone.
	Timezone string
	// Active controls whether the generator considers this schedule during
	// a materialisation cycle. Inactive schedules keep their history but stop
	// producing new instances.
	Active bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

// DailyEveryNParams is the typed shape of Schedule.Params for KindDailyEveryN.
type DailyEveryNParams struct {
	N int `json:"n"`
}

// MonthlyDaysParams is the typed shape of Schedule.Params for KindMonthlyDays.
// Days are days of the month in the range [1, 30].
type MonthlyDaysParams struct {
	Days []int `json:"days"`
}

// SpecificDatesParams is the typed shape of Schedule.Params for KindSpecificDates.
// Dates are interpreted as calendar dates in Schedule.Timezone.
type SpecificDatesParams struct {
	Dates []time.Time `json:"dates"`
}

// EvenOddParams is the typed shape of Schedule.Params for KindEvenOdd.
type EvenOddParams struct {
	Parity Parity `json:"parity"`
}

// Location returns the schedule's effective *time.Location, falling back to
// DefaultTimezone when the Timezone field is empty.
func (s Schedule) Location() (*time.Location, error) {
	tz := s.Timezone
	if tz == "" {
		tz = DefaultTimezone
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, ErrInvalidTimezone
	}

	return loc, nil
}
