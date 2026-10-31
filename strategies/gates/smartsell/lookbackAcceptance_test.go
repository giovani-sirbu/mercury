package smartsell

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/earlytakeprofit"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// Acceptance scenarios of trade 117291 (SOL/USDT, backtest 230, 2022-06-09), played hour by
// hour at Apply: the engine's prints, the block sophos caches for the hour — the tape closed to
// the bar before it — and the rows each print writes, appended as the engines do (engineTick).
// The tape is a staircase built from the swing decline's constants and every expectation comes
// from swingdecline.Read alone, so the scenarios hold under any calibration and skip, naming
// it, where no staircase reads as one needs. Each test sets the features it needs (lbGate).

// lbBelow and lbBand are two prints of the ladder's life, the second over the first.
// Both sit under the ladder's break even, where its sale books a real loss, and neither
// decides more than that while the shared action's trigger takes any loss.
const (
	lbBelow = 37.0
	lbBand  = 38.0
	lbTop   = 41.0 // the staircase's first high
	// lbRise and lbFall scale lbBand into a print a share of two over it and under it: over
	// the pair row's trailing take profit and under minus its tolerance, so that the ladder
	// the shared action activated at lbBand re-anchors at the one and is sold at the other.
	lbRise = 1.02
	lbFall = 0.98
)

var (
	lbDay     = time.Date(2022, time.June, 9, 0, 0, 0, 0, time.UTC)
	lbLegBars = max(2*swingdecline.PivotBars, swingdecline.MinBarsBetween) // a leg in bars: every turn is a swing once its bars have closed, and a down leg counts
	lbMinutes = []time.Duration{5 * time.Second, 17*time.Minute + 10*time.Second, 45 * time.Minute, 59 * time.Minute}
)

func lbBar(index int) time.Time { return lbDay.Add(time.Duration(index) * time.Hour) }

// lbTape is the staircase's closes, an hour apart from lbDay: a run in to the first high on bar
// lbLegBars, a first down leg of firstLeg bars, then down legs and bounces of lbLegBars bars, every
// fall one and a half swingdecline.MinDropPct and every bounce half swingdecline.MaxBouncePct, with
// the legs it takes to hold under every constant of the rule and two to spare.
func lbTape(firstLeg int) []float64 {
	legs := 1 + max(swingdecline.MinLegsDown, swingdecline.MinLegsUp, 1)
	fall := 1.5 * swingdecline.MinDropPct / 100
	bounce := swingdecline.MaxBouncePct / 2 / 100
	closes := []float64{lbTop * (1 - fall)}
	run := func(to float64, bars int) {
		from := closes[len(closes)-1]
		for step := 1; step <= bars; step++ {
			closes = append(closes, from+(to-from)*float64(step)/float64(bars))
		}
	}
	price := lbTop
	run(price, lbLegBars)
	for leg := 1; leg <= legs+2; leg++ {
		price *= 1 - fall
		run(price, map[bool]int{true: firstLeg, false: lbLegBars}[leg == 1])
		price *= 1 + bounce
		run(price, lbLegBars)
	}
	return closes
}

// lbSeries is the tape closed to the bar before `hour`, as sophos serves it, flat wicks on the
// closes; lbBlock the hour's block: that series, the fill window sophos read and a Trend Direction
// row that just meets the need.
func lbSeries(closes []float64, hour int) series.Series {
	tape := series.Series{Closes: closes[:min(hour, len(closes))]}
	for bar := range tape.Closes {
		tape.Opens = append(tape.Opens, lbBar(bar).UnixMilli())
	}
	tape.Highs, tape.Lows = slices.Clone(tape.Closes), slices.Clone(tape.Closes)
	return tape
}

func lbBlock(closes []float64, hour int) aggragates.SmartSellIndicators {
	tape := lbSeries(closes, hour)
	return aggragates.SmartSellIndicators{
		QuietDeclineFillFrom: lbBar(0).UnixMilli(), SeriesOpens: tape.Opens, SeriesCloses: tape.Closes, SeriesHighs: tape.Highs, SeriesLows: tape.Lows,
		SmcTrendDown: max(swingdecline.MinSmcTrendDown, 0), SmcTrendValid: 3,
	}
}

