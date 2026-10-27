package smarttakeloss

import "strings"

// logLines are the log rows of a tick's rows: a gate's reset, opening and shared rows merged into
// one at its first row (mergeFrom), every other row as it is.
func logLines(rows []Row) []Row {
	used := make([]bool, len(rows))
	lines := make([]Row, 0, len(rows))
	for index, row := range rows {
		if used[index] {
			continue
		}
		line, merged := mergeFrom(rows, used, index)
		if !merged {
			line = row
		}
		used[index] = true
		lines = append(lines, line)
	}
	return lines
}

// mergeFrom is the one line of a gate's rows starting at rows[start]: its reset and the triggered
// row after it, and the gate's held or activation row after the opening; false when nothing merges.
func mergeFrom(rows []Row, used []bool, start int) (Row, bool) {
	gate := rows[start].Gate
	reset, open := -1, start
	if rows[start].Event == EventReset && gate != GateSlowDecline {
		reset, open = start, nextOf(rows, used, start, gate, EventTriggered)
		if open < 0 {
			return Row{}, false
		}
	}
	if !opens(rows[open]) {
		return Row{}, false
	}
	shared := nextOf(rows, used, open, gate, EventSaleHeld, EventActivated)
	if reset < 0 && shared < 0 {
		return Row{}, false
	}
	var tail *Row
	if shared >= 0 {
		tail = &rows[shared]
	}
	message, ok := mergedMessage(rows[open], reset >= 0, tail)
	if !ok {
		return Row{}, false
	}
	for _, at := range []int{reset, open, shared} {
		if at >= 0 {
			used[at] = true
		}
	}
	line := rows[open]
	line.Message = message
	return line, true
}

// opens is whether a row opens its gate's episode: an episode gate's triggered row, the slow decline gate's pending row.
func opens(row Row) bool {
	if row.Gate == GateSlowDecline {
		return row.Event == EventPending
	}
	return row.Event == EventTriggered
}

// nextOf is the first unused row after from filed under gate with one of events, -1 when none.
func nextOf(rows []Row, used []bool, from int, gate string, events ...string) int {
	for index := from + 1; index < len(rows); index++ {
		if used[index] || rows[index].Gate != gate {
			continue
		}
		for _, event := range events {
			if rows[index].Event == event {
				return index
			}
		}
	}
	return -1
}

// mergedMessage is an opening row's message re-armed after a reset, pending on a held sale or
// activated with its profit (shared); false when a message is not the one its builder writes.
func mergedMessage(open Row, rearmed bool, shared *Row) (string, bool) {
	verb, suffix := "", ""
	if shared != nil {
		head, label := saleHeldHead(open.Gate), "sale held: "
		verb = "pending"
		if shared.Event == EventActivated {
			head, label, verb = activatedHead(open.Gate), "", "stopLoss activated"
		}
		why, ok := strings.CutPrefix(shared.Message, head)
		if !ok {
			return "", false
		}
		suffix = ", " + label + why
	}
	if open.Gate == GateSlowDecline {
		return pendingMessage(open, verb, suffix)
	}
	base := "smartTakeLoss: " + saleHeldLabel(open.Gate)
	rest, ok := strings.CutPrefix(open.Message, base+" triggered")
	if !ok {
		return "", false
	}
	var verbs []string
	if rearmed {
		verbs = append(verbs, "re-armed at the new depth")
	}
	if verb != "" {
		verbs = append(verbs, verb)
	}
	return base + " " + strings.Join(verbs, ", ") + rest + suffix, true
}
