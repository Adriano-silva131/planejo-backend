package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// The user's week of Monday 2026-11-09, seen on Saturday the 14th.
var (
	nov9     = calendar.NewDate(2026, time.November, 9)
	saturday = nov9.AddDays(5)
)

type habitCase struct {
	task  PlannableTask
	habit Habit
}

func recurring(t *testing.T, title string, target int, period calendar.Period, days ...time.Weekday) habitCase {
	t.Helper()
	task := PlannableTask{ID: uuid.New(), Title: title, Area: "Programação", Kind: RecurringKind, Duration: time.Hour, Period: period}
	return habitCase{task: task, habit: Habit{TaskID: task.ID, Target: target, Weekdays: weekdays(t, days...), Since: nov9.AddDays(-30)}}
}

type userWeek struct {
	aws, ddia, english, reading, run, longRun, gym habitCase
}

func newUserWeek(t *testing.T) userWeek {
	t.Helper()
	mon, tue, wed, thu, fri, sat, sun := time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday
	return userWeek{
		aws:     recurring(t, "AWS", 3, calendar.Evening, mon, wed, fri),
		ddia:    recurring(t, "DDIA", 3, calendar.Evening, tue, thu, sat),
		english: recurring(t, "Inglês", 5, calendar.Evening, mon, tue, wed, thu, fri),
		reading: recurring(t, "Leitura", 5, calendar.Evening, mon, tue, wed, thu, fri),
		run:     recurring(t, "Corrida pós treino", 3, "", tue, thu, fri),
		longRun: recurring(t, "Corrida longa", 2, calendar.Afternoon, sat, sun),
		gym:     recurring(t, "Academia", 4, calendar.Morning, tue, wed, thu, fri),
	}
}

func (u userWeek) all() []habitCase {
	return []habitCase{u.aws, u.ddia, u.english, u.reading, u.run, u.longRun, u.gym}
}

func (u userWeek) checkIns() []CheckIn {
	return []CheckIn{
		{TaskID: u.gym.task.ID, Date: nov9.AddDays(1)},
		{TaskID: u.gym.task.ID, Date: nov9.AddDays(3), Start: calendar.SomeTime(at(7))},
		{TaskID: u.gym.task.ID, Date: nov9.AddDays(4)},
		{TaskID: u.run.task.ID, Date: nov9.AddDays(3), Start: calendar.SomeTime(at(19))},
		{TaskID: u.run.task.ID, Date: nov9.AddDays(4)},
	}
}

func plan(today calendar.Date, cases []habitCase, sessions []Session, checkIns []CheckIn) Week {
	var tasks []PlannableTask
	p := WeekProgress{CheckIns: checkIns}
	for _, c := range cases {
		tasks = append(tasks, c.task)
		p.Habits = append(p.Habits, c.habit)
	}
	w := BuildWeek(nov9, nil, nil, tasks, sessions)
	w.Plan(today, tasks, sessions, p)
	return w
}

// planned lists "title source period done" by day offset.
func planned(w Week) map[int][]string {
	out := map[int][]string{}
	for i, d := range w.Days {
		for _, p := range d.Plans {
			done := ""
			if p.Done {
				done = " done"
			}
			out[i] = append(out[i], p.Title+" "+string(p.Source)+" "+string(p.Period)+done)
		}
	}
	return out
}

func toPlace(w Week) map[string][2]int {
	out := map[string][2]int{}
	for _, t := range w.ToPlace {
		out[t.Title] = [2]int{t.Remaining, t.Missed}
	}
	return out
}

func TestPlanTheUsersWeekOnSaturday(t *testing.T) {
	u := newUserWeek(t)

	w := plan(saturday, u.all(), nil, u.checkIns())

	assert.Equal(t, map[int][]string{
		1: {"Academia checkin morning done"},
		3: {"Academia checkin morning done", "Corrida pós treino checkin evening done"},
		4: {"Academia checkin morning done", "Corrida pós treino checkin  done"},
		5: {"Corrida longa usual afternoon", "DDIA usual evening"},
		6: {"Corrida longa usual afternoon"},
	}, planned(w))
	assert.Equal(t, map[string][2]int{
		"AWS": {3, 3}, "DDIA": {2, 2}, "Inglês": {5, 5}, "Leitura": {5, 5}, "Corrida pós treino": {1, 1}, "Academia": {1, 1},
	}, toPlace(w), "Corrida longa has its two days")
}

func TestPlanTheUsersWeekOnMonday(t *testing.T) {
	u := newUserWeek(t)

	w := plan(nov9, u.all(), nil, nil)

	assert.Empty(t, w.ToPlace)
	assert.Equal(t, []string{"AWS usual evening", "Inglês usual evening", "Leitura usual evening"}, planned(w)[0])
	assert.Equal(t, []string{"Academia usual morning", "DDIA usual evening", "Inglês usual evening", "Leitura usual evening", "Corrida pós treino usual "}, planned(w)[1])
	assert.Equal(t, []string{"Corrida longa usual afternoon"}, planned(w)[6])
}

