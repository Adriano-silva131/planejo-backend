package domain

import (
	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// Habit is the light follow-up of a recurring task: the days done in the last
// weeks and how many weeks in a row it was met.
type Habit struct {
	Task Task
	// Streak counts the weeks met in a row up to the reference week. That week
	// counts once met, and never breaks the streak while still going on.
	Streak int
	// Weeks are oldest first, from the week the task was created on.
	Weeks []HabitWeek
}

// HabitWeek is met when the task was done on at least as many days as it is
// set for, on any days of the week.
type HabitWeek struct {
	Start calendar.Date
	Done  int
	Met   bool
	Days  []HabitDay
}

type HabitDay struct {
	Date      calendar.Date
	Scheduled bool
	Done      bool
}

// doneDays holds the days a task was checked in, by Date.String().
type doneDays map[string]bool

// BuildHabits builds the habits of the recurring tasks for the weeks weeks
// up to the one that contains day. Tasks created after that week are left out.
func BuildHabits(day calendar.Date, weeks int, tasks []Task, checkIns []CheckIn) []Habit {
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
		done[c.TaskID][c.Date.String()] = true
	}

	habits := []Habit{}
	for _, t := range tasks {
		if t.Kind == RecurringTask && !t.CreatedOn.After(end) {
			habits = append(habits, buildHabit(t, ref, weeks, done[t.ID]))
		}
	}
	return habits
}

func buildHabit(t Task, ref calendar.Date, weeks int, done doneDays) Habit {
	h := Habit{Task: t, Streak: streak(t, ref, done)}
	first := t.CreatedOn.StartOfWeek()
	for i := weeks - 1; i >= 0; i-- {
		if start := ref.AddDays(-7 * i); !start.Before(first) {
			h.Weeks = append(h.Weeks, habitWeek(t, start, done))
		}
	}
	return h
}

func habitWeek(t Task, start calendar.Date, done doneDays) HabitWeek {
	w := HabitWeek{Start: start, Days: make([]HabitDay, 0, 7)}
	for i := range 7 {
		d := start.AddDays(i)
		day := HabitDay{Date: d, Scheduled: t.Weekdays.Has(d.Weekday()), Done: done[d.String()]}
		if day.Done {
			w.Done++
		}
		w.Days = append(w.Days, day)
	}
	w.Met = w.Done >= t.TimesPerWeek
	return w
}

// streak walks back from the reference week, over the whole history, until a
// week that was not met or the week the task was created on.
func streak(t Task, ref calendar.Date, done doneDays) int {
	n := 0
	if met(t, ref, done) {
		n = 1
	}
	first := t.CreatedOn.StartOfWeek()
	for start := ref.AddDays(-7); !start.Before(first) && met(t, start, done); start = start.AddDays(-7) {
		n++
	}
	return n
}

func met(t Task, start calendar.Date, done doneDays) bool {
	count := 0
	for i := range 7 {
		if done[start.AddDays(i).String()] {
			count++
		}
	}
	return count >= t.TimesPerWeek
}
