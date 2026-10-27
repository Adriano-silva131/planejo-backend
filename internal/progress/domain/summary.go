package domain

import (
	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// WeekSummary is the progress of one week, Monday to Sunday. Fixed tasks are
// left out: they always happen.
type WeekSummary struct {
	Start     calendar.Date
	End       calendar.Date
	Recurring []RecurringProgress
	OneOff    []OneOffProgress
}

type RecurringProgress struct {
	Task Task
	// Expected is how many times the task is set for in a week; Done holds
	// the days it was checked in, which may be other days of the week.
	Expected int
	Done     []calendar.Date
}

type OneOffProgress struct {
	Task        Task
	WeekMinutes int
	// DoneMinutes adds up every check-in until the end of the week.
	DoneMinutes int
	// CompletedOn is zero while the task is open.
	CompletedOn calendar.Date
}

// Summarize builds the summary of the week that contains day. One-off tasks
// show up while open and in the week they are completed.
func Summarize(day calendar.Date, tasks []Task, checkIns []CheckIn) WeekSummary {
	s := WeekSummary{Start: day.StartOfWeek()}
	s.End = s.Start.AddDays(6)

	byTask := map[uuid.UUID][]CheckIn{}
	for _, c := range checkIns {
		if !c.Date.After(s.End) {
			byTask[c.TaskID] = append(byTask[c.TaskID], c)
		}
	}

	for _, t := range tasks {
		switch t.Kind {
		case RecurringTask:
			s.Recurring = append(s.Recurring, s.recurring(t, byTask[t.ID]))
		case OneOffTask:
			if p := s.oneOff(t, byTask[t.ID]); p.CompletedOn.IsZero() || !p.CompletedOn.Before(s.Start) {
				s.OneOff = append(s.OneOff, p)
			}
		}
	}
	return s
}

func (s WeekSummary) contains(d calendar.Date) bool {
	return !d.Before(s.Start) && !d.After(s.End)
}

func (s WeekSummary) recurring(t Task, checkIns []CheckIn) RecurringProgress {
	p := RecurringProgress{Task: t, Expected: t.TimesPerWeek}
	for _, c := range checkIns {
		if s.contains(c.Date) {
			p.Done = append(p.Done, c.Date)
		}
	}
	return p
}

func (s WeekSummary) oneOff(t Task, checkIns []CheckIn) OneOffProgress {
	p := OneOffProgress{Task: t}
	for _, c := range checkIns {
		p.DoneMinutes += c.Minutes
		if s.contains(c.Date) {
			p.WeekMinutes += c.Minutes
		}
		if c.Completed && (p.CompletedOn.IsZero() || c.Date.Before(p.CompletedOn)) {
			p.CompletedOn = c.Date
		}
	}
	return p
}