// lbFill is an entry fill: its bar, stamped inside it, and its price (117291's four, then lower).
type lbFill struct {
	bar   int
	price float64
}

func (fill lbFill) at() time.Time { return lbBar(fill.bar).Add(17*time.Minute + 9*time.Second) }

func lbPrice(k int) float64 {
	return []float64{40.56, 39.61, 38.68, 37.13}[min(k, 3)] * math.Pow(0.96, float64(max(k-3, 0)))
}

// lbPrint is one print of the ladder's life: when, the state its trade was in, the block it was
// given and what the gate answered.
type lbPrint struct {
	at     time.Time
	before state
	block  aggragates.SmartSellIndicators
	result Result
}

// lbPlan sets one print — it may edit the hour's block or hold the trade — and answers its price.
type lbPlan func(hour, minute int, trade *aggragates.Trades, block *aggragates.SmartSellIndicators) float64

func lbAt(price float64) lbPlan {
	return func(int, int, *aggragates.Trades, *aggragates.SmartSellIndicators) float64 { return price }
}

// lbAfter prints lbBand up to and including the print at `from` and price on every print
// after it: the print that activates the trailingBuy at lbBand, then the trail's ticks.
func lbAfter(from time.Time, price float64) lbPlan {
	return func(hour, minute int, _ *aggragates.Trades, _ *aggragates.SmartSellIndicators) float64 {
		if lbBar(hour).Add(lbMinutes[minute]).After(from) {
			return price
		}
		return lbBand
	}
}

// lbLife plays the ladder's life from hour `from` to `to`, four prints an hour, on the hour's block,
// every print on the empty proposal: a tick that proposes nothing.
func lbLife(closes []float64, fills []lbFill, from, to int, plan lbPlan) []lbPrint {
	return lbLifeProposing("", closes, fills, from, to, plan)
}

// lbLifeProposing is lbLife with every print carrying the proposal position, as the engines hand it
// to Apply. The trade only collects the rows each print writes (engineTick) and rests in buy
// throughout, so the shared action finds it in buy on every print.
func lbLifeProposing(position string, closes []float64, fills []lbFill, from, to int, plan lbPlan) []lbPrint {
	return lbPlay(engineTick, position, closes, fills, from, to, plan)
}

// lbLifeStepping is lbLifeProposing with the state the engines leave between two ticks
// (engineStep): after the print that activates the trailingBuy the ladder rests in trailingBuy.
func lbLifeStepping(position string, closes []float64, fills []lbFill, from, to int, plan lbPlan) []lbPrint {
	return lbPlay(engineStep, position, closes, fills, from, to, plan)
}

// lbTick is one engine tick on the trade (engineTick, engineStep): the trade it leaves and Apply's answer.
type lbTick func(trade aggragates.Trades, position string, price float64, now time.Time, ai aggragates.AIIndicators) (aggragates.Trades, Result)

// lbPlay plays the life with the proposal position on every print, each print ticked by tick.
func lbPlay(tick lbTick, position string, closes []float64, fills []lbFill, from, to int, plan lbPlan) []lbPrint {
	trade, next := testutil.LadderTrade(false), 0
	var prints []lbPrint
	for hour := from; hour <= to; hour++ {
		for minute, offset := range lbMinutes {
			at := lbBar(hour).Add(offset)
			for ; next < len(fills) && !fills[next].at().After(at); next++ {
				trade = withFill(trade, fills[next].price, fills[next].at())
			}
			block := lbBlock(closes, hour)
			price := plan(hour, minute, &trade, &block)
			before := rebuildState(trade)
			var got Result
			trade, got = tick(trade, position, price, at, withBlock(block))
			prints = append(prints, lbPrint{at, before, block, got})
		}
	}
	return prints
}

