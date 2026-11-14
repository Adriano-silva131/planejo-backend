package domain

import (
	"cmp"
	"slices"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// EndOfDay is 24:00, the end of a block that goes on the next day.
const EndOfDay calendar.TimeOfDay = 24 * 60

// Week is the routine of one week, Monday to Sunday, built from the fixed
// tasks, their adjustments and the sessions the user placed that week.
type Week struct {
	Start calendar.Date
	End   calendar.Date
	Days  []Day
}

type Day struct {
	Date   calendar.Date
	Blocks []Block
}

// Kind is the kind of task a block comes from.
type Kind string

const (
	FixedKind     Kind = "fixed"
	RecurringKind Kind = "recurring"
	OneOffKind    Kind = "one_off"
)

// Block is the part of an occurrence of a fixed task that falls on one day,
// or a session the user placed. An occurrence that crosses midnight is split
// in two blocks, one on each day. Adjusted is only set on fixed tasks moved
// that week.
type Block struct {
	TaskID   uuid.UUID
	Title    string
	Area     string
	Kind     Kind
	Start    calendar.TimeOfDay
	End      calendar.TimeOfDay
	Adjusted bool

	ContinuesFromPreviousDay bool
	ContinuesNextDay         bool
}

type occurrenceKey struct {
	taskID uuid.UUID
	date   string
}

// BuildWeek builds the week that contains day. It starts on the Sunday
// before, whose overnight occurrences end on Monday. Adjustments of dates
// that are no longer occurrences are ignored, and so are sessions of tasks
// not in plannable and sessions without a time.
func BuildWeek(
	day calendar.Date, fixed []FixedTask, adjustments []Adjustment, plannable []PlannableTask, sessions []Session,
) Week {
	start := day.StartOfWeek()
	w := Week{Start: start, End: start.AddDays(6), Days: make([]Day, 7)}
	for i := range w.Days {
		w.Days[i].Date = start.AddDays(i)
	}

	byOccurrence := make(map[occurrenceKey]Adjustment, len(adjustments))
	for _, a := range adjustments {
		byOccurrence[occurrenceKey{a.TaskID, a.Date.String()}] = a
	}

	for offset := -1; offset < len(w.Days); offset++ {
		date := start.AddDays(offset)
		for _, t := range fixed {
			if b, ok := occurrence(t, date, byOccurrence); ok {
				w.place(offset, b)
			}
		}
	}
	w.placeSessions(plannable, sessions)

	for i := range w.Days {
		slices.SortStableFunc(w.Days[i].Blocks, func(a, b Block) int {
			return cmp.Or(cmp.Compare(a.Start, b.Start), cmp.Compare(a.Title, b.Title))
		})
	}
	return w
}

// occurrence returns the block of t starting on date, with its adjustment
// applied, unless t does not happen that day or the occurrence was skipped.
func occurrence(t FixedTask, date calendar.Date, adjustments map[occurrenceKey]Adjustment) (Block, bool) {
	if !t.OccursOn(date) {
		return Block{}, false
	}
	b := Block{TaskID: t.ID, Title: t.Title, Area: t.Area, Kind: FixedKind, Start: t.Start, End: t.End}
	a, ok := adjustments[occurrenceKey{t.ID, date.String()}]
	if !ok {
		return b, true
	}
	if a.Skipped {
		return Block{}, false
	}
	b.Start, b.End = a.Start, a.End
	b.Adjusted = a.Start != t.Start || a.End != t.End
	return b, true
}

// place adds b, which starts on the day at offset, splitting it at midnight.
// An occurrence ending exactly at midnight stays on its day, up to 24:00.
func (w *Week) place(offset int, b Block) {
	switch {
	case b.End > b.Start:
		w.add(offset, b)
	case b.End == 0:
		b.End = EndOfDay
		w.add(offset, b)
	default:
		first, second := b, b
		first.End, first.ContinuesNextDay = EndOfDay, true
		second.Start, second.ContinuesFromPreviousDay = 0, true
		w.add(offset, first)
		w.add(offset+1, second)
	}
}

// add ignores offsets outside the week, such as the Sunday before.
func (w *Week) add(offset int, b Block) {
	if offset >= 0 && offset < len(w.Days) {
		w.Days[offset].Blocks = append(w.Days[offset].Blocks, b)
	}
}

// placeSessions adds the timed sessions of this week as blocks of their
// tasks. A session only in a period has no time to take, so it is no block.
func (w *Week) placeSessions(plannable []PlannableTask, sessions []Session) {
	byID := make(map[uuid.UUID]PlannableTask, len(plannable))
	for _, t := range plannable {
		byID[t.ID] = t
	}
	for _, s := range sessions {
		if t, ok := byID[s.TaskID]; ok && s.Timed() {
			w.add(s.Date.DaysSince(w.Start), Block{TaskID: t.ID, Title: t.Title, Area: t.Area, Kind: t.Kind, Start: s.Start, End: s.End})
		}
	}
}
