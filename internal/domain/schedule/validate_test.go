package schedule_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"example.com/taskservice/internal/domain/schedule"
	taskdomain "example.com/taskservice/internal/domain/task"
)

func validBase() schedule.Schedule {
	return schedule.Schedule{
		Title:         "Daily standup",
		DefaultStatus: taskdomain.StatusNew,
		Kind:          schedule.KindDailyEveryN,
		Params:        jsonRaw(map[string]any{"n": 1}),
		StartDate:     mkDate(2026, 4, 10),
		Timezone:      "Europe/Moscow",
		Active:        true,
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	t.Run("valid schedule passes", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		if err := schedule.Validate(&s); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty title returns error", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		s.Title = "   "
		assertErrIs(t, schedule.Validate(&s), schedule.ErrInvalidSchedule)
	})

	t.Run("invalid status returns error", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		s.DefaultStatus = "unknown"
		assertErrIs(t, schedule.Validate(&s), schedule.ErrInvalidSchedule)
	})

	t.Run("invalid kind returns error", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		s.Kind = "hourly"
		assertErrIs(t, schedule.Validate(&s), schedule.ErrInvalidSchedule)
	})

	t.Run("zero start_date returns error", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		s.StartDate = time.Time{}
		assertErrIs(t, schedule.Validate(&s), schedule.ErrInvalidSchedule)
	})

	t.Run("end_date before start_date returns error", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		end := mkDate(2026, 4, 5)
		s.EndDate = &end
		assertErrIs(t, schedule.Validate(&s), schedule.ErrInvalidSchedule)
	})

	t.Run("invalid timezone returns error", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		s.Timezone = "Not/ATimezone"
		assertErrIs(t, schedule.Validate(&s), schedule.ErrInvalidSchedule)
	})

	t.Run("empty timezone defaults to Europe/Moscow — no error", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		s.Timezone = ""
		if err := schedule.Validate(&s); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestValidateAndNormalizeParams(t *testing.T) {
	t.Parallel()

	t.Run("daily n=1 valid", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		if err := schedule.ValidateAndNormalizeParams(&s); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("daily n=0 invalid", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		s.Params = jsonRaw(map[string]any{"n": 0})
		assertErrIs(t, schedule.ValidateAndNormalizeParams(&s), schedule.ErrInvalidParams)
	})

	t.Run("daily n=366 invalid", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		s.Params = jsonRaw(map[string]any{"n": 366})
		assertErrIs(t, schedule.ValidateAndNormalizeParams(&s), schedule.ErrInvalidParams)
	})

	t.Run("daily unknown field rejected", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		s.Params = jsonRaw(map[string]any{"n": 1, "extra": true})
		assertErrIs(t, schedule.ValidateAndNormalizeParams(&s), schedule.ErrInvalidParams)
	})

	t.Run("monthly_days valid with dedup+sort", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		s.Kind = schedule.KindMonthlyDays
		s.Params = jsonRaw(map[string]any{"days": []int{15, 1, 15}})
		if err := schedule.ValidateAndNormalizeParams(&s); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Params must be normalised to {"days":[1,15]}.
		var p schedule.MonthlyDaysParams
		if err := json.Unmarshal(s.Params, &p); err != nil {
			t.Fatalf("unmarshal normalised params: %v", err)
		}
		if len(p.Days) != 2 || p.Days[0] != 1 || p.Days[1] != 15 {
			t.Errorf("normalised days = %v, want [1 15]", p.Days)
		}
	})

	t.Run("monthly_days day=31 invalid", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		s.Kind = schedule.KindMonthlyDays
		s.Params = jsonRaw(map[string]any{"days": []int{31}})
		assertErrIs(t, schedule.ValidateAndNormalizeParams(&s), schedule.ErrInvalidParams)
	})

	t.Run("monthly_days day=0 invalid", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		s.Kind = schedule.KindMonthlyDays
		s.Params = jsonRaw(map[string]any{"days": []int{0}})
		assertErrIs(t, schedule.ValidateAndNormalizeParams(&s), schedule.ErrInvalidParams)
	})

	t.Run("monthly_days empty list invalid", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		s.Kind = schedule.KindMonthlyDays
		s.Params = jsonRaw(map[string]any{"days": []int{}})
		assertErrIs(t, schedule.ValidateAndNormalizeParams(&s), schedule.ErrInvalidParams)
	})

	t.Run("specific_dates valid with sort+dedup", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		s.Kind = schedule.KindSpecificDates
		s.Params = jsonRaw(map[string]any{"dates": []string{"2026-05-01", "2026-04-10", "2026-04-10"}})
		if err := schedule.ValidateAndNormalizeParams(&s); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var p schedule.SpecificDatesParams
		if err := json.Unmarshal(s.Params, &p); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(p.Dates) != 2 {
			t.Errorf("got %d dates after normalise, want 2", len(p.Dates))
		}
		if p.Dates[0].String() != "2026-04-10" {
			t.Errorf("first date = %q, want 2026-04-10", p.Dates[0].String())
		}
	})

	t.Run("specific_dates empty list invalid", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		s.Kind = schedule.KindSpecificDates
		s.Params = jsonRaw(map[string]any{"dates": []string{}})
		assertErrIs(t, schedule.ValidateAndNormalizeParams(&s), schedule.ErrInvalidParams)
	})

	t.Run("even_odd parity=even valid", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		s.Kind = schedule.KindEvenOdd
		s.Params = jsonRaw(map[string]any{"parity": "even"})
		if err := schedule.ValidateAndNormalizeParams(&s); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("even_odd invalid parity", func(t *testing.T) {
		t.Parallel()
		s := validBase()
		s.Kind = schedule.KindEvenOdd
		s.Params = jsonRaw(map[string]any{"parity": "both"})
		assertErrIs(t, schedule.ValidateAndNormalizeParams(&s), schedule.ErrInvalidParams)
	})
}

func assertErrIs(t *testing.T, err error, target error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error wrapping %v, got nil", target)
	}
	if !errors.Is(err, target) {
		t.Fatalf("error %v does not wrap %v", err, target)
	}
}
