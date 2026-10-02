package swingdecline

// Trigger reads a ladder's decline between its fills, the way the swing
// decline decides, and answers whether it holds and the reading that decided.
//
// fills are the ladder's entry fills in the order it placed them, series the
// closed 1h bars sophos served and trend the Trend Direction row of its SMC
// trend dashboard as of the series' last bar. The ladder triggers when a window
// holds on its shape AND the trend reads down on at least
// SwingDeclineMinSmcTrendDown rows (withTrend).
//
// The shape is read by readWindows. The newest fill is b, at depth len(fills),
// and its window ends at the bar BarOpen holds it in. The windows are the ones
// from an earlier fill a, at depth 1 up to len(fills) − SwingDeclineMinDepthsBetween
// — b − a at least that many — each starting at the bar BarOpen holds fill a
// in, longest first, and the first that holds is the answer, a reading whose
// Reasons name it. Two fills in one bar share a window, so it is read once,
// and a fill without a stamp has no bar, so its window is skipped; a newest
// fill without one ends no window at all.
//
// No window holding, the shape's answer is the longest readable reading, its
// Breaks naming what missed, or — none readable: too few fills, no stamp, no
// bars served, the bars of the fills not among them — a reading whose Breaks
// say why nothing was read. Fewer fills than SwingDeclineMinDepthsBetween plus
// one are never read, whatever the market did.
//
// The trend then decides the answer: met, the reading that held names it last
// among its Reasons; not met, or not read, the answer is false whatever the
// shape — a reading that held now names its window and the trend in its Breaks,
// one that did not names the trend after what missed.
//
// The shape reads the bars between the fills and the ones just before the
// first, so it is deterministic: the same fills and the bars up to the end bar
// give the same reading on any later tick, however many bars the series has
// since grown by. The trend is the tick's: it is the dashboard as of the
// series' last bar, so the same fills can trigger on a later tick whose trend
// has turned.
func Trigger(fills []Fill, series Series, trend Trend) (Reading, bool) {
	reading := readWindows(fills, series).withTrend(trend, SwingDeclineMinSmcTrendDown)
	return reading, reading.Holds
}

// readWindows is the shape's half of Trigger: the first window that holds,
// longest first, else the longest readable reading, else the reading whose
// Breaks say why nothing was read. The trend is not folded in.
func readWindows(fills []Fill, series Series) Reading {
	depth := len(fills)
	if depth < SwingDeclineMinDepthsBetween+1 {
		return Reading{ToDepth: depth, Breaks: []string{fewFillsBreak(depth)}}
	}
	endAt := BarOpen(fills[depth-1].At)
	if endAt == 0 {
		return Reading{FromDepth: 1, ToDepth: depth, Breaks: []string{unstampedBreak(depth)}}
	}

	var longest Reading
	var read, started bool
	var lastStartAt int64
	for from := 1; from <= depth-SwingDeclineMinDepthsBetween; from++ {
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
			return reading
		}
		if !read {
			longest, read = reading, true
		}
	}
	if read {
		return longest
	}
	return Reading{FromDepth: 1, ToDepth: depth, Breaks: []string{unreadableBreak(series, depth)}}
}