// lbHolds is the oracle: the depth of the first window — longest first — from an earlier
// fill at least swingdecline.MinDepthsBetween depths back to the series' last
// bar that holds on its shape, by swingdecline.Read alone; zero when none.
func lbHolds(tape series.Series, fillBars []int) int {
	end := tape.Opens[len(tape.Opens)-1]
	for from := 1; from <= len(fillBars)-swingdecline.MinDepthsBetween; from++ {
		if reading, readable := swingdecline.Read(tape, lbStart(tape, fillBars, from), end, from, len(fillBars)); readable && reading.Holds {
			return from
		}
	}
	return 0
}

// lbStart is where the window from the fill at depth from starts, as the gate cuts it: the fill's
// bar, led by swingdecline.LookbackBars for the first fill, the series' first bar at the least.
func lbStart(tape series.Series, fillBars []int, from int) int64 {
	start := lbBar(fillBars[from-1]).UnixMilli()
	if from > 1 || swingdecline.LookbackBars <= 0 {
		return start
	}
	led, _ := tape.BarAtOrAfter(start - int64(swingdecline.LookbackBars)*series.BarMs)
	return led
}

// lbGate runs the test with the swing and quiet declines as given, the early take profit
// on and a depth priority pausing the gate, whatever the live switches are.
func lbGate(t *testing.T, swing, quiet bool) {
	t.Helper()
	previousPause := pauseWhenActiveDepthPriority
	pauseWhenActiveDepthPriority = true
	t.Cleanup(func() { pauseWhenActiveDepthPriority = previousPause })
	withSwingDecline(t, swing)
	withQuietDecline(t, quiet)
	withEarlyTakeProfit(t, true)
}

// withEarlyTakeProfitArmDepth runs the test with the early take profit watching a long spot ladder from
// depth filled entries and puts the package's value back after; package state, so never in parallel.
func withEarlyTakeProfitArmDepth(t *testing.T, depth int) {
	t.Helper()
	previous := earlytakeprofit.ArmDepth
	earlytakeprofit.ArmDepth = depth
	t.Cleanup(func() { earlytakeprofit.ArmDepth = previous })
}

