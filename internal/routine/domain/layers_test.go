package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/domainerr"
)

// workday is work from 09:00 to 18:00 with lunch and a snack inside it, the
// two layers of a weekday.
func workday(t *testing.T) (work, lunch, snack FixedTask) {
	t.Helper()
	days := weekdays(t, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday)
	work = FixedTask{ID: uuid.New(), Title: "Olist", Weekdays: days, Start: at(9), End: at(18)}
	lunch = FixedTask{ID: uuid.New(), Title: "Almoço", Weekdays: days, Start: at(12), End: at(13)}
	snack = FixedTask{ID: uuid.New(), Title: "Lanche", Weekdays: days, Start: at(15), End: at(15) + 30}
	return work, lunch, snack
}

func dated(title string, date calendar.Date, start, end calendar.TimeOfDay) FixedTask {
	return FixedTask{ID: uuid.New(), Title: title, Date: date, Start: start, End: end}
}

// overlapMessage returns the message of a schedule_overlap error, failing
// the test on any other error.
func overlapMessage(t *testing.T, err error) string {
	t.Helper()
	var de *domainerr.Error
	require.True(t, errors.As(err, &de), "want a schedule_overlap error, got %v", err)
	assert.Equal(t, domainerr.Conflict, de.Kind)
	assert.Equal(t, ScheduleOverlapCode, de.Code)
	return de.Message
}

func TestASecondLayerFitsAndAThirdIsRefused(t *testing.T) {
	work, lunch, snack := workday(t)
	meeting := dated("Reunião", day(2), at(15), at(16))
	s := Schedule{Fixed: []FixedTask{work, lunch, snack}}

	assert.NoError(t, s.CheckWeekly(lunch.ID, monday))
	assert.NoError(t, s.CheckWeekly(snack.ID, monday))
	s.Fixed = append(s.Fixed, meeting)
	err := s.CheckDates(meeting.ID, day(2), day(3))

	assert.Equal(t, "Nesse horário já há duas tarefas: Olist e Lanche (qua 28/10, 15:00–15:30).", overlapMessage(t, err))
}

func TestPartialOverlapCountsTheSame(t *testing.T) {
	work, _, _ := workday(t)
	class := dated("Faculdade", day(0), at(17)+30, at(19))
	gymTask := PlannableTask{ID: uuid.New(), Title: "Academia", Kind: RecurringKind, Duration: time.Hour}
	gymSession := Session{TaskID: gymTask.ID, Date: day(0), Start: at(17) + 55, End: at(18) + 55}
	s := Schedule{Fixed: []FixedTask{work, class}, Plannable: []PlannableTask{gymTask}}

	assert.NoError(t, s.CheckDates(class.ID, day(0)), "leaving work early is two layers")
	s.Sessions = []Session{gymSession}
	err := s.CheckDates(gymTask.ID, day(0))

	assert.Equal(t, "Nesse horário já há duas tarefas: Olist e Faculdade (seg 26/10, 17:55–18:00).", overlapMessage(t, err))
}

func TestBlocksThatOnlyTouchDoNotOverlap(t *testing.T) {
	work, lunch, _ := workday(t)
	meeting := dated("Reunião", day(2), at(13), at(14))

	err := Schedule{Fixed: []FixedTask{work, lunch, meeting}}.CheckDates(meeting.ID, day(2))

	assert.NoError(t, err)
}

func TestTwoThingsAtOnceFitWithoutABaseLayer(t *testing.T) {
	lunch := dated("Almoço", day(5), at(12), at(13))
	meeting := dated("Reunião", day(5), at(12)+30, at(13)+30)

	err := Schedule{Fixed: []FixedTask{lunch, meeting}}.CheckDates(meeting.ID, day(5))

	assert.NoError(t, err, "on Saturday the user picks which one to do")
}

func TestOvernightBlocksCountOnTheNextDay(t *testing.T) {
	sleep := FixedTask{ID: uuid.New(), Title: "Dormir", Weekdays: weekdays(t, time.Sunday), Start: at(23), End: at(7)}
	run := PlannableTask{ID: uuid.New(), Title: "Corrida", Kind: RecurringKind}
	reading := PlannableTask{ID: uuid.New(), Title: "Leitura", Kind: RecurringKind}
	s := Schedule{
		Fixed:     []FixedTask{sleep},
		Plannable: []PlannableTask{run, reading},
		Sessions: []Session{
			{TaskID: run.ID, Date: day(0), Start: at(6), End: at(7)},
			{TaskID: reading.ID, Date: day(0), Start: at(6) + 30, End: at(7) + 30},
		},
	}

	err := s.CheckDates(reading.ID, day(0))

	assert.Equal(t, "Nesse horário já há duas tarefas: Dormir e Corrida (seg 26/10, 06:30–07:00).", overlapMessage(t, err))
}

func TestSkippedOccurrencesDoNotCount(t *testing.T) {
	work, lunch, _ := workday(t)
	meeting := dated("Reunião", day(2), at(12), at(13))
	skipped := Adjustment{TaskID: lunch.ID, Date: day(2), Skipped: true}

	err := Schedule{Fixed: []FixedTask{work, lunch, meeting}, Adjustments: []Adjustment{skipped}}.CheckDates(meeting.ID, day(2))

	assert.NoError(t, err)
}

func TestOverlapsWithoutTheChangedTaskAreLeftAlone(t *testing.T) {
	work, lunch, snack := workday(t)
	extra := dated("Extra", day(2), at(12), at(13))

	err := Schedule{Fixed: []FixedTask{work, lunch, snack, extra}}.CheckWeekly(snack.ID, monday)

	assert.NoError(t, err, "the old overlap at noon does not block the snack")
}

func TestCheckWeeklyLooksAtWeeksWithSessions(t *testing.T) {
	work, _, _ := workday(t)
	meeting := FixedTask{ID: uuid.New(), Title: "Reunião", Weekdays: weekdays(t, time.Thursday), Start: at(15), End: at(16)}
	english := PlannableTask{ID: uuid.New(), Title: "Inglês", Kind: RecurringKind}
	inThreeWeeks := day(3).AddDays(21)
	s := Schedule{
		Fixed: []FixedTask{work, meeting}, Plannable: []PlannableTask{english},
		Sessions: []Session{{TaskID: english.ID, Date: inThreeWeeks, Start: at(15), End: at(16)}},
	}

	err := s.CheckWeekly(meeting.ID, monday)

	assert.Equal(t, "Nesse horário já há duas tarefas: Olist e Inglês (qui 19/11, 15:00–16:00).", overlapMessage(t, err))
}

func TestCheckWeeklyLooksAtAWeekWithoutChanges(t *testing.T) {
	work, lunch, _ := workday(t)
	meeting := FixedTask{ID: uuid.New(), Title: "Reunião", Weekdays: weekdays(t, time.Wednesday), Start: at(12), End: at(13)}
	workSkipped := Adjustment{TaskID: work.ID, Date: day(2), Skipped: true}
	s := Schedule{Fixed: []FixedTask{work, lunch, meeting}, Adjustments: []Adjustment{workSkipped}}

	err := s.CheckWeekly(meeting.ID, monday)

	assert.Equal(t, "Nesse horário já há duas tarefas: Olist e Almoço (qua 04/11, 12:00–13:00).", overlapMessage(t, err),
		"work is skipped only this week")
}
