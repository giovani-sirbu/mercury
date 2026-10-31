package smartsell

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// sharedTrigger is the shared terms' trigger on one tick, read off the SMC trend dashboard
// (sharedTriggerOf): pct the Percentage of the strictest rule of SaleTriggers that holds, on
// false while none holds or the dashboard was not read (read), when the activation and a self-armed
// sale wait; trend, reads and inverse word the held row (heldByTrend).
type sharedTrigger struct {
	pct     float64
	on      bool
	read    bool
	inverse bool
	trend   series.Trend
	reads   []series.TrendRow
}

// sharedTriggerOf is the trigger the dashboard gives a ladder of side (aggragates.EntrySide): the
// strictest rule that holds on the trend, its rows and their free fall (ruleHolds, stricter; the
// last listed on a tie); off when none holds, the trend not served or the table empty.
func sharedTriggerOf(trend series.Trend, reads []series.TrendRow, side string) sharedTrigger {
	shared := sharedTrigger{read: trend.Served(), inverse: side == aggragates.SideShort, trend: trend, reads: reads}
	if !shared.read {
		return shared
	}
	best := -1
	for index, rule := range saleTriggers {
		if ruleHolds(rule, trend, reads, side) && (best < 0 || !stricter(saleTriggers[best], rule)) {
			best = index
		}
	}
	if best < 0 {
		return shared
	}
	shared.pct, shared.on = saleTriggers[best].Percentage, true
	return shared
}

// ruleHolds is whether the dashboard meets the rule for side: at least rule.MinTrendDirection rows reading that
// way (none asked at zero or below), for every read minimum above zero that row served and
// reading that way on at least that many reads, and for every MinFreeFall entry above zero that
// row served and in free fall that way.
func ruleHolds(rule SaleTrigger, trend series.Trend, reads []series.TrendRow, side string) bool {
	if rule.MinTrendDirection > 0 && trend.Rows(side) < rule.MinTrendDirection {
		return false
	}
	for index, least := range rule.MinReads {
		if least > 0 && (index >= len(reads) || reads[index].Way(side) < least) {
			return false
		}
	}
	for index, asked := range rule.MinFreeFall {
		if asked > 0 && (index >= len(reads) || !reads[index].Free(side)) {
			return false
		}
	}
	return true
}

// stricter is whether a asks more than b: more MinTrendDirection, then a higher sum of MinReads,
// then more rows asked in free fall.
func stricter(a, b SaleTrigger) bool {
	if a.MinTrendDirection != b.MinTrendDirection {
		return a.MinTrendDirection > b.MinTrendDirection
	}
	if sumA, sumB := sumReads(a.MinReads), sumReads(b.MinReads); sumA != sumB {
		return sumA > sumB
	}
	return freeFallRows(a.MinFreeFall) > freeFallRows(b.MinFreeFall)
}

// sumReads is the sum of a rule's read minimums above zero.
func sumReads(reads []int) int {
	sum := 0
	for _, least := range reads {
		sum += max(least, 0)
	}
	return sum
}

// freeFallRows is how many rows a rule asks in free fall: its MinFreeFall entries above zero.
func freeFallRows(rows []int) int {
	count := 0
	for _, asked := range rows {
		if asked > 0 {
			count++
		}
	}
	return count
}

// FullTrendTrigger is the trigger of a dashboard reading the ladder's way on every read, every row
// in free fall: the Percentage of the table's strictest rule (stricter, the last listed on a tie),
// which the exported row builders name; zero on an empty table.
func FullTrendTrigger() float64 {
	best := -1
	for index, rule := range saleTriggers {
		if best < 0 || !stricter(saleTriggers[best], rule) {
			best = index
		}
	}
	if best < 0 {
		return 0
	}
	return saleTriggers[best].Percentage
}

// heldByTrend is the reason text of a sale the dashboard holds: not read, or meeting no rule, the
// trend named the ladder's way (trendWords).
func (shared sharedTrigger) heldByTrend() string {
	if !shared.read {
		return "SMC trend not read"
	}
	return "SMC trend meets no trigger rule (" + trendWords(shared.trend, shared.reads, shared.inverse) + ")"
}

// heldByTrendOf is heldByTrend for the block as served, for a long or an inverse ladder.
func heldByTrendOf(block aggragates.SmartSellIndicators, inverse bool) string {
	side := aggragates.SideLong
	if inverse {
		side = aggragates.SideShort
	}
	return sharedTriggerOf(series.TrendOf(block), series.ReadsOf(block), side).heldByTrend()
}

// trendWords names the trend the ladder's way (up and free rise for an inverse ladder),
// "down on 2 of 3 rows: 1h 7/9, 4h 5/9 free fall, 1D 3/9", the rows left out when none was served.
func trendWords(trend series.Trend, reads []series.TrendRow, inverse bool) string {
	way, side, free := "down", aggragates.SideLong, " free fall"
	if inverse {
		way, side, free = "up", aggragates.SideShort, " free rise"
	}
	words := fmt.Sprintf("%s on %d of %d rows", way, trend.Rows(side), trend.Valid)
	rows := make([]string, 0, len(reads))
	for _, row := range reads {
		word := fmt.Sprintf("%s %d/%d", row.Timeframe, row.Way(side), row.Reads)
		if row.Free(side) {
			word += free
		}
		rows = append(rows, word)
	}
	if len(rows) > 0 {
		words += ": " + strings.Join(rows, ", ")
	}
	return words
}