// A ladder whose swing shape misses at its newest fill's bar and holds on a later bar, with
// no new fill, goes pending on the first print of the hour after that bar closes: one pending
// row, naming the window up to that bar, none before it and none after it, in the hour or the
// hours that follow, whichever proposal the ladder makes. The shared action follows the row on the
// very same print: the ladder rests in buy and its sale books a real loss, so it has its trailingBuy
// activated, one activation row after the pending one, in place of whatever it proposed. Before that
// print a ladder not yet pending is activated and sold on none; from the next one on, resting
// in trailingBuy as the engines leave it, it trails from the activation print with no row,
// whichever proposal it makes: held at the same print, re-anchored by the first print over
// the trail (and held at that print again, the anchor having moved), sold on every print under
// the tolerance.
func TestLookbackAHoldOnALaterBarGoesPendingOnItsFirstTickOnly(t *testing.T) {
	lbGate(t, true, false)
	withLinearDecline(t, false)
	withAllowedLoss(t, anyLoss)
	withSaleIgnoreLastDepths(t, 0)
	for _, depth := range []int{swingdecline.ArmDepth, swingdecline.ArmDepth + 1} {
		t.Run(fmt.Sprintf("%d fills", depth), func(t *testing.T) {
			closes, bars, fills := lbTape(depth+2), make([]int, depth), make([]lbFill, depth)
			for k := range bars {
				bars[k], fills[k] = lbLegBars+k, lbFill{lbLegBars + k, lbPrice(k)}
			}
			newest, held := bars[depth-1], -1
			for bar := newest + 1; bar < len(closes) && held < 0; bar++ {
				if lbHolds(lbSeries(closes, bar+1), bars) > 0 {
					held = bar
				}
			}
			needsAScenario(t, held > newest && lbHolds(lbSeries(closes, newest+1), bars) == 0, "a decline that misses at the newest fill's bar and holds on a later one")
			from := lbHolds(lbSeries(closes, held+1), bars)
			start := time.UnixMilli(lbStart(lbSeries(closes, held+1), bars, from)).UTC()
			window := fmt.Sprintf("depth %d to %d, 1h bars %s to %s UTC", from, depth, start.Format("2006-01-02 15:04"), lbBar(held).Format("2006-01-02 15:04"))
			features, activatedOn := []string{FeatureEarlyTakeProfit, FeatureSwingDecline}, []string{FeatureSwingDecline}

			var rows []string
			for _, step := range lbLifeStepping("", closes, fills, bars[0], held+8, lbAt(lbBelow)) {
				for _, row := range step.result.Rows {
					rows = append(rows, step.at.Format("15:04:05")+" "+row.Event)
					switch row.Event {
					case EventPending:
						if !strings.Contains(row.Message, window) || row.Price != lbPrice(depth-1) || fmt.Sprint(row.Features) != fmt.Sprint(features) {
							t.Errorf("the row %+v must name %q at the newest fill's price on %v", row, window, features)
						}
					case EventActivated:
						if !strings.HasPrefix(row.Message, "smartSell: slow decline "+activatedWordsOf(row.State)+", profit ") || row.Price != lbPrice(depth-1) || fmt.Sprint(row.Features) != fmt.Sprint(activatedOn) {
							t.Errorf("the row %+v must be the activation of the slow decline's sale at the newest fill's price on %v", row, activatedOn)
						}
					default:
						t.Errorf("the row %+v is neither the pending row nor the activation", row)
					}
				}
			}
			rowAt := lbBar(held + 1).Add(5 * time.Second)
			if stamp := rowAt.Format("15:04:05"); !slices.Equal(rows, []string{stamp + " pending", stamp + " activated"}) {
				t.Fatalf("rows %q, want exactly the pending row and the activation, on the first print of the hour after bar %d closed: %s", rows, held, stamp)
			}

			for _, position := range addSideProposals {
				for index, step := range lbLifeStepping(position, closes, fills, bars[0], held+8, lbAt(lbBand)) {
					where := fmt.Sprintf("print %d at %s on %q", index, step.at.Format("15:04:05"), position)
					switch {
					case step.at.Before(rowAt):
						if step.result.Position != position || step.result.Reason != "" || step.result.Activated || len(step.result.Rows) != 0 {
							t.Fatalf("%s: a ladder not pending is neither activated nor sold, nor does it write a row, got %+v", where, step.result)
						}
					case step.at.Equal(rowAt):
						if rows := step.result.Rows; len(rows) != 2 || rows[0].Event != EventPending || rows[1].Event != EventActivated {
							t.Fatalf("%s: the print that goes pending writes the pending row and the activation, got %+v", where, step.result.Rows)
						}
						if step.result.Position != "trailingBuy" || !step.result.Activated || step.result.Reason != "" {
							t.Fatalf("%s: the ladder in buy has its trailingBuy activated on that print, in place of what it proposed, got %+v", where, step.result)
						}
					default:
						assertTrailHeld(t, step.result)
					}
				}
				first := true
				for index, step := range lbLifeStepping(position, closes, fills, bars[0], held+8, lbAfter(rowAt, lbBand*lbRise)) {
					where := fmt.Sprintf("print %d at %s on %q rising", index, step.at.Format("15:04:05"), position)
					switch {
					case !step.at.After(rowAt):
					case first:
						first = false
						assertReanchored(t, step.result)
					default:
						if step.result.Position != "" || step.result.Reason != "" || step.result.Activated || len(step.result.Rows) != 0 {
							t.Fatalf("%s: the anchor moved to the rise, so the same print is held, got %+v", where, step.result)
						}
					}
				}
				for _, step := range lbLifeStepping(position, closes, fills, bars[0], held+8, lbAfter(rowAt, lbBand*lbFall)) {
					if step.at.After(rowAt) {
						assertSold(t, step.result, reasonSlowDecline)
					}
				}
			}
		})
	}
}
