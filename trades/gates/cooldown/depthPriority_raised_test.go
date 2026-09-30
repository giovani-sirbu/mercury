package cooldown

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The amounts the opened events of this file carry: their own, apart from the
// shipped constants, so a retune moves no expectation.
const (
	raisedPoints = 0.5
	raisedDepths = 2
)

// openedRaised is the ladder as the engine leaves it once it opened raised: the
// flag on for a long spot parent and the opened pair the engine's own writer
// appends, the event the raise is read from beside the row an operator reads.
func openedRaised(trade aggragates.Trades) aggragates.Trades {
	trade.Strategy.Params.DynamicParams = true
	trade.Strategy.TradeType = aggragates.Spot
	row, event := dynamicparams.Opened{Points: raisedPoints, Depths: raisedDepths}.Rows(trade, trade.PositionPrice, time.Time{})

	return aggragates.AppendStrategyRow(trade, row, event)
}

// raisedAtTheStoredCeiling has filled every depth its stored rows allow and
// opened raised: its own first entry was sized for more depths than that.
func raisedAtTheStoredCeiling(id uint, symbol string) aggragates.Trades {
	return openedRaised(testutil.LadderDepthTrade(id, symbol, walletDepths, walletDepths))
}

// raisedHold is the gate's answer for the trade on a buy tick that carries the
// wallet view and the balance.
func raisedHold(trade aggragates.Trades, view []aggragates.LadderDepth, free float64) gates.Hold {
	return DepthPriorityHold(priorityEvent(trade, "buy", view, free), "stopLoss")
}

// assertWaits fails unless the hold names the depth priority event of the wallet
// kept for priority while own waits, and its row text is formatted from it.
func assertWaits(t *testing.T, hold gates.Hold, priority, own aggragates.LadderDepth) {
	t.Helper()

	want := DepthPriorityEvent{
		Event:            gates.EventHeld,
		PrioritySymbol:   priority.Symbol,
		PriorityDepth:    priority.Depth,
		PriorityMaxDepth: priority.MaxDepth,
		Depth:            own.Depth,
		MaxDepth:         own.MaxDepth,
	}
	if hold.Param != aggragates.StrategyParamCooldown || hold.Gate != GateDepthPriority || hold.Data != want {
		t.Fatalf("hold names %q/%q with %+v, want the depth priority event %+v", hold.Param, hold.Gate, hold.Data, want)
	}
	if text := depthPriorityHoldMessage(want); hold.Reason != text {
		t.Fatalf("reason = %q, want %q", hold.Reason, text)
	}
}

// A ladder that opened raised was sized for more depths than its stored rows
// say: at the stored ceiling its view carries the raised ceiling and what the
// extra depths cost, and the sibling that would spend into that waits. The same
// ladder without the opened event is full and holds only what the wallet cannot pay for.
func TestDepthPriorityKeepsTheWalletForARaisedLadderAtItsStoredCeiling(t *testing.T) {
	requireDepthPriority(t)

	keeper := ladder.DepthOf(raisedAtTheStoredCeiling(14, "LINK/USDT"))
	if keeper.Depth != walletDepths || keeper.MaxDepth != walletDepths+raisedDepths || keeper.RemainingCost <= 0 {
		t.Fatalf("view = %+v, want depth %d of the raised ceiling %d and a reserve for the extra depths", keeper, walletDepths, walletDepths+raisedDepths)
	}
	sibling := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	view := []aggragates.LadderDepth{keeper}
	short := walletShortFor(t, sibling, keeper.RemainingCost)

	hold := raisedHold(sibling, view, short)
	assertWaits(t, hold, keeper, ladder.DepthOf(sibling))
	for _, fragment := range []string{"keeps the wallet for its remaining depths", ruleKeeps(keeper)} {
		if !strings.Contains(hold.Reason, fragment) {
			t.Errorf("reason = %q, want it to carry %q", hold.Reason, fragment)
		}
	}
	if strings.Contains(hold.Reason, "until it closes") {
		t.Errorf("reason = %q, a ladder with raised depths left is not holding the wallet until it closes", hold.Reason)
	}
	if hold := raisedHold(sibling, view, short+1); hold.Held() {
		t.Errorf("a wallet level with the reserve and the entry must let the sibling through, got %q", hold.Reason)
	}

	full := ladder.DepthOf(testutil.LadderDepthTrade(14, "LINK/USDT", walletDepths, walletDepths))
	if full.Depth != full.MaxDepth || full.RemainingCost != 0 {
		t.Fatalf("view = %+v, want the ladder without its opened event full and reserving nothing", full)
	}
	fullView := []aggragates.LadderDepth{full}
	if hold := raisedHold(sibling, fullView, short); hold.Held() {
		t.Errorf("a full ladder keeps nothing the wallet can pay for, got %q", hold.Reason)
	}
	unpayable := raisedHold(sibling, fullView, nextEntryCostOf(t, sibling, short)-1)
	assertWaits(t, unpayable, full, ladder.DepthOf(sibling))
	if !strings.Contains(unpayable.Reason, "holds the wallet until it closes") {
		t.Errorf("reason = %q, want the full ladder to hold the wallet until it closes", unpayable.Reason)
	}
}

