package automation

import (
	"time"

	"horizon/foundation/kernel/capability/contracts"
)

// Schedule kinds. The anchor run_at stores the first execution instant; a
// recurring kind reuses its local wall-clock components (minute, weekday or
// day-of-month) in the task timezone, so a DST change never silently shifts a
// daily task to a different hour.
const (
	ScheduleOnce    = "once"
	ScheduleHourly  = "hourly"
	ScheduleDaily   = "daily"
	ScheduleWeekly  = "weekly"
	ScheduleMonthly = "monthly"
)

// ScheduleKinds is the closed set shared by the capability schema, the runtime
// form and the worker. A value outside it is rejected before any row is written.
var ScheduleKinds = []string{ScheduleOnce, ScheduleHourly, ScheduleDaily, ScheduleWeekly, ScheduleMonthly}

// Timezones is the curated IANA list offered by the form and the agent tool.
// The capability still validates with time.LoadLocation, so a stale list entry
// cannot put an unusable zone into the database.
var Timezones = []string{
	"Asia/Shanghai", "Asia/Singapore", "Asia/Tokyo", "UTC",
	"Europe/London", "Europe/Berlin", "America/New_York", "America/Los_Angeles",
}

// DefaultTimezone keeps agent created tasks usable without a second question.
const DefaultTimezone = "Asia/Shanghai"

// ScheduleSpec is the wall-clock shape the owner picked in the form. Only the
// components the kind needs are read; the rest stay nil.
type ScheduleSpec struct {
	Kind       string
	Timezone   *time.Location
	RunAt      string
	Hour       *int
	Minute     *int
	Second     *int
	Weekday    *int // ISO: 1 = Monday ... 7 = Sunday.
	DayOfMonth *int
}

// BuildAnchor validates one schedule and returns its first occurrence plus the
// intended day-of-month for monthly series (0 for every other kind).
func BuildAnchor(spec ScheduleSpec, now time.Time) (time.Time, int, error) {
	if spec.Timezone == nil {
		return time.Time{}, 0, contracts.Problem("invalid")
	}
	switch spec.Kind {
	case ScheduleOnce:
		if spec.RunAt == "" {
			return time.Time{}, 0, contracts.Problem("invalid")
		}
		anchor, err := ParseRunAt(spec.RunAt, spec.Timezone)
		if err != nil || !anchor.After(now) {
			return time.Time{}, 0, contracts.Problem("invalid")
		}
		return anchor, 0, nil
	case ScheduleHourly:
		minute, err := component(spec.Minute, 0, 59)
		if err != nil {
			return time.Time{}, 0, err
		}
		second := 0
		if spec.Second != nil {
			second, err = component(spec.Second, 0, 59)
			if err != nil {
				return time.Time{}, 0, err
			}
		}
		local := now.In(spec.Timezone)
		candidate := time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), minute, second, 0, spec.Timezone)
		if !candidate.After(now) {
			candidate = candidate.Add(time.Hour)
		}
		return candidate.UTC(), 0, nil
	case ScheduleDaily:
		hour, minute, err := clock(spec)
		if err != nil {
			return time.Time{}, 0, err
		}
		local := now.In(spec.Timezone)
		candidate := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, spec.Timezone)
		if !candidate.After(now) {
			candidate = time.Date(local.Year(), local.Month(), local.Day()+1, hour, minute, 0, 0, spec.Timezone)
		}
		return candidate.UTC(), 0, nil
	case ScheduleWeekly:
		weekday, err := component(spec.Weekday, 1, 7)
		if err != nil {
			return time.Time{}, 0, err
		}
		hour, minute, err := clock(spec)
		if err != nil {
			return time.Time{}, 0, err
		}
		local := now.In(spec.Timezone)
		delta := (weekday - isoWeekday(local) + 7) % 7
		candidate := time.Date(local.Year(), local.Month(), local.Day()+delta, hour, minute, 0, 0, spec.Timezone)
		if !candidate.After(now) {
			candidate = candidate.AddDate(0, 0, 7)
		}
		return candidate.UTC(), 0, nil
	case ScheduleMonthly:
		day, err := component(spec.DayOfMonth, 1, 31)
		if err != nil {
			return time.Time{}, 0, err
		}
		hour, minute, err := clock(spec)
		if err != nil {
			return time.Time{}, 0, err
		}
		local := now.In(spec.Timezone)
		for offset := 0; offset <= 24; offset++ {
			first := time.Date(local.Year(), local.Month()+time.Month(offset), 1, 0, 0, 0, 0, spec.Timezone)
			lastDay := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, spec.Timezone).Day()
			candidateDay := day
			if candidateDay > lastDay {
				candidateDay = lastDay
			}
			candidate := time.Date(first.Year(), first.Month(), candidateDay, hour, minute, 0, 0, spec.Timezone)
			if candidate.After(now) {
				return candidate.UTC(), day, nil
			}
		}
		return time.Time{}, 0, contracts.Problem("invalid")
	}
	return time.Time{}, 0, contracts.Problem("invalid")
}

