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
	// current and later weeks count once met and never break the streak.
	Streak int
	// Weeks are oldest first, only those in which the task was a habit.
	Weeks []HabitWeek
}

// HabitWeek is met when the task was done on at least Target days, on any
// days of the week. Target is the number of weekdays set that week, or fewer
// in the week the task was created, counting only the days left then.
type HabitWeek struct {
	Start  calendar.Date
	Target int
	Done   int
	Met    bool
	Days   []HabitDay
}

type HabitDay struct {
	Date      calendar.Date
	Scheduled bool
	Done      bool
}

// doneDays holds the days a task was checked in.
type doneDays map[int64]bool

func dayKey(d calendar.Date) int64 {
	return d.Time().Unix()
}

// BuildHabits builds the habits of the recurring tasks for the weeks weeks up
// to the one that contains day. today tells which week is still going on.
// Tasks created after the reference week are left out.
func BuildHabits(today, day calendar.Date, weeks int, tasks []Task, checkIns []CheckIn) []Habit {
	ref := day.StartOfWeek()
	end := ref.AddDays(6)

	done := map[uuid.UUID]doneDays{}
	for _, c := range checkIns {
		if c.Date.After(end) {
			continue
		}
		if done[c.TaskID] == nil {
			done[c.TaskID] = doneDays{}
		}
		done[c.TaskID][dayKey(c.Date)] = true
	}

	habits := []Habit{}
	for _, t := range tasks {
		if t.Kind == RecurringTask && !t.CreatedOn.After(end) {
			habits = append(habits, buildHabit(t, today, ref, weeks, done[t.ID]))
		}
	}
	return habits
}

func buildHabit(t Task, today, ref calendar.Date, weeks int, done doneDays) Habit {
	h := Habit{Task: t, Streak: streak(t, today, ref, done)}
	for i := weeks - 1; i >= 0; i-- {
		if start := ref.AddDays(-7 * i); t.activeIn(start) {
			h.Weeks = append(h.Weeks, habitWeek(t, start, done))
		}
	}
	return h
}

// streak walks back from the reference week, over the whole history, until a
// finished week that was not met or a week in which the task was no habit.
func streak(t Task, today, ref calendar.Date, done doneDays) int {
	current := today.StartOfWeek()
	n := 0
	for start := ref; t.activeIn(start); start = start.AddDays(-7) {
		if habitWeek(t, start, done).Met {
			n++
			continue
		}
		if start.Before(current) {
			break
		}
	}
	return n
}

// activeIn reports whether the task existed and was recurring in the week
// starting on start.
func (t Task) activeIn(start calendar.Date) bool {
	return !start.Before(t.CreatedOn.StartOfWeek()) && !t.weekdaysOn(start).IsEmpty()
}

func habitWeek(t Task, start calendar.Date, done doneDays) HabitWeek {
	set := t.weekdaysOn(start)
	w := HabitWeek{Start: start, Target: t.target(start, set), Days: make([]HabitDay, 0, 7)}
	for i := range 7 {
		d := start.AddDays(i)
		day := HabitDay{Date: d, Scheduled: set.Has(d.Weekday()), Done: done[dayKey(d)]}
		if day.Done {
			w.Done++
		}
		w.Days = append(w.Days, day)
	}
	w.Met = w.Done >= w.Target
	return w
}

// target is how many days the week asks for: the weekdays set, but no more
// than the days left in the week the task was created.
func (t Task) target(start calendar.Date, set calendar.Weekdays) int {
	target := len(set.Days())
	left := 0
	for i := range 7 {
		if !start.AddDays(i).Before(t.CreatedOn) {
			left++
		}
	}
	return min(target, left)
}
