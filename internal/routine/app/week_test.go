package app

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// fakeProgress stands in for the progress module.
type fakeProgress struct {
	habits    []domain.Habit
	checkIns  []domain.CheckIn
	completed map[uuid.UUID]bool

	monday, from, to calendar.Date
	asked            []uuid.UUID
}

func (p *fakeProgress) Habits(_ context.Context, _ uuid.UUID, monday calendar.Date) ([]domain.Habit, error) {
	p.monday = monday
	return p.habits, nil
}

func (p *fakeProgress) CheckIns(_ context.Context, _ uuid.UUID, from, to calendar.Date) ([]domain.CheckIn, error) {
	p.from, p.to = from, to
	return p.checkIns, nil
}

func (p *fakeProgress) Completed(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	p.asked = ids
	return p.completed, nil
}

func TestWeekPlansWhatHasNoTime(t *testing.T) {
	report := domain.PlannableTask{ID: uuid.New(), Title: "Relatório", Kind: domain.OneOffKind}
	progress := &fakeProgress{
		habits: []domain.Habit{{TaskID: gym.ID, Target: 2, Weekdays: calendar.Weekdays(1<<time.Monday | 1<<time.Friday)}},
	}
	tasks := &fakeTasks{plannable: []domain.PlannableTask{gym, report}}
	stores := Stores{Tasks: tasks, Adjustments: &fakeRepo{}, Sessions: &fakeSessions{}, Progress: progress}
	// Tuesday 02:00 in UTC is still Monday in São Paulo.
	tuesday := time.Date(2026, time.October, 27, 2, 0, 0, 0, time.UTC)
	r := NewRoutine(stores, fakeTx{}).WithClock(func() time.Time { return tuesday })
	loc, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)

	w, err := r.Week(calendar.WithLocation(t.Context(), loc), userID, monday.AddDays(3))

	require.NoError(t, err)
	assert.Equal(t, monday, progress.monday)
	assert.Equal(t, [2]calendar.Date{monday, monday.AddDays(6)}, [2]calendar.Date{progress.from, progress.to})
	assert.Equal(t, []uuid.UUID{report.ID}, progress.asked, "only one-off tasks are completed")
	require.Len(t, w.Days[0].Plans, 1, "Monday is still today")
	assert.Equal(t, domain.UsualPlan, w.Days[0].Plans[0].Source)
	require.Len(t, w.ToPlace, 1)
	assert.Equal(t, report.ID, w.ToPlace[0].TaskID, "gym has its two days")
}

func TestWeekWithoutOneOffTasksAsksNoCompletion(t *testing.T) {
	progress := &fakeProgress{}
	tasks := &fakeTasks{plannable: []domain.PlannableTask{gym}}
	r := NewRoutine(Stores{Tasks: tasks, Adjustments: &fakeRepo{}, Sessions: &fakeSessions{}, Progress: progress}, fakeTx{})

	_, err := r.Week(t.Context(), userID, monday)

	require.NoError(t, err)
	assert.Nil(t, progress.asked)
}