func component(value *int, low, high int) (int, error) {
	if value == nil || *value < low || *value > high {
		return 0, contracts.Problem("invalid")
	}
	return *value, nil
}

func clock(spec ScheduleSpec) (int, int, error) {
	hour, err := component(spec.Hour, 0, 23)
	if err != nil {
		return 0, 0, err
	}
	minute, err := component(spec.Minute, 0, 59)
	if err != nil {
		return 0, 0, err
	}
	return hour, minute, nil
}

func isoWeekday(value time.Time) int {
	if value.Weekday() == time.Sunday {
		return 7
	}
	return int(value.Weekday())
}

// NextOccurrence returns the first occurrence strictly after the reference
// instant. The anchor defines the wall-clock shape of the series: minute and
// second for hourly, hour and minute for daily, weekday for weekly and the
// stored day-of-month for monthly. Returns false when a one-shot task has
// already fired.
func NextOccurrence(kind string, anchor time.Time, loc *time.Location, after time.Time, monthlyDay int) (time.Time, bool) {
	if loc == nil {
		loc = time.UTC
	}
	anchorLocal := anchor.In(loc)
	afterLocal := after.In(loc)
	switch kind {
	case ScheduleOnce:
		if anchor.After(after) {
			return anchor.UTC(), true
		}
		return time.Time{}, false
	case ScheduleHourly:
		candidate := time.Date(afterLocal.Year(), afterLocal.Month(), afterLocal.Day(), afterLocal.Hour(), anchorLocal.Minute(), anchorLocal.Second(), 0, loc)
		if !candidate.After(after) {
			candidate = candidate.Add(time.Hour)
		}
		return candidate.UTC(), true
	case ScheduleDaily:
		candidate := time.Date(afterLocal.Year(), afterLocal.Month(), afterLocal.Day(), anchorLocal.Hour(), anchorLocal.Minute(), 0, 0, loc)
		if !candidate.After(after) {
			candidate = time.Date(candidate.Year(), candidate.Month(), candidate.Day()+1, candidate.Hour(), candidate.Minute(), 0, 0, loc)
		}
		return candidate.UTC(), true
	case ScheduleWeekly:
		delta := (int(anchorLocal.Weekday()) - int(afterLocal.Weekday()) + 7) % 7
		candidate := time.Date(afterLocal.Year(), afterLocal.Month(), afterLocal.Day()+delta, anchorLocal.Hour(), anchorLocal.Minute(), 0, 0, loc)
		if !candidate.After(after) {
			candidate = candidate.AddDate(0, 0, 7)
		}
		return candidate.UTC(), true
	case ScheduleMonthly:
		// A monthly series keeps the day the owner picked, clamped to the last
		// day of a shorter month, and never drifts to the first of the next month.
		day := monthlyDay
		if day <= 0 {
			day = anchorLocal.Day()
		}
		for offset := 0; offset <= 60; offset++ {
			first := time.Date(afterLocal.Year(), afterLocal.Month()+time.Month(offset), 1, 0, 0, 0, 0, loc)
			lastDay := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, loc).Day()
			candidateDay := day
			if candidateDay > lastDay {
				candidateDay = lastDay
			}
			candidate := time.Date(first.Year(), first.Month(), candidateDay, anchorLocal.Hour(), anchorLocal.Minute(), 0, 0, loc)
			if candidate.After(after) {
				return candidate.UTC(), true
			}
		}
		return time.Time{}, false
	}
	return time.Time{}, false
}

// ParseRunAt accepts the runtime datetime-local value ("2006-01-02T15:04"),
// a matching local string with seconds, or an RFC3339 instant. Local wall-clock
// values are interpreted in the task timezone, which is the only interpretation
// the form can offer.
func ParseRunAt(value string, loc *time.Location) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), nil
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05"} {
		if parsed, err := time.ParseInLocation(layout, value, loc); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, contracts.Problem("invalid")
}

// FormatRunAt renders an instant in the task timezone for the datetime-local
// form control, so editing a task does not shift the wall clock the owner chose.
func FormatRunAt(value time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	return value.In(loc).Format("2006-01-02T15:04")
}