func TestASessionInAPeriodCountsAsPlaced(t *testing.T) {
	u := newUserWeek(t)
	sunday := Session{TaskID: u.aws.task.ID, Date: nov9.AddDays(6), Period: calendar.Evening}

	w := plan(saturday, []habitCase{u.aws}, []Session{sunday}, nil)

	assert.Equal(t, map[int][]string{6: {"AWS session evening"}}, planned(w))
	assert.Equal(t, map[string][2]int{"AWS": {2, 2}}, toPlace(w))
}

func TestACheckInOnAnotherDayMakesUp(t *testing.T) {
	u := newUserWeek(t)
	u.aws.habit.Target = 2
	tuesday := nov9.AddDays(1)

	w := plan(tuesday, []habitCase{u.aws}, nil, []CheckIn{{TaskID: u.aws.task.ID, Date: tuesday}})

	assert.Equal(t, map[int][]string{1: {"AWS checkin evening done"}, 2: {"AWS usual evening"}}, planned(w),
		"Friday is not foreseen anymore")
	assert.Empty(t, w.ToPlace, "Monday was missed, but the week asks for nothing more")
}

func TestARecurringTaskWithoutUsualDays(t *testing.T) {
	u := newUserWeek(t)
	free := recurring(t, "Violão", 3, "")
	sessions := []Session{
		{TaskID: free.task.ID, Date: nov9.AddDays(2), Start: at(19), End: at(20)}, // missed
		{TaskID: free.task.ID, Date: nov9.AddDays(6), Period: calendar.Morning},
	}
	checkIns := []CheckIn{{TaskID: free.task.ID, Date: nov9}}

	w := plan(saturday, []habitCase{free, u.aws}, sessions, checkIns)

	assert.Equal(t, []string{"Violão session morning"}, planned(w)[6])
	assert.Equal(t, [2]int{1, 1}, toPlace(w)["Violão"], "3 − 1 done − 1 session ahead")
}

func TestPlanMarksTheSessionsDone(t *testing.T) {
	u := newUserWeek(t)
	sessions := []Session{
		{TaskID: u.gym.task.ID, Date: nov9.AddDays(1), Start: at(7), End: at(8)},
		{TaskID: u.gym.task.ID, Date: nov9.AddDays(2), Start: at(7), End: at(8)},
		{TaskID: u.gym.task.ID, Date: nov9.AddDays(4), Period: calendar.Morning},
	}

	w := plan(saturday, []habitCase{u.gym}, sessions, u.checkIns())

	require.Len(t, w.Days[1].Blocks, 1)
	assert.True(t, w.Days[1].Blocks[0].Done)
	assert.False(t, w.Days[2].Blocks[0].Done)
	assert.Equal(t, []string{"Academia session morning done"}, planned(w)[4], "a session day is no check-in plan")
	assert.Equal(t, [2]int{1, 1}, toPlace(w)["Academia"], "Wednesday's session was missed")
}

func TestPlanBeforeTheTaskExisted(t *testing.T) {
	u := newUserWeek(t)
	u.aws.habit.Since = nov9.AddDays(3)
	u.aws.habit.Target = 2

	w := plan(saturday, []habitCase{u.aws}, nil, nil)

	assert.Equal(t, map[string][2]int{"AWS": {2, 1}}, toPlace(w), "only Friday was missed")
}

func TestPlanAPastWeekForeseesNothing(t *testing.T) {
	u := newUserWeek(t)

	w := plan(nov9.AddDays(7), []habitCase{u.aws}, nil, nil)

	assert.Empty(t, planned(w))
	assert.Equal(t, map[string][2]int{"AWS": {3, 3}}, toPlace(w))
}

func TestPlanAFutureWeekForeseesEveryUsualDay(t *testing.T) {
	u := newUserWeek(t)

	w := plan(nov9.AddDays(-3), []habitCase{u.longRun, u.aws}, nil, nil)

	assert.Empty(t, w.ToPlace)
	assert.Len(t, planned(w), 5)
}

func TestPlanOneOffTasks(t *testing.T) {
	deadline := nov9.AddDays(11)
	report := PlannableTask{ID: uuid.New(), Title: "Relatório", Area: "Trabalho", Kind: OneOffKind, Deadline: deadline}
	placed := PlannableTask{ID: uuid.New(), Title: "Imposto", Area: "Pessoal", Kind: OneOffKind, Period: calendar.Morning}
	done := PlannableTask{ID: uuid.New(), Title: "Mudança", Area: "Pessoal", Kind: OneOffKind}
	tasks := []PlannableTask{report, placed, done}
	sessions := []Session{{TaskID: placed.ID, Date: nov9.AddDays(2), Period: calendar.Afternoon}}
	p := WeekProgress{
		CheckIns:  []CheckIn{{TaskID: placed.ID, Date: nov9.AddDays(2)}},
		Completed: map[uuid.UUID]bool{placed.ID: true, done.ID: true},
	}

	w := BuildWeek(nov9, nil, nil, tasks, sessions)
	w.Plan(saturday, tasks, sessions, p)

	assert.Equal(t, map[int][]string{2: {"Imposto session afternoon done"}}, planned(w), "one-off tasks have no check-in plans")
	assert.Equal(t, []ToPlace{{
		TaskID: report.ID, Title: "Relatório", Area: "Trabalho", Kind: OneOffKind, Deadline: deadline, Remaining: 1,
	}}, w.ToPlace)
}
