package domain

import (
	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// Habit is the light follow-up of a recurring task: the days done in the last
// weeks and how many weeks in a row it was met.
type Habit struct {
	Task Task
	// Streak counts the weeks met in a row up to the reference week. The
	// current week counts once met and never breaks the streak.
	Streak int
	// Weeks are oldest first, only those in which the task was a habit.
	Weeks []HabitWeek
}

// HabitWeek is met when the task was done on at least Target days, on any
// days of the week. Target is how many times a week the task was set for. The
// week the task was created or its schedule changed never asks for more than
// was fair then: the days left, or the lower of the old and new counts.
type HabitWeek struct {
	Start  calendar.Date
	Target int
	Done   int
	Met    bool
	Days   []HabitDay
}

// HabitDay is Scheduled on the days the user usually does the task.
type HabitDay struct {
	Date      calendar.Date
	Scheduled bool
	Done      bool
}

// BuildHabits builds the habits of the recurring tasks for the weeks weeks up
// to the one that contains day. today tells which week is still going on.
// Tasks created after the reference week are left out.
func BuildHabits(today, day calendar.Date, weeks int, tasks []Task, checkIns []CheckIn) []Habit {
	ref := day.StartOfWeek()
	end := ref.AddDays(6)

	done := map[uuid.UUID]map[calendar.Date]bool{}
	for _, c := range checkIns {
		if c.Date.After(end) {
			continue
		}
		if done[c.TaskID] == nil {
			done[c.TaskID] = map[calendar.Date]bool{}
		}
		done[c.TaskID][c.Date] = true
	}

	habits := []Habit{}
	for _, t := range tasks {
		if t.Kind == RecurringTask && !t.CreatedOn.After(end) {
			b := habitBuilder{task: t, done: done[t.ID], weeks: map[calendar.Date]HabitWeek{}}
			habits = append(habits, b.habit(today, ref, weeks))
		}
	}
	return habits
}

// habitBuilder builds each week of a task once, for the grid and the streak.
type habitBuilder struct {
	task  Task
	done  map[calendar.Date]bool
	weeks map[calendar.Date]HabitWeek
}

func (b habitBuilder) habit(today, ref calendar.Date, weeks int) Habit {
	h := Habit{Task: b.task, Streak: b.streak(today, ref)}
	for i := weeks - 1; i >= 0; i-- {
		if start := ref.AddDays(-7 * i); b.task.activeIn(start) {
			h.Weeks = append(h.Weeks, b.week(start))
		}
	}
	return h
}

// streak walks back over the whole history until a finished week that was
// not met or a week in which the task was no habit. Weeks after the current
// one have no check-ins, so the walk starts at the current week at the latest.
func (b habitBuilder) streak(today, ref calendar.Date) int {
	current := today.StartOfWeek()
	start := ref
	if start.After(current) {
		start = current
	}
	n := 0
	for ; b.task.activeIn(start); start = start.AddDays(-7) {
		if b.week(start).Met {
			n++
			continue
		}
		if start.Before(current) {
			break
		}
	}
	return n
}

func (b habitBuilder) week(start calendar.Date) HabitWeek {
	if w, ok := b.weeks[start]; ok {
		return w
	}
	r := b.task.recurrenceIn(start)
	w := HabitWeek{Start: start, Target: b.task.target(start, r.TimesPerWeek), Days: make([]HabitDay, 0, 7)}
	for i := range 7 {
		d := start.AddDays(i)
		day := HabitDay{Date: d, Scheduled: r.Weekdays.Has(d.Weekday()), Done: b.done[d]}
		if day.Done {
			w.Done++
		}
		w.Days = append(w.Days, day)
	}
	w.Met = w.Done >= w.Target
	b.weeks[start] = w
	return w
}

// WeekSchedule is how a recurring task was set for one week: Target days,
// as HabitWeek asks for, usually on Weekdays.
type WeekSchedule struct {
	Task     Task
	Target   int
	Weekdays calendar.Weekdays
	// Since is the first day of the recurrence active in this week.
	Since calendar.Date
}

// ScheduleIn returns how t was set for the week starting on start, or false
// when it did not exist or was not recurring then.
func (t Task) ScheduleIn(start calendar.Date) (WeekSchedule, bool) {
	if t.Kind != RecurringTask || !t.activeIn(start) {
		return WeekSchedule{}, false
	}
	r := t.recurrenceIn(start)
	since := t.CreatedOn
	for _, previous := range t.PastSchedules {
		if !start.Before(previous.Until()) && previous.TimesPerWeek == 0 && since.Before(previous.ChangedOn) {
			since = previous.ChangedOn
		}
	}
	return WeekSchedule{Task: t, Target: t.target(start, r.TimesPerWeek), Weekdays: r.Weekdays, Since: since}, true
}

// activeIn reports whether the task existed and was recurring in the week
// starting on start.
func (t Task) activeIn(start calendar.Date) bool {
	return !start.Before(t.CreatedOn.StartOfWeek()) && t.recurrenceIn(start).TimesPerWeek > 0
}

// target is how many days the week starting on start asks for, when the task
// was set for times a week.
func (t Task) target(start calendar.Date, times int) int {
	target := times
	if start == t.CreatedOn.StartOfWeek() {
		target = min(target, daysLeft(start, t.CreatedOn))
	}
	if change, ok := t.changeIn(start); ok {
		if change.TimesPerWeek == 0 {
			target = min(target, daysLeft(start, change.ChangedOn))
		} else {
			target = min(target, change.TimesPerWeek)
		}
	}
	return target
}

// daysLeft counts the days of the week starting on start from day on.
func daysLeft(start, day calendar.Date) int {
	left := 0
	for i := range 7 {
		if !start.AddDays(i).Before(day) {
			left++
		}
	}
	return left
}
