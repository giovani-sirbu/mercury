package slowpattern

// Trigger reads a ladder's decline between its fills, the way the slow pattern
// decline decides, and answers whether it holds and the reading that decided.
//
// fills are the ladder's entry fills in the order it placed them, series the
// closed 1h bars sophos served. The newest fill is b, at depth len(fills), and
// its window ends at the bar BarOpen holds it in. The windows are the ones
// from an earlier fill a, at depth 1 up to len(fills) − SlowPatternMinDepthsBetween
// — b − a at least that many — each starting at the bar BarOpen holds fill a
// in, longest first, and the first that holds is the answer, a reading whose
// Reasons name it. Two fills in one bar share a window, so it is read once,
// and a fill without a stamp has no bar, so its window is skipped; a newest
// fill without one ends no window at all.
//
// No window holding, the answer is false with the longest readable reading,
// its Breaks naming what missed, or — none readable: too few fills, no stamp,
// no bars served, the bars of the fills not among them — a reading whose
// Breaks say why nothing was read. Fewer fills than SlowPatternMinDepthsBetween
// plus one are never read, whatever the market did.
//
// It reads the bars between the fills and the ones just before the first, so
// it is deterministic: the same fills and the bars up to the end bar give the
// same answer on any later tick, however many bars the series has since grown
// by.
func Trigger(fills []Fill, series Series) (Reading, bool) {
	depth := len(fills)
	if depth < SlowPatternMinDepthsBetween+1 {
		return Reading{ToDepth: depth, Breaks: []string{fewFillsBreak(depth)}}, false
	}
	endAt := BarOpen(fills[depth-1].At)
	if endAt == 0 {
		return Reading{FromDepth: 1, ToDepth: depth, Breaks: []string{unstampedBreak(depth)}}, false
	}

	var longest Reading
	var read, started bool
	var lastStartAt int64
	for from := 1; from <= depth-SlowPatternMinDepthsBetween; from++ {
		startAt := BarOpen(fills[from-1].At)
		if startAt == 0 || (started && startAt == lastStartAt) {
			continue
		}
		started, lastStartAt = true, startAt
		reading, readable := Read(series, startAt, endAt, from, depth)
		if !readable {
			continue
		}
		if reading.Holds {
			return reading, true
		}
		if !read {
			longest, read = reading, true
		}
	}
	if read {
		return longest, false
	}
	return Reading{FromDepth: 1, ToDepth: depth, Breaks: []string{unreadableBreak(series, depth)}}, false
}
