package schedule_test

import (
	"encoding/json"
	"testing"
	"time"

	"example.com/taskservice/internal/domain/schedule"
)

// mkDate builds a time.Time at midnight UTC for the given date components.
// It is used throughout the test file so that test cases remain readable.
func mkDate(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// jsonRaw encodes v to json.RawMessage. Panics on error — tests only.
func jsonRaw(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// makeSchedule returns a minimal valid schedule for the given kind + params.
// The caller overrides fields as needed.
func makeSchedule(kind schedule.Kind, params any, start time.Time) schedule.Schedule {
	return schedule.Schedule{
		Kind:      kind,
		Params:    jsonRaw(params),
		StartDate: start,
		Timezone:  "UTC",
		Active:    true,
	}
}


func TestOccurrences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		schedule  schedule.Schedule
		from      time.Time
		to        time.Time
		wantDates []time.Time // if nil, just check len(wantLen)
		wantLen   int
		wantErr   bool
	}{
		// ── daily_every_n ────────────────────────────────────────────────────────
		{
			name:    "daily n=1 seven-day window",
			schedule: makeSchedule(schedule.KindDailyEveryN, map[string]any{"n": 1}, mkDate(2026, 4, 1)),
			from:    mkDate(2026, 4, 1),
			to:      mkDate(2026, 4, 7),
			wantLen: 7,
		},
		{
			name: "daily n=3 window includes anchor",
			schedule: makeSchedule(schedule.KindDailyEveryN, map[string]any{"n": 3}, mkDate(2026, 4, 10)),
			from:     mkDate(2026, 4, 8),
			to:       mkDate(2026, 4, 20),
			wantDates: []time.Time{
				mkDate(2026, 4, 10),
				mkDate(2026, 4, 13),
				mkDate(2026, 4, 16),
				mkDate(2026, 4, 19),
			},
		},
		{
			// Critical: lo is NOT on the anchor; must snap forward correctly.
			name: "daily n=3 window does not include anchor — anchoring snap",
			schedule: makeSchedule(schedule.KindDailyEveryN, map[string]any{"n": 3}, mkDate(2026, 4, 10)),
			from:     mkDate(2026, 4, 11),
			to:       mkDate(2026, 4, 20),
			wantDates: []time.Time{
				mkDate(2026, 4, 13),
				mkDate(2026, 4, 16),
				mkDate(2026, 4, 19),
			},
		},
		{
			name: "daily n=2 end_date truncates last hit",
			schedule: func() schedule.Schedule {
				endDate := mkDate(2026, 4, 8)
				s := makeSchedule(schedule.KindDailyEveryN, map[string]any{"n": 2}, mkDate(2026, 4, 1))
				s.EndDate = &endDate
				return s
			}(),
			from: mkDate(2026, 4, 1),
			to:   mkDate(2026, 4, 10),
			wantDates: []time.Time{
				mkDate(2026, 4, 1),
				mkDate(2026, 4, 3),
				mkDate(2026, 4, 5),
				mkDate(2026, 4, 7),
			},
		},
		{
			name:    "daily n=0 invalid — expects error",
			schedule: makeSchedule(schedule.KindDailyEveryN, map[string]any{"n": 0}, mkDate(2026, 4, 1)),
			from:    mkDate(2026, 4, 1),
			to:      mkDate(2026, 4, 7),
			wantErr: true,
		},
		{
			name:    "daily n=1 large window 400 days — no overflow",
			schedule: makeSchedule(schedule.KindDailyEveryN, map[string]any{"n": 1}, mkDate(2026, 1, 1)),
			from:    mkDate(2026, 1, 1),
			to:      mkDate(2027, 2, 4), // 400 days later
			wantLen: 400,
		},
		{
			name:    "daily n=365 over 5 years — about 5 hits",
			schedule: makeSchedule(schedule.KindDailyEveryN, map[string]any{"n": 365}, mkDate(2026, 1, 1)),
			from:    mkDate(2026, 1, 1),
			to:      mkDate(2031, 1, 1),
			wantLen: 6, // 2026-01-01, 2027-01-01, 2028-01-01, 2029-01-01, 2030-01-01, 2031-01-01
		},

		// ── monthly_days ──────────────────────────────────────────────────────
		{
			name: "monthly [1,15] window spans march-april",
			schedule: makeSchedule(schedule.KindMonthlyDays, map[string]any{"days": []int{1, 15}}, mkDate(2026, 3, 1)),
			from: mkDate(2026, 3, 1),
			to:   mkDate(2026, 4, 30),
			wantDates: []time.Time{
				mkDate(2026, 3, 1),
				mkDate(2026, 3, 15),
				mkDate(2026, 4, 1),
				mkDate(2026, 4, 15),
			},
		},
		{
			// Days 29 and 30 silently skipped in February non-leap.
			name: "monthly [29,30] february 2026 non-leap produces nothing",
			schedule: makeSchedule(schedule.KindMonthlyDays, map[string]any{"days": []int{29, 30}}, mkDate(2026, 2, 1)),
			from: mkDate(2026, 2, 1),
			to:   mkDate(2026, 2, 28),
			wantLen: 0,
		},
		{
			// Same schedule across February and March: March has both 29 and 30.
			name: "monthly [29,30] february+march 2026 — only march hits",
			schedule: makeSchedule(schedule.KindMonthlyDays, map[string]any{"days": []int{29, 30}}, mkDate(2026, 2, 1)),
			from: mkDate(2026, 2, 1),
			to:   mkDate(2026, 3, 31),
			wantDates: []time.Time{
				mkDate(2026, 3, 29),
				mkDate(2026, 3, 30),
			},
		},
		{
			// Feb 29 exists in 2028 (leap year).
			name: "monthly [29] february 2028 leap year — produces Feb 29",
			schedule: makeSchedule(schedule.KindMonthlyDays, map[string]any{"days": []int{29}}, mkDate(2028, 2, 1)),
			from: mkDate(2028, 2, 1),
			to:   mkDate(2028, 2, 29),
			wantDates: []time.Time{mkDate(2028, 2, 29)},
		},
		{
			// Spec caps valid days at 30; 31 must be rejected by Occurrences.
			name:    "monthly [31] invalid — expects error",
			schedule: makeSchedule(schedule.KindMonthlyDays, map[string]any{"days": []int{31}}, mkDate(2026, 1, 1)),
			from:    mkDate(2026, 1, 1),
			to:      mkDate(2026, 12, 31),
			wantErr: true,
		},
		{
			name:    "monthly [0] invalid — expects error",
			schedule: makeSchedule(schedule.KindMonthlyDays, map[string]any{"days": []int{0}}, mkDate(2026, 1, 1)),
			from:    mkDate(2026, 1, 1),
			to:      mkDate(2026, 12, 31),
			wantErr: true,
		},
		{
			// Validator deduplicates [15,1,15] to [1,15]; Occurrences also dedupes.
			name: "monthly [15,1,15] deduped and sorted",
			schedule: makeSchedule(schedule.KindMonthlyDays, map[string]any{"days": []int{15, 1, 15}}, mkDate(2026, 4, 1)),
			from: mkDate(2026, 4, 1),
			to:   mkDate(2026, 4, 30),
			wantDates: []time.Time{
				mkDate(2026, 4, 1),
				mkDate(2026, 4, 15),
			},
		},

		// ── specific_dates ────────────────────────────────────────────────────
		{
			name: "specific_dates — one in window, two outside",
			schedule: makeSchedule(schedule.KindSpecificDates, map[string]any{
				"dates": []string{"2026-03-15", "2026-04-10", "2026-05-01"},
			}, mkDate(2026, 1, 1)),
			from:    mkDate(2026, 4, 1),
			to:      mkDate(2026, 4, 30),
			wantDates: []time.Time{mkDate(2026, 4, 10)},
		},
		{
			name: "specific_dates — all in past, empty result",
			schedule: makeSchedule(schedule.KindSpecificDates, map[string]any{
				"dates": []string{"2025-01-01", "2025-06-15"},
			}, mkDate(2025, 1, 1)),
			from:    mkDate(2026, 1, 1),
			to:      mkDate(2026, 12, 31),
			wantLen: 0,
		},
		{
			name: "specific_dates — duplicates deduplicated",
			schedule: makeSchedule(schedule.KindSpecificDates, map[string]any{
				"dates": []string{"2026-04-10", "2026-04-10", "2026-04-15"},
			}, mkDate(2026, 1, 1)),
			from: mkDate(2026, 4, 1),
			to:   mkDate(2026, 4, 30),
			wantDates: []time.Time{mkDate(2026, 4, 10), mkDate(2026, 4, 15)},
		},
		{
			name: "specific_dates — empty list returns error",
			schedule: makeSchedule(schedule.KindSpecificDates, map[string]any{
				"dates": []string{},
			}, mkDate(2026, 1, 1)),
			from:    mkDate(2026, 4, 1),
			to:      mkDate(2026, 4, 30),
			wantErr: true,
		},

		// ── even_odd ──────────────────────────────────────────────────────────
		{
			name:    "even_odd parity=even window Apr 1-5",
			schedule: makeSchedule(schedule.KindEvenOdd, map[string]any{"parity": "even"}, mkDate(2026, 4, 1)),
			from:    mkDate(2026, 4, 1),
			to:      mkDate(2026, 4, 5),
			wantDates: []time.Time{
				mkDate(2026, 4, 2),
				mkDate(2026, 4, 4),
			},
		},
		{
			name:    "even_odd parity=odd window Apr 28 to May 3",
			schedule: makeSchedule(schedule.KindEvenOdd, map[string]any{"parity": "odd"}, mkDate(2026, 4, 1)),
			from:    mkDate(2026, 4, 28),
			to:      mkDate(2026, 5, 3),
			wantDates: []time.Time{
				mkDate(2026, 4, 29),
				mkDate(2026, 5, 1),
				mkDate(2026, 5, 3),
			},
		},
		{
			name:    "even_odd invalid parity returns error",
			schedule: makeSchedule(schedule.KindEvenOdd, map[string]any{"parity": "both"}, mkDate(2026, 4, 1)),
			from:    mkDate(2026, 4, 1),
			to:      mkDate(2026, 4, 10),
			wantErr: true,
		},

		// ── window / date boundary cases ────────────────────────────────────
		{
			name: "end_date before start_date — expect error",
			schedule: func() schedule.Schedule {
				start := mkDate(2026, 4, 10)
				end := mkDate(2026, 4, 5)
				s := makeSchedule(schedule.KindDailyEveryN, map[string]any{"n": 1}, start)
				s.EndDate = &end
				return s
			}(),
			from:    mkDate(2026, 4, 1),
			to:      mkDate(2026, 4, 30),
			wantErr: true,
		},
		{
			name:    "from after to — empty result, no error",
			schedule: makeSchedule(schedule.KindDailyEveryN, map[string]any{"n": 1}, mkDate(2026, 4, 1)),
			from:    mkDate(2026, 4, 20),
			to:      mkDate(2026, 4, 10),
			wantLen: 0,
		},
		{
			name:    "window entirely before start_date — empty result",
			schedule: makeSchedule(schedule.KindDailyEveryN, map[string]any{"n": 1}, mkDate(2026, 5, 1)),
			from:    mkDate(2026, 4, 1),
			to:      mkDate(2026, 4, 30),
			wantLen: 0,
		},
		{
			name: "window entirely after end_date — empty result",
			schedule: func() schedule.Schedule {
				end := mkDate(2026, 3, 31)
				s := makeSchedule(schedule.KindDailyEveryN, map[string]any{"n": 1}, mkDate(2026, 3, 1))
				s.EndDate = &end
				return s
			}(),
			from:    mkDate(2026, 4, 1),
			to:      mkDate(2026, 4, 30),
			wantLen: 0,
		},

		// ── timezone ─────────────────────────────────────────────────────────
		{
			// 2026-04-09 21:00 UTC == 2026-04-10 00:00 MSK.
			// A daily-every-1 schedule anchored on Apr 10 must fire on Apr 10.
			name: "timezone Europe/Moscow: from in UTC resolves correctly to MSK day",
			schedule: func() schedule.Schedule {
				s := makeSchedule(schedule.KindDailyEveryN, map[string]any{"n": 1}, mkDate(2026, 4, 10))
				s.Timezone = "Europe/Moscow"
				return s
			}(),
			from: time.Date(2026, 4, 9, 21, 0, 0, 0, time.UTC),
			to:   time.Date(2026, 4, 10, 20, 59, 59, 0, time.UTC),
			// Window in MSK: Apr 10 00:00 – Apr 10 23:59.
			// Schedule fires on Apr 10 (n=1, anchor Apr 10).
			wantLen: 1,
		},
		{
			// Basic sanity: code path for a non-Moscow TZ works.
			// from/to are UTC midnight; converted to PDT (UTC-7) the window
			// lands on Mar 31–Apr 3, so the only even day is Apr 2 → 1 hit.
			name: "timezone America/Los_Angeles — sanity check",
			schedule: func() schedule.Schedule {
				s := makeSchedule(schedule.KindEvenOdd, map[string]any{"parity": "even"}, mkDate(2026, 3, 31))
				s.Timezone = "America/Los_Angeles"
				return s
			}(),
			from:    mkDate(2026, 4, 1),
			to:      mkDate(2026, 4, 4),
			wantLen: 1,
		},
		{
			name: "invalid timezone — expect error",
			schedule: func() schedule.Schedule {
				s := makeSchedule(schedule.KindDailyEveryN, map[string]any{"n": 1}, mkDate(2026, 4, 1))
				s.Timezone = "Not/ATimezone"
				return s
			}(),
			from:    mkDate(2026, 4, 1),
			to:      mkDate(2026, 4, 7),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := schedule.Occurrences(tc.schedule, tc.from, tc.to)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; results=%v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantDates != nil {
				if len(got) != len(tc.wantDates) {
					t.Fatalf("got %d dates, want %d\ngot:  %v\nwant: %v", len(got), len(tc.wantDates), got, tc.wantDates)
				}
				for i := range tc.wantDates {
					if !sameDate(got[i], tc.wantDates[i]) {
						t.Errorf("got[%d] = %v, want %v", i, got[i].Format("2006-01-02"), tc.wantDates[i].Format("2006-01-02"))
					}
				}
				return
			}

			if len(got) != tc.wantLen {
				t.Fatalf("got %d dates, want %d", len(got), tc.wantLen)
			}
		})
	}
}

// sameDate reports whether a and b represent the same calendar date,
// ignoring time-of-day and timezone.
func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
