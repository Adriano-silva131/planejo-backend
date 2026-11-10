package domain

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/domainerr"
)

// Schedule is what takes time in the user's routine: the fixed tasks, the
// adjustments of their occurrences and the sessions of the other tasks.
type Schedule struct {
	Fixed       []FixedTask
	Plannable   []PlannableTask
	Adjustments []Adjustment
	Sessions    []Session
}

func (s Schedule) Week(day calendar.Date) Week {
	return BuildWeek(day, s.Fixed, s.Adjustments, s.Plannable, s.Sessions)
}

// CheckDates fails with a schedule_overlap error when a block of taskID on
// one of dates is more than MaxLayers things at once.
func (s Schedule) CheckDates(taskID uuid.UUID, dates ...calendar.Date) error {
	checked := map[calendar.Date]bool{}
	for _, d := range dates {
		monday := d.StartOfWeek()
		if checked[monday] {
			continue
		}
		checked[monday] = true
		if o, ok := s.Week(monday).OverlapOf(taskID, dates...); ok {
			return overlapError(o)
		}
	}
	return nil
}

// CheckWeekly is CheckDates for a task that repeats every week, from the
// week of from on: that week, every later week with something dated in it,
// and the week after them, which has only what repeats every week.
func (s Schedule) CheckWeekly(taskID uuid.UUID, from calendar.Date) error {
	for _, monday := range s.weeksToCheck(from.StartOfWeek()) {
		if o, ok := s.Week(monday).OverlapOf(taskID); ok {
			return overlapError(o)
		}
	}
	return nil
}

func (s Schedule) weeksToCheck(first calendar.Date) []calendar.Date {
	weeks := map[calendar.Date]bool{first: true}
	add := func(d calendar.Date) {
		// An overnight block also takes the next day, maybe in the next week.
		for _, day := range []calendar.Date{d, d.AddDays(1)} {
			if monday := day.StartOfWeek(); !monday.Before(first) {
				weeks[monday] = true
			}
		}
	}
	for _, a := range s.Adjustments {
		add(a.Date)
	}
	for _, session := range s.Sessions {
		add(session.Date)
	}
	for _, t := range s.Fixed {
		if !t.Date.IsZero() {
			add(t.Date)
		}
	}

	out := make([]calendar.Date, 0, len(weeks)+1)
	for monday := range weeks {
		out = append(out, monday)
	}
	slices.SortFunc(out, func(a, b calendar.Date) int { return a.Time().Compare(b.Time()) })
	return append(out, out[len(out)-1].AddDays(7))
}

var weekdayAbbreviations = [...]string{
	time.Sunday: "dom", time.Monday: "seg", time.Tuesday: "ter", time.Wednesday: "qua",
	time.Thursday: "qui", time.Friday: "sex", time.Saturday: "sáb",
}

func overlapError(o Overlap) error {
	titles := make([]string, 0, len(o.Others))
	for _, b := range o.Others {
		titles = append(titles, b.Title)
	}
	count := fmt.Sprintf("%d tarefas", len(titles))
	if len(titles) == MaxLayers {
		count = "duas tarefas"
	}
	day := o.Date.Time()
	message := fmt.Sprintf("Nesse horário já há %s: %s (%s %02d/%02d, %s–%s).",
		count, joinTitles(titles), weekdayAbbreviations[o.Date.Weekday()], day.Day(), int(day.Month()),
		o.Start, o.End)
	return domainerr.New(domainerr.Conflict, ScheduleOverlapCode, message)
}

// joinTitles lists titles as in "A, B e C".
func joinTitles(titles []string) string {
	if len(titles) < 2 {
		return strings.Join(titles, "")
	}
	return strings.Join(titles[:len(titles)-1], ", ") + " e " + titles[len(titles)-1]
}
