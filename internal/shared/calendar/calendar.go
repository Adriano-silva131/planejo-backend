// Package calendar holds calendar values without time zone: days of the week,
// wall-clock times and dates, all read in the user's time zone.
package calendar

import (
	"fmt"
	"time"
)

// Weekdays is a set of days of the week.
type Weekdays uint8

func NewWeekdays(days ...time.Weekday) (Weekdays, error) {
	var w Weekdays
	for _, d := range days {
		if d < time.Sunday || d > time.Saturday {
			return 0, ErrInvalidWeekday
		}
		w |= 1 << d
	}
	return w, nil
}

func (w Weekdays) Has(d time.Weekday) bool {
	return w&(1<<d) != 0
}

func (w Weekdays) IsEmpty() bool {
	return w == 0
}

func (w Weekdays) Days() []time.Weekday {
	var days []time.Weekday
	for d := time.Sunday; d <= time.Saturday; d++ {
		if w.Has(d) {
			days = append(days, d)
		}
	}
	return days
}

// TimeOfDay is a wall-clock time in minutes since midnight, in the user's
// time zone.
type TimeOfDay int

const minutesPerDay = 24 * 60

func NewTimeOfDay(hour, minute int) (TimeOfDay, error) {
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, ErrInvalidTime
	}
	return TimeOfDay(hour*60 + minute), nil
}

// ParseTimeOfDay reads the "15:04" format.
func ParseTimeOfDay(s string) (TimeOfDay, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, ErrInvalidTime
	}
	return NewTimeOfDay(t.Hour(), t.Minute())
}

func (t TimeOfDay) Valid() bool {
	return t >= 0 && t < minutesPerDay
}

func (t TimeOfDay) String() string {
	return fmt.Sprintf("%02d:%02d", int(t)/60, int(t)%60)
}

// Date is a calendar day, without time or time zone.
type Date struct {
	t time.Time
}

func NewDate(year int, month time.Month, day int) Date {
	return Date{t: time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

// DateOf returns the day of t in t's location.
func DateOf(t time.Time) Date {
	return NewDate(t.Date())
}

// ParseDate reads the "2006-01-02" format.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return Date{}, ErrInvalidDate
	}
	return Date{t: t}, nil
}

// ParseOptionalDate is ParseDate where an empty string is the zero Date.
func ParseOptionalDate(s string) (Date, error) {
	if s == "" {
		return Date{}, nil
	}
	return ParseDate(s)
}

func (d Date) IsZero() bool {
	return d.t.IsZero()
}

// Time returns the day at midnight UTC.
func (d Date) Time() time.Time {
	return d.t
}

// String formats the "2006-01-02" layout; the zero Date is empty.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return d.t.Format(time.DateOnly)
}

func (d Date) Weekday() time.Weekday {
	return d.t.Weekday()
}

func (d Date) AddDays(n int) Date {
	return Date{t: d.t.AddDate(0, 0, n)}
}

func (d Date) Equal(other Date) bool {
	return d.t.Equal(other.t)
}

func (d Date) Before(other Date) bool {
	return d.t.Before(other.t)
}

func (d Date) After(other Date) bool {
	return d.t.After(other.t)
}

// StartOfWeek returns the Monday of d's week.
func (d Date) StartOfWeek() Date {
	return d.AddDays(-(int(d.Weekday()) + 6) % 7)
}
