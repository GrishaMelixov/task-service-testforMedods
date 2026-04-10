package schedule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// Occurrences returns every date on which s fires inside the inclusive
// window [from, to]. Returned values are midnight in s's timezone; callers
// can safely convert them to UTC for persistence.
//
// The window is clamped by the schedule's own StartDate and EndDate. An empty
// slice and a nil error mean "the schedule is valid but does not fire in this
// window"; an error means the schedule itself is malformed.
//
// The function is pure: it performs no IO, does not depend on the wall clock
// and is safe for concurrent use.
func Occurrences(s Schedule, from, to time.Time) ([]time.Time, error) {
	loc, err := s.Location()
	if err != nil {
		return nil, err
	}

	start := dateIn(s.StartDate, loc)

	var end time.Time
	hasEnd := s.EndDate != nil
	if hasEnd {
		end = dateIn(*s.EndDate, loc)
		if end.Before(start) {
			return nil, fmt.Errorf("%w: end_date before start_date", ErrInvalidSchedule)
		}
	}

	lo := dateIn(from, loc)
	hi := dateIn(to, loc)
	if hi.Before(lo) {
		return nil, nil
	}

	// Clamp the window to [StartDate, EndDate].
	if lo.Before(start) {
		lo = start
	}
	if hasEnd && hi.After(end) {
		hi = end
	}
	if hi.Before(lo) {
		return nil, nil
	}

	switch s.Kind {
	case KindDailyEveryN:
		return dailyEveryN(s, start, lo, hi)
	case KindMonthlyDays:
		return monthlyDays(s, lo, hi)
	case KindSpecificDates:
		return specificDates(s, loc, lo, hi)
	case KindEvenOdd:
		return evenOdd(s, lo, hi)
	default:
		return nil, fmt.Errorf("%w: unknown kind %q", ErrInvalidSchedule, s.Kind)
	}
}

// dateIn truncates t to midnight in loc, keeping the calendar date the same
// as the one the caller would read from t in loc.
func dateIn(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func dailyEveryN(s Schedule, start, lo, hi time.Time) ([]time.Time, error) {
	var p DailyEveryNParams
	if err := strictUnmarshal(s.Params, &p); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidParams, err)
	}
	if p.N < 1 {
		return nil, fmt.Errorf("%w: n must be >= 1", ErrInvalidParams)
	}

	// Snap lo forward to the first hit on or after lo, anchored on start.
	days := max(int(lo.Sub(start).Hours()/24), 0)
	offset := days % p.N
	d := lo
	if offset != 0 {
		d = lo.AddDate(0, 0, p.N-offset)
	}

	// Pre-size the slice to avoid repeated growth for long horizons.
	span := int(hi.Sub(d).Hours()/24) + 1
	if span <= 0 {
		return nil, nil
	}
	out := make([]time.Time, 0, span/p.N+1)
	for !d.After(hi) {
		out = append(out, d)
		d = d.AddDate(0, 0, p.N)
	}
	return out, nil
}

func monthlyDays(s Schedule, lo, hi time.Time) ([]time.Time, error) {
	var p MonthlyDaysParams
	if err := strictUnmarshal(s.Params, &p); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidParams, err)
	}
	if len(p.Days) == 0 {
		return nil, fmt.Errorf("%w: days must not be empty", ErrInvalidParams)
	}

	// Deduplicate and sort defensively; validation does this on write but
	// Occurrences is also called from tests and legacy data.
	days := sortedUniqueDays(p.Days)
	for _, d := range days {
		if d < 1 || d > 30 {
			return nil, fmt.Errorf("%w: day %d out of [1,30]", ErrInvalidParams, d)
		}
	}

	loc := lo.Location()
	out := make([]time.Time, 0, 16)

	// Walk month by month. Start from the first day of lo's month so the
	// loop always makes progress on boundary conditions.
	cur := time.Date(lo.Year(), lo.Month(), 1, 0, 0, 0, 0, loc)
	for !cur.After(hi) {
		year, month := cur.Year(), cur.Month()
		lastDay := daysInMonth(year, month)
		for _, d := range days {
			if d > lastDay {
				// 29/30 in February non-leap — silently skipped. The spec
				// does not ask for snap-to-last-day behaviour.
				continue
			}
			candidate := time.Date(year, month, d, 0, 0, 0, 0, loc)
			if candidate.Before(lo) || candidate.After(hi) {
				continue
			}
			out = append(out, candidate)
		}
		cur = cur.AddDate(0, 1, 0)
	}
	return out, nil
}

func specificDates(s Schedule, loc *time.Location, lo, hi time.Time) ([]time.Time, error) {
	var p SpecificDatesParams
	if err := strictUnmarshal(s.Params, &p); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidParams, err)
	}
	if len(p.Dates) == 0 {
		return nil, fmt.Errorf("%w: dates must not be empty", ErrInvalidParams)
	}

	seen := make(map[int64]struct{}, len(p.Dates))
	out := make([]time.Time, 0, len(p.Dates))
	for _, raw := range p.Dates {
		d := raw.In(loc)
		key := d.Unix()
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		if d.Before(lo) || d.After(hi) {
			continue
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out, nil
}

func evenOdd(s Schedule, lo, hi time.Time) ([]time.Time, error) {
	var p EvenOddParams
	if err := strictUnmarshal(s.Params, &p); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidParams, err)
	}
	if !p.Parity.Valid() {
		return nil, fmt.Errorf("%w: parity must be even or odd", ErrInvalidParams)
	}

	wantEven := p.Parity == ParityEven
	out := make([]time.Time, 0, 16)
	for d := lo; !d.After(hi); d = d.AddDate(0, 0, 1) {
		isEven := d.Day()%2 == 0
		if isEven == wantEven {
			out = append(out, d)
		}
	}
	return out, nil
}

// sortedUniqueDays returns a sorted copy of days with duplicates removed.
func sortedUniqueDays(days []int) []int {
	if len(days) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(days))
	out := make([]int, 0, len(days))
	for _, d := range days {
		if _, dup := seen[d]; dup {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	sort.Ints(out)
	return out
}

// daysInMonth returns the number of days in the given (year, month).
func daysInMonth(year int, month time.Month) int {
	// Trick: day 0 of the next month == last day of the current month.
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// strictUnmarshal decodes raw JSON into v while rejecting unknown fields.
// It is used on every Params payload so that typos like {"n": 3, "days": [1]}
// are rejected instead of being silently dropped.
func strictUnmarshal(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return fmt.Errorf("params are empty")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	return nil
}