// The ladder asked about reads its own depths off its own opened event, so a
// raised ladder at its stored ceiling is ranked as one with depths left. Two
// level ones split on trade id: the lower is in front and the higher waits,
// where read on the stored rows the higher would be full and wait for nobody.
func TestDepthPriorityRanksARaisedOwnLadderOnItsRaisedDepths(t *testing.T) {
	requireDepthPriority(t)

	ahead, own := raisedAtTheStoredCeiling(13, "SOL/USDT"), raisedAtTheStoredCeiling(14, "LINK/USDT")
	aheadView, ownView := ladder.DepthOf(ahead), ladder.DepthOf(own)
	view := []aggragates.LadderDepth{aheadView, ownView}

	if ownView.Depth != aheadView.Depth || ownView.PlannedRemainingCost != aheadView.PlannedRemainingCost || ownView.PlannedRemainingCost <= 0 {
		t.Fatalf("views = %+v and %+v, want two raised ladders level on everything but the id, planned to cost something", ownView, aheadView)
	}
	priority, found := depthPriorityFor(ownView, view)
	if !found || priority.TradeID != ahead.ID || priority.Depth >= priority.MaxDepth {
		t.Fatalf("depthPriorityFor = %+v, %v, want the lower id in front, with raised depths left to keep the wallet for", priority, found)
	}
	assertWaits(t, raisedHold(own, view, walletShortFor(t, own, aheadView.RemainingCost)), aheadView, ownView)

	if front, found := depthPriorityFor(aheadView, view); found {
		t.Errorf("depthPriorityFor = %+v, want nothing ahead of the lower id", front)
	}
	if hold := raisedHold(ahead, view, walletShortFor(t, ahead, aheadView.RemainingCost)); hold.Held() {
		t.Errorf("the ladder in front must not be held by the one behind it, got %q", hold.Reason)
	}

	fullAhead := ladder.DepthOf(testutil.LadderDepthTrade(13, "SOL/USDT", walletDepths, walletDepths))
	fullOwn := testutil.LadderDepthTrade(14, "LINK/USDT", walletDepths, walletDepths)
	fullView := []aggragates.LadderDepth{fullAhead, ladder.DepthOf(fullOwn)}
	if front, found := depthPriorityFor(ladder.DepthOf(fullOwn), fullView); !found || front.Depth < front.MaxDepth {
		t.Fatalf("depthPriorityFor = %+v, %v, want the full ladder in front, keeping nothing", front, found)
	}
	if hold := raisedHold(fullOwn, fullView, nextEntryCostOf(t, fullOwn, 0)); hold.Held() {
		t.Errorf("a full ladder ahead keeps nothing the wallet can pay for, got %q", hold.Reason)
	}
}

// The row names the raised ceiling of both ladders, written out from the
// fixture's own depths apart from the builder the gate writes it with: the keeper
// at the depth its stored rows stop at, and the raised ladder behind a full one.
func TestDepthPriorityRowNamesTheRaisedCeilingsOfBothLadders(t *testing.T) {
	requireDepthPriority(t)

	const waiting = 4
	keeper := ladder.DepthOf(raisedAtTheStoredCeiling(14, "LINK/USDT"))
	sibling := testutil.LadderDepthTrade(12, "ETH/USDT", waiting, walletDepths)

	hold := raisedHold(sibling, []aggragates.LadderDepth{keeper}, walletShortFor(t, sibling, keeper.RemainingCost))
	want := fmt.Sprintf(
		DepthPriorityHoldMarker+", %s at depth %d of %d keeps the wallet for its remaining depths, this ladder waits at depth %d of %d",
		"LINK/USDT", walletDepths, walletDepths+raisedDepths, waiting, walletDepths,
	)
	if hold.Reason != want {
		t.Errorf("reason = %q, want %q", hold.Reason, want)
	}

	// The full ladder is planned cheaper to finish, so it is in front and keeps
	// nothing: the raised ladder waits only for an entry the wallet cannot pay for.
	own := raisedAtTheStoredCeiling(12, "LINK/USDT")
	full := ladder.DepthOf(testutil.LadderDepthTrade(13, "SOL/USDT", walletDepths, walletDepths))
	hold = raisedHold(own, []aggragates.LadderDepth{full, ladder.DepthOf(own)}, nextEntryCostOf(t, own, 0)-1)
	want = fmt.Sprintf(
		DepthPriorityHoldMarker+", %s at depth %d of %d holds the wallet until it closes, this ladder waits at depth %d of %d",
		"SOL/USDT", walletDepths, walletDepths, walletDepths, walletDepths+raisedDepths,
	)
	if hold.Reason != want {
		t.Errorf("reason = %q, want %q", hold.Reason, want)
	}
}

