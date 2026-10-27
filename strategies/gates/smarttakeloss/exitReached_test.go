package smarttakeloss

import (
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// ExitReached is the tests' oracle for Apply's forced sale: a Reason set on the empty proposal at
// this price, with no clock.
func ExitReached(trade aggragates.Trades, price float64, block aggragates.SmartTakeLossIndicators) bool {
	return Apply(trade, "", price, aggragates.AIIndicators{SmartTakeLoss: block}).Reason != ""
}

// CapitalProtectionActs is the tests' oracle for capital protection's act on the empty proposal at
// now: its own sale, or where nothing is sold a row under its gate.
func CapitalProtectionActs(trade aggragates.Trades, price float64, block aggragates.SmartTakeLossIndicators, now time.Time) bool {
	result := ApplyAt(trade, "", price, aggragates.AIIndicators{SmartTakeLoss: block}, now)
	if result.Reason != "" {
		return result.Reason == reasonCapitalProtection
	}
	return slices.ContainsFunc(result.Rows, func(row Row) bool {
		return row.Gate == GateCapitalProtection
	})
}

// missedWindow is a reading sophos served on a window it read whose last closed bar reads for
// no ladder: what broke and the fill window, with no verdict and no leg on and quiet.
func missedWindow() aggragates.AIIndicators {
	return withBlock(withEveryRowTrend(aggragates.SmartTakeLossIndicators{
		QuietDeclineBreakReasons: quietDeclineBreakReasons,
		QuietDeclineFillFrom:     fillWindowFrom.UnixMilli(),
	}))
}

// ExitReached reads this tick's judgement as Apply does, being Apply's own answer on the
// empty proposal: a fill the reading cancels leaves nothing pending, a fill no reading
// judges yet waits, and a fill the leg on and quiet confirms, like a watched ladder the
// verdict marks pending on this very tick, leaves the trigger of the shared action standing
// at the print. What the action then does depends on the ladder's state alone. In stopLoss
// it sells, with the gate's row beside the sale, so ExitReached is true; in buy it
// activates the stopLoss, with the gate's row and the activation row, so ExitReached is
// false: the print is no exit. Where nothing stands neither state is acted on. The trade
// ExitReached reads keeps its rows and its events.
func TestExitReachedReadsThisTicksJudgementAndNeverAnActivation(t *testing.T) {
	withQuietDecline(t, true)
	pending := pendingAtFive()
	rows, events := len(pending.Logs), len(pending.StrategyEvents)
	armed := armingPrice(pending)
	activation := StopLossActivatedRow(sixthFill, -handLossPct(pending, armed), FeatureQuietDecline)

	for _, state := range []string{"buy", "stopLoss"} {
		resting := restingIn(pending, state)
		if ExitReached(resting, armed, missedWindow().SmartTakeLoss) {
			t.Fatalf("in %s: a fill the reading cancels leaves nothing pending to sell", state)
		}
		assertWaits(t, Apply(resting, "stopLoss", armed, missedWindow()), "stopLoss", sixthCancelled())
		assertUntouched(t, Apply(resting, "stopLoss", armed, nothingServed()), "stopLoss")
		if ExitReached(resting, armed, nothingServed().SmartTakeLoss) {
			t.Fatalf("in %s: a fill no reading judges yet waits", state)
		}
	}
	if !ExitReached(restingIn(pending, "stopLoss"), armed, legOnAndQuiet().SmartTakeLoss) {
		t.Fatal("a fill the leg on and quiet confirms leaves the trigger standing, and a ladder in stopLoss is sold on it")
	}
	assertSoldWith(t, Apply(restingIn(pending, "stopLoss"), "", armed, legOnAndQuiet()), reasonSlowDecline, sixthConfirmed())
	if ExitReached(pending, armed, legOnAndQuiet().SmartTakeLoss) {
		t.Fatal("the same fill confirmed, a ladder in buy has its stopLoss activated, and that is no exit")
	}
	assertActivated(t, Apply(pending, "", armed, legOnAndQuiet()), sixthConfirmed(), activation)

	watched := watchedTrade()
	armedWatched := armingPrice(watched)
	marked := StopLossActivatedRow(slowDeclineLastFill, -handLossPct(watched, armedWatched), FeatureQuietDecline)
	if !ExitReached(restingIn(watched, "stopLoss"), armedWatched, slowDeclineBlock(true).SmartTakeLoss) {
		t.Fatal("the verdict marks the ladder pending on this tick, and a ladder in stopLoss is sold on it")
	}
	assertSoldWith(t, Apply(restingIn(watched, "stopLoss"), "", armedWatched, slowDeclineBlock(true)), reasonSlowDecline, quietPending(slowDeclineLastFill))
	if ExitReached(watched, armedWatched, slowDeclineBlock(true).SmartTakeLoss) {
		t.Fatal("the verdict marks the ladder pending on this tick, and a ladder in buy has its stopLoss activated: no exit")
	}
	assertActivated(t, Apply(watched, "", armedWatched, slowDeclineBlock(true)), quietPending(slowDeclineLastFill), marked)
	for _, state := range []string{"buy", "stopLoss"} {
		resting := restingIn(watched, state)
		assertUntouched(t, Apply(resting, "", armedWatched, slowDeclineBlock(false)), "")
		assertUntouched(t, Apply(resting, "", armedWatched, withBlock(aggragates.SmartTakeLossIndicators{})), "")
		if ExitReached(resting, armedWatched, aggragates.SmartTakeLossIndicators{}) {
			t.Fatalf("in %s: the zero block reaches nothing", state)
		}
	}
	if len(pending.Logs) != rows || len(pending.StrategyEvents) != events {
		t.Fatalf("ExitReached must write nothing, got %+v and %+v", pending.Logs, pending.StrategyEvents)
	}
}

// The recent path as Apply reads it, under the switch on and off, on a watched ladder, a
// pending one and a pending one with a new fill, over readings whose recent bar reads for
// the ladder's newest fill or not, on every proposal the add side makes. The decline read
// recently marks a watched ladder pending on that tick and confirms a new fill the last
// closed bar would cancel, so the trigger stands exactly where the ladder is pending once
// this tick's rows are written and read back off the events. Where it stands the shared
// action acts on the ladder's state alone, whatever the proposal: a ladder in stopLoss is
// sold, and ExitReached says so; one in buy has its stopLoss activated at a loss and at a
// gain alike (the trigger takes any loss), and ExitReached is false; where nothing stands
// the proposal comes back untouched.
func TestTheDeclineReadRecentlyIsActedOnByTheStateAndExitReachedFollowsTheSale(t *testing.T) {
	stood, from, late := testutil.At("16:00:00"), testutil.At("00:00:00"), testutil.At("23:30:00")
	var blocks []aggragates.SmartTakeLossIndicators
	for _, reading := range []aggragates.AIIndicators{missedWindow(), legOnAndQuiet()} {
		for _, oldest := range []time.Time{from, late, {}} {
			blocks = append(blocks, withRecent(reading, stood, oldest).SmartTakeLoss)
		}
	}
	acted := map[string]int{}
	for _, on := range []bool{true, false} {
		withQuietDecline(t, on)
		for _, trade := range []aggragates.Trades{watchedTrade(), pendingTrade(), pendingAtFive()} {
			armed, average := armingPrice(trade), ladder.AverageEntryPrice(trade)
			for _, block := range blocks {
				for _, price := range []float64{armed, armed - 1, average * 1.02} {
					after, _ := engineTick(trade, "", price, time.Time{}, withBlock(block))
					stands := slowDeclineTriggered(after, rebuildState(after))
					for _, state := range []string{"buy", "stopLoss"} {
						resting := restingIn(trade, state)
						if exit := ExitReached(resting, price, block); exit != (state == "stopLoss" && stands) {
							t.Fatalf("switch %v, %d fills in %s at %v, block %+v: ExitReached %v, the trigger stands after this tick's rows: %v", on, len(trade.History), state, price, block, exit, stands)
						}
						for _, position := range addSideProposals {
							got := Apply(resting, position, price, withBlock(block))
							switch {
							case state == "stopLoss" && stands:
								if got.Position != "sellLoss" || got.Reason != reasonSlowDecline || got.Activated {
									t.Fatalf("switch %v, %d fills in stopLoss, %q at %v, block %+v: the trigger stands, the sale is made, got %+v", on, len(trade.History), position, price, block, got)
								}
								acted["sold"]++
							case state == "buy" && stands:
								if got.Position != "stopLoss" || !got.Activated || got.Reason != "" {
									t.Fatalf("switch %v, %d fills in buy, %q at %v, block %+v: the trigger stands, the stopLoss is activated, got %+v", on, len(trade.History), position, price, block, got)
								}
								acted[map[bool]string{true: "activated at a loss", false: "activated at a gain"}[price < average]]++
							default:
								if got.Position != position || got.Reason != "" || got.Activated {
									t.Fatalf("switch %v, %d fills in %s, %q at %v, block %+v: nothing to act on (trigger %v), got %+v", on, len(trade.History), state, position, price, block, stands, got)
								}
								acted["nothing stands"]++
							}
						}
					}
				}
			}
		}
	}
	for _, key := range []string{"sold", "activated at a loss", "activated at a gain", "nothing stands"} {
		if acted[key] == 0 {
			t.Fatalf("fixture drifted: the readings must make a ladder pending and leave it not, over a loss and a gain, never reached %q, got %v", key, acted)
		}
	}

	withQuietDecline(t, true)
	watched := restingIn(watchedTrade(), "stopLoss")
	armedWatched := armingPrice(watched)
	assertForced(t, Apply(watched, "stopLoss", armedWatched, withRecent(missedWindow(), stood, from)), reasonSlowDecline)
	assertNoSale(t, Apply(watched, "stopLoss", armedWatched, missedWindow()), "stopLoss")
	five := restingIn(pendingAtFive(), "stopLoss")
	armedFive := armingPrice(five)
	assertForced(t, Apply(five, "stopLoss", armedFive, withRecent(missedWindow(), stood, from)), reasonSlowDecline)
	assertNoSale(t, Apply(five, "stopLoss", armedFive, missedWindow()), "stopLoss")
}

// ExitReached follows the shared action's two bounds, because it is the action's own answer
// on the empty proposal, and it writes nothing while it does, the held row included. A
// pending ladder resting in stopLoss within both bounds is an exit, a loss exactly at the cap
// included, a bounce over break even too; over the cap it is not, and Apply writes the one
// held row. Past the depth bound it is not either, at any print. A ladder resting in buy is
// activated, never sold, so it is no exit at any print. Capital protection's trigger, judged on
// its own terms, still sells a ladder resting in stopLoss within its drawdown while the shared
// action's bound holds the slow decline's, and a held action with no trend reading is no exit.
// Armed asks what the gate watches, and the cap and the depth bound are no part of it.
func TestExitReachedFollowsTheCapAndTheDepthBoundOfTheSaleAndArmedDoesNotMoveWithThem(t *testing.T) {
	underTheShippedCap(t)
	buy := pendingTrade()
	trade := restingIn(buy, "stopLoss")
	rows, events := len(trade.Logs), len(trade.StrategyEvents)
	armed := armingPrice(trade)
	within, oneStepLower := firstPriceWithinTheCap(t, trade, armed)
	average := ladder.AverageEntryPrice(trade)
	nothing := withEveryRowTrend(aggragates.SmartTakeLossIndicators{})

	for _, price := range []float64{armed, oneStepLower} {
		if ExitReached(trade, price, nothing) {
			t.Errorf("ExitReached at %v: the loss is over the cap, the sale is held", price)
		}
		assertWaits(t, Apply(trade, "", price, nothingServed()), "", SaleHeldRow(slowDeclineLastFill, -handLossPct(trade, price), FeatureQuietDecline))
	}
	for _, price := range []float64{within, within + 1, average * 1.02} {
		if !ExitReached(trade, price, nothing) {
			t.Errorf("ExitReached at %v: within the cap, or over break even, the ladder in stopLoss is sold", price)
		}
		assertSold(t, Apply(trade, "", price, nothingServed()), reasonSlowDecline)
	}
	for _, price := range []float64{within, within + 1} {
		if ExitReached(buy, price, nothing) {
			t.Errorf("ExitReached at %v: a ladder in buy is activated, never sold", price)
		}
	}
	assertActivated(t, Apply(buy, "", within, nothingServed()), StopLossActivatedRow(slowDeclineLastFill, -handLossPct(buy, within), FeatureQuietDecline))

	atTheCap, lower := exactBoundary(t)
	exact := restingIn(exactLadder(), "stopLoss")
	assertSold(t, Apply(exact, "", atTheCap, nothingServed()), reasonSlowDecline)
	if !ExitReached(exact, atTheCap, nothing) || ExitReached(exact, lower, nothing) {
		t.Fatal("ExitReached must be true at the cap and false one price step lower: the loss exactly at the cap is within it")
	}
	if ExitReached(exactLadder(), atTheCap, nothing) {
		t.Fatal("the same ladder in buy is activated at the cap, never sold")
	}

	filled := ladder.CountFilledEntries(trade)
	pastTheBound(t, filled)
	for _, price := range []float64{armed, within, average * 1.02} {
		assertNoSale(t, Apply(trade, "", price, nothingServed()), "")
		if ExitReached(trade, price, nothing) {
			t.Errorf("ExitReached at %v: the ladder is past the depth bound, the action is held at any print", price)
		}
	}
	if len(trade.Logs) != rows || len(trade.StrategyEvents) != events {
		t.Fatal("ExitReached must write nothing, the held row included")
	}

	withCapitalProtectionExit(t, true)
	last := lastDepthLadder()
	capital := restingIn(withRows(last, quietPending(last.PositionPrice)), "stopLoss")
	withinDrawdown, overDrawdown := firstPriceWithinTheDrawdown(t, last)
	if !ExitReached(capital, withinDrawdown, solBlock()) {
		t.Fatal("capital protection's trigger sells within its drawdown while the shared action is held")
	}
	if ExitReached(capital, overDrawdown, solBlock()) || ExitReached(capital, withinDrawdown, aggragates.SmartTakeLossIndicators{}) {
		t.Fatal("a held action with capital protection over its drawdown or without its trend reading is no exit")
	}

	armedOn := Armed(trade)
	withAllowedLoss(t, anyLoss)
	withSellLossIgnoreLastDepths(t, 0)
	if Armed(trade) != armedOn || !armedOn {
		t.Fatal("Armed asks what the gate watches, and the cap and the depth bound are no part of it")
	}
}

// isShared is whether a row is one of the shared action's: the activation of the stopLoss or
// the held sale. Every other row of a tick is a gate's own.
func isShared(row Row) bool {
	return row.Event == EventActivated || row.Event == EventSaleHeld
}
