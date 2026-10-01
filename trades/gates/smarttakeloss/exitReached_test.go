package smarttakeloss

import (
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// ExitReached agrees with Apply's sale on every grid ladder, state, block and
// price, under the three exits' switches in every position: where it holds, Apply sells
// from every proposal that is not a close and from none that is; where it
// does not, Apply sells from none.
func TestExitReachedAgreesWithApply(t *testing.T) {
	positions := []string{"", "buy", "stopLoss", "update_stopLoss", "update_buy", "forceTrailingStopLoss", "takeProfit", "sellLoss", "forceTrailingTakeProfit"}
	blocks, prices := gridBlocks(), gridPrices()
	reached := map[bool]int{}
	for _, switches := range [][3]bool{{true, true, true}, {true, false, true}, {false, true, true}, {false, false, true}, {true, true, false}, {true, false, false}, {false, true, false}, {false, false, false}} {
		withQuietSlowDeclineExit(t, switches[0])
		withCapitalProtectionExit(t, switches[1])
		withSlowPatternDeclineExit(t, switches[2])
		for _, trade := range gridTrades() {
			for _, state := range []string{"buy", "stopLoss", "takeProfit", "sellLoss"} {
				trade.PositionType = state
				for _, block := range blocks {
					for _, price := range prices {
						exit := ExitReached(trade, price, block)
						reached[exit]++
						for _, position := range positions {
							sold := Apply(trade, position, price, withBlock(block)).Reason != ""
							if sold != (exit && !gridCloses[position]) {
								t.Fatalf("switches %v, %d fills (state %q, events %d), %q at %v, block %+v: ExitReached %v, Apply sold %v",
									switches, len(trade.History), state, len(trade.StrategyEvents), position, price, block, exit, sold)
							}
						}
					}
				}
			}
		}
	}
	if reached[true] == 0 || reached[false] == 0 {
		t.Fatalf("fixture drifted: the grid must reach the exit and miss it, got %v", reached)
	}
}

// It reads this tick's judgement first, as Apply does: a pending ladder whose
// new fill the reading cancels is not pending at the band, one whose new fill
// the leg on and quiet confirms is, and a watched ladder the verdict marks
// pending on this tick sells at the band on the same tick. The trade it reads
// keeps its rows and its events.
func TestExitReachedReadsThisTicksJudgement(t *testing.T) {
	trade := pendingAtFive()
	rows, events := len(trade.Logs), len(trade.StrategyEvents)
	if ExitReached(trade, overJudgeBand, brokenReading().SmartTakeLoss) {
		t.Fatal("a fill the reading cancels leaves nothing pending at the band")
	}
	if !ExitReached(trade, judgeBand, legOnAndQuiet().SmartTakeLoss) {
		t.Fatal("a fill the leg on and quiet confirms keeps the ladder pending at the band")
	}
	if len(trade.Logs) != rows || len(trade.StrategyEvents) != events {
		t.Fatalf("ExitReached must write nothing, got %+v and %+v", trade.Logs, trade.StrategyEvents)
	}
	if !ExitReached(watchedTrade(), slowDeclineBand, slowDeclineBlock(true).SmartTakeLoss) {
		t.Fatal("the verdict marks the ladder pending and the band sells on the same tick")
	}
	if ExitReached(watchedTrade(), slowDeclineBand, slowDeclineBlock(false).SmartTakeLoss) {
		t.Fatal("without the verdict a watched ladder is not pending")
	}
	if ExitReached(watchedTrade(), slowDeclineBand, aggragates.SmartTakeLossIndicators{}) {
		t.Fatal("the zero block reaches nothing")
	}
}

// It reads the recent path as Apply does, under the switch on and off, on a
// watched ladder, a pending one and a pending one with a new fill, over
// readings whose recent bar reads for the ladder's newest fill or not: every
// add-side proposal sells where it holds and nowhere else. The decline read
// recently marks a watched ladder pending and the band sells on that tick,
// and it confirms a new fill the last closed bar would cancel.
func TestExitReachedReadsTheDeclineReadRecently(t *testing.T) {
	stood, from, late := testutil.At("16:00:00"), testutil.At("00:00:00"), testutil.At("23:30:00")
	var blocks []aggragates.SmartTakeLossIndicators
	for _, reading := range []aggragates.AIIndicators{brokenAtTheBand(), brokenReading(), legOnAndQuiet()} {
		for _, oldest := range []time.Time{from, late, {}} {
			blocks = append(blocks, withRecent(reading, stood, oldest).SmartTakeLoss)
		}
	}
	reached := map[bool]int{}
	for _, on := range []bool{true, false} {
		withQuietSlowDeclineExit(t, on)
		for _, trade := range []aggragates.Trades{watchedTrade(), pendingTrade(), pendingAtFive()} {
			for _, block := range blocks {
				for _, price := range []float64{underJudgeBand, judgeBand, underTheBand, slowDeclineBand} {
					exit := ExitReached(trade, price, block)
					reached[exit]++
					for _, position := range []string{"", "buy", "stopLoss"} {
						if sold := Apply(trade, position, price, withBlock(block)).Reason != ""; sold != exit {
							t.Fatalf("switch %v, %d fills, %q at %v, block %+v: ExitReached %v, Apply sold %v", on, len(trade.History), position, price, block, exit, sold)
						}
					}
				}
			}
		}
	}
	if reached[true] == 0 || reached[false] == 0 {
		t.Fatalf("fixture drifted: the readings must reach the exit and miss it, got %v", reached)
	}
	withQuietSlowDeclineExit(t, true)
	if !ExitReached(watchedTrade(), slowDeclineBand, withRecent(brokenAtTheBand(), stood, from).SmartTakeLoss) || ExitReached(watchedTrade(), slowDeclineBand, brokenAtTheBand().SmartTakeLoss) {
		t.Fatal("the decline read recently marks the watched ladder pending and the band sells on that tick; without it nothing sells")
	}
	if !ExitReached(pendingAtFive(), judgeBand, withRecent(brokenReading(), stood, from).SmartTakeLoss) || ExitReached(pendingAtFive(), judgeBand, brokenReading().SmartTakeLoss) {
		t.Fatal("a new fill the recent bar confirms keeps the ladder pending at the band; without it the fill cancels the exit")
	}
}

// On sampled ladders (sampledLadder, sampledState, sampledBlock), rows, states,
// readings and prices under every combination of the three switches,
// ExitReached is Apply's forced sale on the empty proposal and writes
// nothing. Every add-side proposal gets the same answer, and no close is
// replaced. A capital protection sale needs the rule on the ladder's own
// count, and that rule sells whenever the trade is not resting in a close. A
// sell-band sale needs a watched ladder at a band above zero. An indecision
// row comes back exactly for a long spot parent from IndecisionArmDepth fills
// that carries none yet, on a reading serving the indecision, whatever the
// state, and it sells nothing. Every row Apply hands back is filed under a
// gate and kind and carries the price of a fill. The sample reaches a marker
// naming the recent bar's reasons.
func TestExitReachedIsApplysSaleOnTheEmptyProposal(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	states := []string{"buy", "stopLoss", "forceTrailingStopLoss", "", "takeProfit", "forceTrailingTakeProfit", "update_takeProfit", "sellLoss", "sell", "sellParent", "impasse"}
	prices := []float64{-1, 0, math.Nextafter(capitalProtectionBand, 0), capitalProtectionBand, (capitalProtectionBand + slowDeclineBand) / 2, slowDeclineBand, slowDeclineBand + 1}
	seen := map[string]int{}
	for _, switches := range [][3]bool{{true, true, true}, {true, false, true}, {false, true, true}, {false, false, true}, {true, true, false}, {true, false, false}, {false, true, false}, {false, false, false}} {
		withQuietSlowDeclineExit(t, switches[0])
		withCapitalProtectionExit(t, switches[1])
		withIndecisionDirection(t, switches[2])
		for range 5000 {
			trade := sampledLadder(r)
			trade = sampledState(r, trade)
			trade.PositionType = states[r.IntN(len(states))]
			block, price := sampledBlock(r), prices[r.IntN(len(prices))]
			logs, stored := slices.Clone(trade.Logs), slices.Clone(trade.StrategyEvents)
			exit := ExitReached(trade, price, block)
			empty := Apply(trade, "", price, withBlock(block))
			closed := gridCloses[trade.PositionType]
			where := fmt.Sprintf("%d fills, inverse %v, %s, impasse %v, parent %d, state %q, %d rows, %d events, at %v",
				len(trade.History), trade.Inverse, trade.Strategy.TradeType, trade.Strategy.Params.Impasse, trade.ParentID, trade.PositionType, len(trade.Logs), len(trade.StrategyEvents), price)
			sold := ""
			if exit {
				sold = "sellLoss"
			}
			if !reflect.DeepEqual(trade.Logs, logs) || !reflect.DeepEqual(trade.StrategyEvents, stored) || exit != (empty.Reason != "") || empty.Position != sold {
				t.Fatalf("switches %v, %s: ExitReached %v, Apply on the empty proposal %+v", switches, where, exit, empty)
			}
			sellBandHolds := trade.Strategy.Params.SmartTakeLoss && trade.ParentID == 0 && slowDeclineWatched(trade) &&
				block.SlowDeclineSellBand > 0 && price >= block.SlowDeclineSellBand
			if (exit && closed) || (!closed && capitalProtectionHolds(trade, price, block) && !exit) ||
				(empty.Reason == reasonCapitalProtection && !capitalProtectionHolds(trade, price, block)) ||
				(empty.Reason == reasonSellBand && !sellBandHolds) {
				t.Fatalf("switches %v, %s: sold %q, block %+v", switches, where, empty.Reason, block)
			}
			for _, position := range []string{"buy", "stopLoss", "update_stopLoss", "update_buy", "forceTrailingStopLoss"} {
				want := empty
				if !exit {
					want.Position = position
				}
				if got := Apply(trade, position, price, withBlock(block)); !reflect.DeepEqual(got, want) {
					t.Fatalf("switches %v, %s, %q: got %+v, want %+v", switches, where, position, got, want)
				}
			}
			for position := range gridCloses {
				if got := Apply(trade, position, price, withBlock(block)); got.Position != position || got.Reason != "" {
					t.Fatalf("switches %v, %s: the close %q was replaced: %+v", switches, where, position, got)
				}
			}
			latched := gridHas(trade, GateIndecision, EventLatched)
			rowHolds := switches[2] && trade.Strategy.Params.SmartTakeLoss && trade.ParentID == 0 && price > 0 &&
				len(trade.StrategyPair.StrategySettings) > 0 && !trade.Inverse && trade.Strategy.TradeType != aggragates.Futures &&
				ladder.CountFilledEntries(trade) >= IndecisionArmDepth && !latched && block.SlowDeclineIndecision
			if (empty.Indecision != nil) != rowHolds {
				t.Fatalf("switches %v, %s: indecision row %+v, want one %v", switches, where, empty.Indecision, rowHolds)
			}
			if rowHolds {
				assertRow(t, empty.Indecision, IndecisionMessage(trade.PositionType, block.SlowDeclineBreakReasons), rebuildState(trade).lastFill().Price)
				seen["indecision row"]++
			}
			for _, row := range []*Row{empty.SlowDecline, empty.Indecision} {
				if row != nil && (row.Price <= 0 || row.Gate == "" || row.Event == "") {
					t.Fatalf("switches %v, %s: a row Apply hands back names its filing and a fill's price, got %+v", switches, where, *row)
				}
			}
			outcome := "no row"
			if empty.SlowDecline != nil {
				outcome = "row"
			}
			if empty.SlowDecline != nil && empty.SlowDecline.Message == SlowDeclineMessage(trade.PositionType, slowDeclineRecentReasons) {
				seen["recent row"]++
			}
			if exit {
				outcome += " sold"
			}
			seen[outcome]++
			seen[empty.Reason]++
			if closed && capitalProtectionHolds(trade, price, block) {
				seen["close kept"]++
			}
		}
	}
	for _, key := range []string{"", reasonSellBand, reasonCapitalProtection, "row", "row sold", "no row sold", "close kept", "recent row", "indecision row"} {
		if seen[key] == 0 {
			t.Fatalf("fixture drifted: the sample never reached %q, got %v", key, seen)
		}
	}
	t.Logf("answers: %v", seen)
}
