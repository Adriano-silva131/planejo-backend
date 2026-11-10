package domain

import (
	"cmp"
	"slices"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// MaxLayers is how many things may happen at the same time. The routine is
// thought of in layers: work from 09:00 to 18:00 is one layer, and lunch at
// 12:00 or a snack at 15:00 during work are a second one. A meeting at 15:00
// on top of the snack would be a third layer, so it is refused. The rule only
// counts how many blocks happen at each minute: two blocks that partly
// overlap, such as work until 18:00 and a class from 17:30, are two layers,
// and a block ending when another starts does not overlap it.
const MaxLayers = 2

// Overlap is a stretch of Date, from Start to End, where a block of the
// changed task makes more than MaxLayers things happen at once. Others are
// the blocks of other tasks happening then.
type Overlap struct {
	Date   calendar.Date
	Start  calendar.TimeOfDay
	End    calendar.TimeOfDay
	Others []Block
}

// OverlapOf returns the first stretch, on the given dates of the week or on
// any of them when none is given, where a block of taskID is one thing too
// many. Overlaps without the task are left alone.
func (w Week) OverlapOf(taskID uuid.UUID, dates ...calendar.Date) (Overlap, bool) {
	for _, d := range w.Days {
		if len(dates) > 0 && !slices.ContainsFunc(dates, d.Date.Equal) {
			continue
		}
		if o, ok := d.overlapOf(taskID); ok {
			return o, true
		}
	}
	return Overlap{}, false
}

type edge struct {
	at    calendar.TimeOfDay
	start bool
	block int
}

func (d Day) overlapOf(taskID uuid.UUID) (Overlap, bool) {
	edges := make([]edge, 0, 2*len(d.Blocks))
	for i, b := range d.Blocks {
		edges = append(edges, edge{at: b.Start, start: true, block: i}, edge{at: b.End, block: i})
	}
	// At the same minute, blocks end before others start.
	slices.SortFunc(edges, func(a, b edge) int {
		return cmp.Or(cmp.Compare(a.at, b.at), compareStart(a.start, b.start))
	})

	active := map[int]bool{}
	for i, e := range edges {
		if e.start {
			active[e.block] = true
		} else {
			delete(active, e.block)
		}
		last := i+1 == len(edges) || edges[i+1].at != e.at
		if !last || len(active) <= MaxLayers {
			continue
		}
		if others, ok := d.othersThan(taskID, active); ok {
			return Overlap{Date: d.Date, Start: e.at, End: edges[i+1].at, Others: others}, true
		}
	}
	return Overlap{}, false
}

func compareStart(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	}
	return -1
}

// othersThan returns the active blocks of other tasks, if a block of taskID
// is active.
func (d Day) othersThan(taskID uuid.UUID, active map[int]bool) ([]Block, bool) {
	var others []Block
	found := false
	for i, b := range d.Blocks {
		switch {
		case !active[i]:
		case b.TaskID == taskID:
			found = true
		default:
			others = append(others, b)
		}
	}
	return others, found
}
