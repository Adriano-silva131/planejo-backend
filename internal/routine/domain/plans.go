package domain

import (
	"cmp"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// PlanSource is why a task is planned on a day.
type PlanSource string

const (
	// UsualPlan is a day the user usually does a recurring task, foreseen
	// while the week still asks for it.
	UsualPlan PlanSource = "usual"
	// SessionPlan is a session placed only in a period, without a time.
	SessionPlan PlanSource = "session"
	// CheckInPlan is a day a recurring task was done without a session.
	CheckInPlan PlanSource = "checkin"
)

// Plan is a recurring or one-off task on a day, without a time. Period may be
// empty, and so may Duration, on a one-off task.
type Plan struct {
	TaskID   uuid.UUID
	Title    string
	Area     string
	Kind     Kind
	Source   PlanSource
	Period   calendar.Period
	Duration time.Duration
	Done     bool
}

// ToPlace is a task that still asks for Remaining days this week, Missed of
// them for days already gone. Deadline is only set on one-off tasks.
type ToPlace struct {
	TaskID    uuid.UUID
	Title     string
	Area      string
	Kind      Kind
	Duration  time.Duration
	Period    calendar.Period
	Deadline  calendar.Date
	Remaining int
	Missed    int
}

// Plan fills the plans of the days and what is left to place, as seen on
// today. plannable and sessions are those the week was built with.
//
// A recurring task asks for its habit's Target days. Its check-ins count
// first, then its sessions from today on, and then the days it is usually
// done, from today on, become usual plans while the week still asks for more.
// What is left is to place. A one-off task is to place while open and without
// a session in the week.
func (w *Week) Plan(today calendar.Date, plannable []PlannableTask, sessions []Session, p WeekProgress) {
	habits := make(map[uuid.UUID]Habit, len(p.Habits))
	for _, h := range p.Habits {
		habits[h.TaskID] = h
	}
	sessionsByTask := byTaskAndDate(w, sessions, func(s Session) (uuid.UUID, calendar.Date) { return s.TaskID, s.Date })
	checkInsByTask := byTaskAndDate(w, p.CheckIns, func(c CheckIn) (uuid.UUID, calendar.Date) { return c.TaskID, c.Date })

	for _, t := range plannable {
		tw := taskWeek{week: w, task: t, today: today, sessions: sessionsByTask[t.ID], checkIns: checkInsByTask[t.ID]}
		tw.markDone()
		tw.planSessions()
		switch {
		case t.Kind == RecurringKind:
			tw.planCheckIns()
			tw.planRecurring(habits[t.ID])
		case !t.CreatedOn.After(w.End) && !p.Completed[t.ID] && len(tw.sessions) == 0:
			w.ToPlace = append(w.ToPlace, tw.toPlace(1, 0))
		}
	}

	for i := range w.Days {
		slices.SortStableFunc(w.Days[i].Plans, func(a, b Plan) int {
			return cmp.Or(cmp.Compare(periodRank(a.Period), periodRank(b.Period)), cmp.Compare(a.Title, b.Title))
		})
	}
}

// byTaskAndDate groups the items dated in the week by task and day.
func byTaskAndDate[T any](w *Week, items []T, key func(T) (uuid.UUID, calendar.Date)) map[uuid.UUID]map[calendar.Date]T {
	out := map[uuid.UUID]map[calendar.Date]T{}
	for _, item := range items {
		taskID, date := key(item)
		if date.Before(w.Start) || date.After(w.End) {
			continue
		}
		if out[taskID] == nil {
			out[taskID] = map[calendar.Date]T{}
		}
		out[taskID][date] = item
	}
	return out
}

func periodRank(p calendar.Period) int {
	switch p {
	case calendar.Morning:
		return 0
	case calendar.Afternoon:
		return 1
	case calendar.Evening:
		return 2
	}
	return 3
}

// taskWeek plans one task in the week, from its sessions and check-ins there.
type taskWeek struct {
	week     *Week
	task     PlannableTask
	today    calendar.Date
	sessions map[calendar.Date]Session
	checkIns map[calendar.Date]CheckIn
}

func (tw taskWeek) doneOn(d calendar.Date) bool {
	_, ok := tw.checkIns[d]
	return ok
}

// markDone marks the blocks of the task's sessions on days it was done.
func (tw taskWeek) markDone() {
	for i := range tw.week.Days {
		day := &tw.week.Days[i]
		if !tw.doneOn(day.Date) {
			continue
		}
		for j := range day.Blocks {
			if b := &day.Blocks[j]; b.TaskID == tw.task.ID && b.Kind != FixedKind {
				b.Done = true
			}
		}
	}
}

func (tw taskWeek) add(d calendar.Date, source PlanSource, period calendar.Period) {
	t := tw.task
	plan := Plan{
		TaskID: t.ID, Title: t.Title, Area: t.Area, Kind: t.Kind, Source: source, Period: period,
		Duration: t.Duration, Done: tw.doneOn(d),
	}
	day := &tw.week.Days[d.DaysSince(tw.week.Start)]
	day.Plans = append(day.Plans, plan)
}

// planSessions plans the sessions placed only in a period.
func (tw taskWeek) planSessions() {
	for d, s := range tw.sessions {
		if !s.Timed() {
			tw.add(d, SessionPlan, s.Period)
		}
	}
}

// planCheckIns plans the days done without a session, in the period the task
// was started in, or else the one the task prefers.
func (tw taskWeek) planCheckIns() {
	for d, c := range tw.checkIns {
		if _, ok := tw.sessions[d]; ok {
			continue
		}
		period := tw.task.Period
		if start, ok := c.Start.Get(); ok {
			period = calendar.PeriodOf(start)
		}
		tw.add(d, CheckInPlan, period)
	}
}

func (tw taskWeek) planRecurring(h Habit) {
	taken := len(tw.checkIns) + tw.sessionsAhead()
	usual := 0
	for _, day := range tw.week.Days {
		if taken+usual >= h.Target {
			break
		}
		if tw.usualOn(h, day.Date) {
			tw.add(day.Date, UsualPlan, tw.task.Period)
			usual++
		}
	}
	if remaining := h.Target - taken - usual; remaining > 0 {
		tw.week.ToPlace = append(tw.week.ToPlace, tw.toPlace(remaining, min(remaining, tw.missed(h))))
	}
}

// sessionsAhead counts the sessions from today on on days not yet done.
func (tw taskWeek) sessionsAhead() int {
	n := 0
	for d := range tw.sessions {
		if !d.Before(tw.today) && !tw.doneOn(d) {
			n++
		}
	}
	return n
}

// usualOn reports whether d, from today on, is a day the task is usually
// done that has nothing of the task yet.
func (tw taskWeek) usualOn(h Habit, d calendar.Date) bool {
	_, placed := tw.sessions[d]
	return !d.Before(tw.today) && !d.Before(h.Since) && h.Weekdays.Has(d.Weekday()) && !placed && !tw.doneOn(d)
}

// missed counts the days before today, since the task exists, that were
// usual or had a session and were not done.
func (tw taskWeek) missed(h Habit) int {
	n := 0
	for _, day := range tw.week.Days {
		d := day.Date
		if !d.Before(tw.today) {
			break
		}
		_, placed := tw.sessions[d]
		if !d.Before(h.Since) && (h.Weekdays.Has(d.Weekday()) || placed) && !tw.doneOn(d) {
			n++
		}
	}
	return n
}

func (tw taskWeek) toPlace(remaining, missed int) ToPlace {
	t := tw.task
	return ToPlace{
		TaskID: t.ID, Title: t.Title, Area: t.Area, Kind: t.Kind, Duration: t.Duration, Period: t.Period,
		Deadline: t.Deadline, Remaining: remaining, Missed: missed,
	}
}