// The ladder asked about is not judged as full at its stored ceiling when it
// opened raised: a shallower sibling is never in front of it however short the
// wallet runs, and against a full ladder at the same depth it ranks behind, on
// what it still has to fill, where on its stored rows it would tie and win.
func TestDepthPriorityDoesNotJudgeARaisedManagedLadderAsFull(t *testing.T) {
	requireDepthPriority(t)

	own := raisedAtTheStoredCeiling(12, "LINK/USDT")
	ownView := ladder.DepthOf(own)
	if ownView.Depth >= ownView.MaxDepth {
		t.Fatalf("view = %+v, want a raised ladder at its stored ceiling to have depths left", ownView)
	}
	shallower := ladder.DepthOf(testutil.LadderDepthTrade(11, "ETH/USDT", walletDepths/2, walletDepths))
	behind := []aggragates.LadderDepth{shallower, ownView}
	if front, found := depthPriorityFor(ownView, behind); found {
		t.Errorf("depthPriorityFor = %+v, want nothing in front of the deeper ladder", front)
	}
	for _, free := range []float64{0, walletShortFor(t, own, shallower.RemainingCost)} {
		if hold := raisedHold(own, behind, free); hold.Held() {
			t.Errorf("a shallower sibling must never hold the deeper ladder, got %q on a wallet of %f", hold.Reason, free)
		}
	}

	level := ladder.DepthOf(testutil.LadderDepthTrade(13, "SOL/USDT", walletDepths, walletDepths))
	if level.Depth != ownView.Depth || level.PlannedRemainingCost >= ownView.PlannedRemainingCost {
		t.Fatalf("views = %+v and %+v, want a level full ladder planned cheaper to finish", level, ownView)
	}
	withLevel := []aggragates.LadderDepth{level, ownView}
	if front, found := depthPriorityFor(ownView, withLevel); !found || front.TradeID != level.TradeID {
		t.Errorf("depthPriorityFor = %+v, %v, want the full ladder in front of the raised one", front, found)
	}
	ownCost := nextEntryCostOf(t, own, 0)
	if hold := raisedHold(own, withLevel, ownCost-1); !strings.Contains(hold.Reason, "holds the wallet until it closes") {
		t.Errorf("reason = %q, want the raised ladder held behind the full one on an entry the wallet cannot pay for", hold.Reason)
	}
	if hold := raisedHold(own, withLevel, ownCost); hold.Held() {
		t.Errorf("reason = %q, want the raised ladder let through on a wallet that pays for its entry", hold.Reason)
	}
}

// The raise changes how deep a ladder may go and what its depths still cost,
// never what an add costs: the entry the gate weighs is priced alike.
func TestDepthPriorityPricesAnAddOfARaisedLadderLikeAnyOther(t *testing.T) {
	requireDepthPriority(t)

	plain := testutil.LadderDepthTrade(12, "ETH/USDT", walletDepths, walletDepths)

	if got, want := nextEntryCostOf(t, openedRaised(plain), 0), nextEntryCostOf(t, plain, 0); got != want {
		t.Errorf("an add of the raised ladder costs %f, want the %f the same ladder costs without the raise", got, want)
	}
}

// The opened event is the raise and the row beside it only text: a ladder at its
// stored ceiling whose logs carry the opened row but whose events do not is full
// and keeps nothing, so the sibling the raised ladder holds goes through.
func TestDepthPriorityReadsTheRaisedLadderFromItsEventNeverItsRow(t *testing.T) {
	requireDepthPriority(t)

	raised := raisedAtTheStoredCeiling(14, "LINK/USDT")
	textOnly := raised
	textOnly.StrategyEvents = nil
	keeper, full := ladder.DepthOf(raised), ladder.DepthOf(textOnly)
	if len(textOnly.Logs) != 1 || full.MaxDepth != walletDepths || full.RemainingCost != 0 {
		t.Fatalf("view = %+v, want the opened row alone to leave the ladder full and reserving nothing", full)
	}

	sibling := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	short := walletShortFor(t, sibling, keeper.RemainingCost)
	if !raisedHold(sibling, []aggragates.LadderDepth{keeper}, short).Held() {
		t.Fatal("fixture drifted: the raised ladder must hold the sibling on this wallet")
	}
	if hold := raisedHold(sibling, []aggragates.LadderDepth{full}, short); hold.Held() {
		t.Errorf("a row without its event raises nothing: the full ladder keeps nothing, got %q", hold.Reason)
	}
}
