package schedule

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Validate checks the whole-schedule invariants. It does NOT check Params —
// call ValidateParams separately (or call Validate which calls both).
func Validate(s *Schedule) error {
	var errs []string

	s.Title = strings.TrimSpace(s.Title)
	if s.Title == "" {
		errs = append(errs, "title is required")
	}
	if len(s.Title) > 255 {
		errs = append(errs, "title must be at most 255 characters")
	}
	if len(s.Description) > 4096 {
		errs = append(errs, "description must be at most 4096 characters")
	}

	if !s.DefaultStatus.Valid() {
		errs = append(errs, fmt.Sprintf("invalid default_status %q", s.DefaultStatus))
	}

	if !s.Kind.Valid() {
		errs = append(errs, fmt.Sprintf("invalid kind %q", s.Kind))
	}

	if s.StartDate.IsZero() {
		errs = append(errs, "start_date is required")
	}
	// Compare calendar dates in the schedule's timezone to avoid time-of-day
	// noise: a user passes "2026-04-05" and "2026-04-10", the stored time.Time
	// values carry midnight UTC; comparing in the schedule's zone is authoritative.
	if s.EndDate != nil && !s.StartDate.IsZero() {
		loc, locErr := s.Location()
		if locErr == nil {
			if dateIn(*s.EndDate, loc).Before(dateIn(s.StartDate, loc)) {
				errs = append(errs, "end_date must not be before start_date")
			}
		}
	}

	tz := s.Timezone
	if tz == "" {
		tz = DefaultTimezone
	}
	if _, err := time.LoadLocation(tz); err != nil {
		errs = append(errs, fmt.Sprintf("invalid timezone %q", tz))
	}

	if len(errs) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalidSchedule, strings.Join(errs, "; "))
	}

	// Validate kind-specific params.
	if err := ValidateAndNormalizeParams(s); err != nil {
		return err
	}

	return nil
}

// ValidateAndNormalizeParams validates the kind-specific Params and writes
// back a normalised (sorted, deduped) copy to s.Params on success.
func ValidateAndNormalizeParams(s *Schedule) error {
	switch s.Kind {
	case KindDailyEveryN:
		return validateDailyEveryN(s)
	case KindMonthlyDays:
		return validateMonthlyDays(s)
	case KindSpecificDates:
		return validateSpecificDates(s)
	case KindEvenOdd:
		return validateEvenOdd(s)
	default:
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidParams, s.Kind)
	}
}

func validateDailyEveryN(s *Schedule) error {
	var p DailyEveryNParams
	if err := strictUnmarshal(s.Params, &p); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidParams, err)
	}
	if p.N < 1 || p.N > 365 {
		return fmt.Errorf("%w: n must be in [1, 365]", ErrInvalidParams)
	}
	return nil
}

func validateMonthlyDays(s *Schedule) error {
	var p MonthlyDaysParams
	if err := strictUnmarshal(s.Params, &p); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidParams, err)
	}
	if len(p.Days) == 0 {
		return fmt.Errorf("%w: days must not be empty", ErrInvalidParams)
	}
	for _, d := range p.Days {
		if d < 1 || d > 30 {
			return fmt.Errorf("%w: day %d is out of allowed range [1, 30]", ErrInvalidParams, d)
		}
	}
	// Normalise: sort and deduplicate.
	p.Days = sortedUniqueDays(p.Days)
	normalized, err := json.Marshal(p)
	if err != nil {
		return err
	}
	s.Params = normalized
	return nil
}

func validateSpecificDates(s *Schedule) error {
	var p SpecificDatesParams
	if err := strictUnmarshal(s.Params, &p); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidParams, err)
	}
	if len(p.Dates) == 0 {
		return fmt.Errorf("%w: dates must not be empty", ErrInvalidParams)
	}
	if len(p.Dates) > 1000 {
		return fmt.Errorf("%w: at most 1000 specific dates are allowed", ErrInvalidParams)
	}
	// Normalise: dedup and sort.
	seen := make(map[string]struct{}, len(p.Dates))
	unique := p.Dates[:0]
	for _, d := range p.Dates {
		key := d.String()
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, d)
	}
	sort.Slice(unique, func(i, j int) bool {
		ti := unique[i].In(time.UTC)
		tj := unique[j].In(time.UTC)
		return ti.Before(tj)
	})
	p.Dates = unique
	normalized, err := json.Marshal(p)
	if err != nil {
		return err
	}
	s.Params = normalized
	return nil
}

func validateEvenOdd(s *Schedule) error {
	var p EvenOddParams
	if err := strictUnmarshal(s.Params, &p); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidParams, err)
	}
	if !p.Parity.Valid() {
		return fmt.Errorf("%w: parity must be %q or %q", ErrInvalidParams, ParityEven, ParityOdd)
	}
	return nil
}
