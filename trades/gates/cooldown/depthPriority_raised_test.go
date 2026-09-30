package cooldown

import (
	"fmt"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The amounts the opened rows of this file carry: their own, apart from the
// shipped constants, so a retune moves no expectation — a ladder trades the
// raise its own row names.
const (
	raisedPoints = 0.5
	raisedDepths = 2
)

// openedRaised is the ladder as the engine leaves it once it opened raised:
// the DynamicParams flag on for a long spot parent, and the one opened row in
// its logs. The rows the trade stores stay as they were.
func openedRaised(trade aggragates.Trades) aggragates.Trades {
	trade.Strategy.Params.DynamicParams = true
	trade.Strategy.TradeType = aggragates.Spot
	trade.Logs = append(append([]aggragates.TradesLogs(nil), trade.Logs...), aggragates.TradesLogs{
		Message: dynamicparams.OpenedMessage(raisedPoints, raisedDepths),
		Type:    aggragates.LOG_INFO,
	})

	return trade
}

// raisedAtTheStoredCeiling is a ladder of the fixture wallet that has filled
// every depth its stored rows allow and opened raised: the ladder whose own
// first entry was sized for more depths than that.
func raisedAtTheStoredCeiling(id uint, symbol string) aggragates.Trades {
	return openedRaised(testutil.LadderDepthTrade(id, symbol, walletDepths, walletDepths))
}

// The wallet is kept for a ladder until the depths its grid was sized for are
// filled, and a ladder that opened raised was sized for more than its stored
// rows say. Standing at the stored ceiling it is not full: the view carries
// the raised ceiling and what its extra depths cost, the sibling that would
// spend into that amount waits, and the row says the ladder keeps the wallet
// for its remaining depths and prints the raised ceiling.
//
// The identical ladder without the opened row is full and keeps nothing: only
// an entry the wallet cannot pay for holds, and the row says the ladder holds
// the wallet until it closes.
func TestDepthPriorityKeepsTheWalletForARaisedLadderAtItsStoredCeiling(t *testing.T) {
	requireDepthPriority(t)

	keeper := ladder.DepthOf(raisedAtTheStoredCeiling(14, "LINK/USDT"))
	if keeper.Depth != walletDepths || keeper.MaxDepth != walletDepths+raisedDepths {
		t.Fatalf("view = %+v, want depth %d of the raised ceiling %d", keeper, walletDepths, walletDepths+raisedDepths)
	}
	if keeper.RemainingCost <= 0 {
		t.Fatalf("view = %+v, want the raised ladder to name what its extra depths cost", keeper)
	}

	sibling := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	view := []aggragates.LadderDepth{keeper}
	short := walletShortFor(t, sibling, keeper.RemainingCost)

	reason := DepthPriorityHoldReason(priorityEvent(sibling, "buy", view, short), "stopLoss")
	if want := depthPriorityHoldMessage(keeper, ladder.DepthOf(sibling)); reason != want {
		t.Fatalf("reason = %q, want %q", reason, want)
	}
	for _, fragment := range []string{"keeps the wallet for its remaining depths", ruleKeeps(keeper)} {
		if !strings.Contains(reason, fragment) {
			t.Errorf("reason = %q, want it to carry %q", reason, fragment)
		}
	}
	if strings.Contains(reason, "until it closes") {
		t.Errorf("reason = %q, a ladder with raised depths left is not holding the wallet until it closes", reason)
	}

	level := short + 1
	if reason := DepthPriorityHoldReason(priorityEvent(sibling, "buy", view, level), "stopLoss"); reason != "" {
		t.Errorf("a wallet level with the reserve and the entry must let the sibling through, got %q", reason)
	}

	// The same ladder without its opened row: full, reserving nothing.
	full := ladder.DepthOf(testutil.LadderDepthTrade(14, "LINK/USDT", walletDepths, walletDepths))
	if full.Depth != full.MaxDepth || full.RemainingCost != 0 {
		t.Fatalf("view = %+v, want the ladder without its opened row full and reserving nothing", full)
	}

	fullView := []aggragates.LadderDepth{full}
	if reason := DepthPriorityHoldReason(priorityEvent(sibling, "buy", fullView, short), "stopLoss"); reason != "" {
		t.Errorf("a full ladder keeps nothing the wallet can pay for, got %q", reason)
	}

	ownCost := nextEntryCostOf(t, sibling, short)
	unpayable := DepthPriorityHoldReason(priorityEvent(sibling, "buy", fullView, ownCost-1), "stopLoss")
	if want := depthPriorityHoldMessage(full, ladder.DepthOf(sibling)); unpayable != want {
		t.Errorf("reason = %q, want only the entry the wallet cannot pay for held: %q", unpayable, want)
	}
	if !strings.Contains(unpayable, "holds the wallet until it closes") {
		t.Errorf("reason = %q, want the full ladder to hold the wallet until it closes", unpayable)
	}
}

// The ladder being asked about reads its own depths off its own opened row: a
// raised ladder at its stored ceiling has depths left, so it is ranked as one
// that does, not as a full ladder that is cheaper to finish than a sibling.
//
// Two raised ladders standing level — the same depth, the same planned cost —
// are split by their trade ids: the lower one is in front and the wallet is
// kept for it, the higher one waits. Read on the stored rows the ladder being
// asked about would be full with nothing left to finish, would outrank the
// lower id on that cost, and would be held by nobody.
func TestDepthPriorityRanksARaisedOwnLadderOnItsRaisedDepths(t *testing.T) {
	requireDepthPriority(t)

	ahead := raisedAtTheStoredCeiling(13, "SOL/USDT")
	own := raisedAtTheStoredCeiling(14, "LINK/USDT")
	aheadView := ladder.DepthOf(ahead)
	ownView := ladder.DepthOf(own)
	view := []aggragates.LadderDepth{aheadView, ownView}

	if ownView.Depth != aheadView.Depth || ownView.PlannedRemainingCost != aheadView.PlannedRemainingCost {
		t.Fatalf("views = %+v and %+v, want the two ladders level on everything but the id", ownView, aheadView)
	}
	if ownView.PlannedRemainingCost <= 0 {
		t.Fatalf("view = %+v, want a raised ladder at its stored ceiling planned to cost something to finish", ownView)
	}

	priority, found := depthPriorityFor(ownView, view)
	if !found || priority.TradeID != ahead.ID {
		t.Fatalf("depthPriorityFor = %+v, %v, want the lower id in front of the ladder asked about", priority, found)
	}
	if priority.Depth >= priority.MaxDepth {
		t.Fatalf("priority = %+v, want a ladder with raised depths left: it keeps the wallet", priority)
	}

	short := walletShortFor(t, own, aheadView.RemainingCost)
	reason := DepthPriorityHoldReason(priorityEvent(own, "buy", view, short), "stopLoss")
	if want := depthPriorityHoldMessage(aheadView, ownView); reason != want {
		t.Fatalf("reason = %q, want the higher id to wait for the lower one: %q", reason, want)
	}

	// The ladder in front is held by nobody: the one behind it is not ahead.
	if front, found := depthPriorityFor(aheadView, view); found {
		t.Errorf("depthPriorityFor = %+v, want nothing ahead of the lower id", front)
	}
	if reason := DepthPriorityHoldReason(priorityEvent(ahead, "buy", view, walletShortFor(t, ahead, aheadView.RemainingCost)), "stopLoss"); reason != "" {
		t.Errorf("the ladder in front must not be held by the one behind it, got %q", reason)
	}

	// The same pair without the opened rows is level and full: the wallet has
	// nothing to be kept for, only the entries it cannot pay for wait.
	fullAhead := ladder.DepthOf(testutil.LadderDepthTrade(13, "SOL/USDT", walletDepths, walletDepths))
	fullOwn := testutil.LadderDepthTrade(14, "LINK/USDT", walletDepths, walletDepths)
	fullView := []aggragates.LadderDepth{fullAhead, ladder.DepthOf(fullOwn)}

	if priority, found := depthPriorityFor(ladder.DepthOf(fullOwn), fullView); !found || priority.Depth < priority.MaxDepth {
		t.Fatalf("depthPriorityFor = %+v, %v, want the full ladder in front, keeping nothing", priority, found)
	}
	if reason := DepthPriorityHoldReason(priorityEvent(fullOwn, "buy", fullView, nextEntryCostOf(t, fullOwn, 0)), "stopLoss"); reason != "" {
		t.Errorf("a full ladder ahead keeps nothing the wallet can pay for, got %q", reason)
	}
}

// The row the operator reads names the raised ceilings, both ends of it, and
// they are the ceilings the ladders trade, not whatever the row builder
// derives: the ladder keeping the wallet at the depth its stored rows stop at
// is named with the raised ceiling, and so is the raised ladder that waits
// behind a full one. The expected text is written out from the fixture's own
// depths, apart from the builder the gate writes it with.
func TestDepthPriorityRowNamesTheRaisedCeilingsOfBothLadders(t *testing.T) {
	requireDepthPriority(t)

	const waiting = 4

	keeper := ladder.DepthOf(raisedAtTheStoredCeiling(14, "LINK/USDT"))
	sibling := testutil.LadderDepthTrade(12, "ETH/USDT", waiting, walletDepths)

	reason := DepthPriorityHoldReason(priorityEvent(sibling, "buy", []aggragates.LadderDepth{keeper}, walletShortFor(t, sibling, keeper.RemainingCost)), "stopLoss")
	want := fmt.Sprintf(
		DepthPriorityHoldMarker+", %s at depth %d of %d keeps the wallet for its remaining depths, this ladder waits at depth %d of %d",
		"LINK/USDT", walletDepths, walletDepths+raisedDepths, waiting, walletDepths,
	)
	if reason != want {
		t.Errorf("reason = %q, want %q", reason, want)
	}

	// The raised ladder waiting behind a FULL one at the same depth: the full
	// ladder is planned cheaper to finish, so it is in front and keeps
	// nothing, and the raised ladder, which has depths left, waits only for
	// an entry the wallet cannot pay for.
	own := raisedAtTheStoredCeiling(12, "LINK/USDT")
	full := ladder.DepthOf(testutil.LadderDepthTrade(13, "SOL/USDT", walletDepths, walletDepths))
	view := []aggragates.LadderDepth{full, ladder.DepthOf(own)}
	ownCost := nextEntryCostOf(t, own, 0)

	reason = DepthPriorityHoldReason(priorityEvent(own, "buy", view, ownCost-1), "stopLoss")
	want = fmt.Sprintf(
		DepthPriorityHoldMarker+", %s at depth %d of %d holds the wallet until it closes, this ladder waits at depth %d of %d",
		"SOL/USDT", walletDepths, walletDepths, walletDepths, walletDepths+raisedDepths,
	)
	if reason != want {
		t.Errorf("reason = %q, want %q", reason, want)
	}
}

// The ladder being asked about is not judged as full at its stored ceiling
// when it opened raised. A shallower sibling is never in front of it, so it is
// never held by one however short the wallet runs; and against a full ladder
// at the same depth it ranks on what it still has to fill — behind that
// ladder, which is planned cheaper to finish — where read on its stored rows
// it would rank on a plan of nothing, tie with it and win on the lower id.
func TestDepthPriorityDoesNotJudgeARaisedManagedLadderAsFull(t *testing.T) {
	requireDepthPriority(t)

	own := raisedAtTheStoredCeiling(12, "LINK/USDT")
	ownView := ladder.DepthOf(own)
	if ownView.Depth >= ownView.MaxDepth {
		t.Fatalf("view = %+v, want a raised ladder at its stored ceiling to have depths left", ownView)
	}

	// Behind it: a shallower sibling, which outranks nothing on the depth.
	shallower := ladder.DepthOf(testutil.LadderDepthTrade(11, "ETH/USDT", walletDepths/2, walletDepths))
	view := []aggragates.LadderDepth{shallower, ownView}

	if front, found := depthPriorityFor(ownView, view); found {
		t.Errorf("depthPriorityFor = %+v, want nothing in front of the deeper ladder", front)
	}
	for _, free := range []float64{0, walletShortFor(t, own, shallower.RemainingCost)} {
		if reason := DepthPriorityHoldReason(priorityEvent(own, "buy", view, free), "stopLoss"); reason != "" {
			t.Errorf("a shallower sibling must never hold the deeper ladder, got %q on a wallet of %f", reason, free)
		}
	}

	// In front of it at the same depth: a ladder that is full and, so, planned
	// to cost nothing more to finish.
	level := ladder.DepthOf(testutil.LadderDepthTrade(13, "SOL/USDT", walletDepths, walletDepths))
	if level.Depth != ownView.Depth || level.PlannedRemainingCost >= ownView.PlannedRemainingCost {
		t.Fatalf("views = %+v and %+v, want a level full ladder planned cheaper to finish", level, ownView)
	}

	front, found := depthPriorityFor(ownView, []aggragates.LadderDepth{level, ownView})
	if !found || front.TradeID != level.TradeID {
		t.Errorf("depthPriorityFor = %+v, %v, want the full ladder planned cheaper to finish in front of the raised one", front, found)
	}

	// The gate asks the same question of the trade itself: the full ladder in
	// front keeps nothing, so the raised ladder is held only on an entry the
	// wallet cannot pay for, and let through on one it can.
	ownCost := nextEntryCostOf(t, own, 0)
	withLevel := []aggragates.LadderDepth{level, ownView}

	if reason := DepthPriorityHoldReason(priorityEvent(own, "buy", withLevel, ownCost-1), "stopLoss"); !strings.Contains(reason, "holds the wallet until it closes") {
		t.Errorf("reason = %q, want the raised ladder held behind the full one on an entry the wallet cannot pay for", reason)
	}
	if reason := DepthPriorityHoldReason(priorityEvent(own, "buy", withLevel, ownCost), "stopLoss"); reason != "" {
		t.Errorf("reason = %q, want the raised ladder let through on a wallet that pays for its entry", reason)
	}
}

// What the raise changes is how deep the ladder may go and what its depths
// still cost, never what an add costs: a ladder that opened raised places its
// adds on the multiplier of the row each one reads, exactly as the same ladder
// without the raise does, so the entry the gate weighs is priced alike.
func TestDepthPriorityPricesAnAddOfARaisedLadderLikeAnyOther(t *testing.T) {
	requireDepthPriority(t)

	plain := testutil.LadderDepthTrade(12, "ETH/USDT", walletDepths, walletDepths)
	raised := openedRaised(plain)

	if got, want := nextEntryCostOf(t, raised, 0), nextEntryCostOf(t, plain, 0); got != want {
		t.Errorf("an add of the raised ladder costs %f, want the %f the same ladder costs without the raise", got, want)
	}
}
