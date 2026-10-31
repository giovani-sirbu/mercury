package depthpriority

import (
	"os"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The suite reads the reserve through the ceiling; the offset tests pin their own.
func TestMain(m *testing.M) {
	depthOffset = 0
	os.Exit(m.Run())
}

// withDepthOffset runs the test at the given DepthOffset and puts the package's back after.
func withDepthOffset(t *testing.T, offset int) {
	t.Helper()

	was := depthOffset
	depthOffset = offset
	t.Cleanup(func() { depthOffset = was })
}

// offsetLadder is a fixture ladder on the wallet grid, as a trade.
func offsetLadder(id uint, symbol string, depth int) aggragates.Trades {
	return testutil.LadderDepthTrade(id, symbol, depth, walletDepths)
}

// LadderOf costs the reserve DepthOffset depths short of the ceiling: one depth less per step,
// the depth and the ceiling untouched, nothing once the offset reaches the depths left, and at 0
// the ladder as ladder.DepthOf reads it.
func TestDepthOffsetCostsTheReserveShortOfTheCeiling(t *testing.T) {
	keeper := offsetLadder(14, "LINK/USDT", 5)
	whole := ladder.DepthOf(keeper)

	previous := whole.RemainingCost + 1
	for offset := 0; offset <= walletDepths-5; offset++ {
		withDepthOffset(t, offset)
		view := LadderOf(keeper)

		if want := ladder.DepthOfOffset(keeper, offset); view != want {
			t.Errorf("offset %d: LadderOf = %+v, want ladder.DepthOfOffset %+v", offset, view, want)
		}
		if view.Depth != whole.Depth || view.MaxDepth != whole.MaxDepth {
			t.Errorf("offset %d: depth %d of %d, want %d of %d", offset, view.Depth, view.MaxDepth, whole.Depth, whole.MaxDepth)
		}
		if view.RemainingCost >= previous {
			t.Errorf("offset %d: RemainingCost = %f, want under the previous offset's %f", offset, view.RemainingCost, previous)
		}
		previous = view.RemainingCost
	}
	if previous != 0 {
		t.Errorf("at the depths left, RemainingCost = %f, want nothing to keep", previous)
	}

	withDepthOffset(t, 0)
	if got := LadderOf(keeper); got != whole {
		t.Errorf("offset 0: LadderOf = %+v, want ladder.DepthOf %+v", got, whole)
	}
}

// The hold moves with the reserve: at DepthOffset 1 a wallet the ceiling-costed reserve held is
// let through, and the shortened reserve holds exactly where the whole one did.
func TestDepthOffsetHoldsOnTheShortenedReserve(t *testing.T) {
	requireDepthPriority(t)

	own := offsetLadder(12, "ETH/USDT", 4)
	keeper := offsetLadder(14, "LINK/USDT", 5)
	whole := ladder.DepthOf(keeper).RemainingCost

	withDepthOffset(t, 1)
	view := []aggragates.LadderDepth{LadderOf(keeper), LadderOf(own)}
	short := viewReserve(t, view, walletAsset)
	if short >= whole {
		t.Fatalf("reserve = %f, want under the whole ladder's %f", short, whole)
	}

	if hold := DepthPriorityHold(priorityEvent(own, "buy", view, walletShortFor(t, own, whole)), "trailingBuy"); hold.Held() {
		t.Errorf("a wallet short of the whole reserve alone must buy, got %q", hold.Reason)
	}
	if hold := DepthPriorityHold(priorityEvent(own, "buy", view, walletShortFor(t, own, short)+1), "trailingBuy"); hold.Held() {
		t.Errorf("a wallet level with the shortened reserve must buy, got %q", hold.Reason)
	}

	hold := DepthPriorityHold(priorityEvent(own, "buy", view, walletShortFor(t, own, short)), "trailingBuy")
	assertWaits(t, hold, LadderOf(keeper), LadderOf(own))
	if !strings.Contains(hold.Reason, "keeps the wallet for its remaining depths") {
		t.Errorf("reason = %q, want the keeper with costed depths left", hold.Reason)
	}
}

// At DepthOffset 1 a ladder one depth short of its ceiling is full for the gate: it keeps its
// place in front but keeps nothing, a sibling waits behind it only for an entry the wallet cannot
// pay, and the reserve goes to the next ladder with costed depths left.
func TestDepthOffsetMakesALadderFullShortOfItsCeiling(t *testing.T) {
	requireDepthPriority(t)
	withDepthOffset(t, 1)

	own := offsetLadder(12, "ETH/USDT", 4)
	full := LadderOf(offsetLadder(14, "LINK/USDT", walletDepths-1))
	if full.RemainingCost != 0 || full.Depth >= full.MaxDepth {
		t.Fatalf("LINK at %d of %d keeps %f, want nothing kept one depth under its ceiling", full.Depth, full.MaxDepth, full.RemainingCost)
	}
	cost := nextEntryCostOf(t, own, 0)

	alone := []aggragates.LadderDepth{full, LadderOf(own)}
	if hold := DepthPriorityHold(priorityEvent(own, "buy", alone, cost), "trailingBuy"); hold.Held() {
		t.Errorf("a wallet that pays the entry must buy behind a full ladder, got %q", hold.Reason)
	}
	hold := DepthPriorityHold(priorityEvent(own, "buy", alone, cost-1), "trailingBuy")
	assertWaits(t, hold, full, LadderOf(own))
	if !strings.Contains(hold.Reason, "holds the wallet until it closes") {
		t.Errorf("reason = %q, want the full ladder holding until it closes", hold.Reason)
	}

	next := LadderOf(offsetLadder(13, "SOL/USDT", 5))
	view := []aggragates.LadderDepth{full, next, LadderOf(own)}
	hold = DepthPriorityHold(priorityEvent(own, "buy", view, walletShortFor(t, own, next.RemainingCost)), "trailingBuy")
	assertWaits(t, hold, next, LadderOf(own))
}
