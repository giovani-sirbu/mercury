package actions

import (
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// walletDepths is the depths every ladder of the fixture wallet is
// configured for.
const walletDepths = 8

// walletAsset is what every ladder of the fixture wallet spends.
const walletAsset = "USDT"

// walletLadderOf maps a fixture ladder into the view an engine hands the
// tick, through the helper all four surfaces build their view with — so the
// depths and the costs the gate reads come off a real ladder.
func walletLadderOf(id uint, symbol string, depth int) aggragates.LadderDepth {
	return ladder.DepthOf(testutil.LadderDepthTrade(id, symbol, depth, walletDepths))
}

// priorityWallet is a wallet whose pairs fell together: several part-filled
// ladders, the deepest still short of the last depth it was sized for.
func priorityWallet() []aggragates.LadderDepth {
	return []aggragates.LadderDepth{
		walletLadderOf(11, "BTC/USDT", 5),
		walletLadderOf(12, "ETH/USDT", 4),
		walletLadderOf(14, "LINK/USDT", 7),
	}
}

// priorityReserve is what that wallet is kept for: the remaining cost of its
// deepest ladder, read back off the view.
func priorityReserve() float64 {
	return walletLadderOf(14, "LINK/USDT", 7).RemainingCost
}

// walletShortOf is a wallet one unit of quote under the line: it cannot carry
// both the reserve and this trade's next entry.
func walletShortOf(t *testing.T, trade aggragates.Trades, reserve float64) float64 {
	t.Helper()

	return walletCovering(t, trade, reserve) - 1
}

// walletCovering is the wallet that carries both, where nobody waits.
func walletCovering(t *testing.T, trade aggragates.Trades, reserve float64) float64 {
	t.Helper()

	_, ownCost := ladder.NextEntryCost(trade, reserve)
	if ownCost <= 0 {
		t.Fatalf("%s must cost something to place its next entry", trade.Symbol)
	}

	return reserve + ownCost
}

// priorityEvent runs a trade through ShouldHold on a tick carrying the wallet
// view and the balance its next entry would be placed from. The clock is left
// unknown, which is what keeps depth spacing out of the way of these
// assertions; the one test that wants both gates speaking stamps it.
func priorityEvent(trade aggragates.Trades, oldPosition string, ladders []aggragates.LadderDepth, free float64) events.Events {
	event := blindPriorityEvent(trade, oldPosition, ladders)
	event.Params.WalletFree = []aggragates.AssetFree{{Asset: walletAsset, Free: free}}

	return event
}

// blindPriorityEvent is the same tick with no balance at all: the engine
// could not name one, and the gate then holds nothing.
func blindPriorityEvent(trade aggragates.Trades, oldPosition string, ladders []aggragates.LadderDepth) events.Events {
	return events.Events{
		Trade: trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{
			OldPosition: oldPosition,
			// The engines anchor this on the trade's own position price, and
			// a hold restores the trade to it. Left at zero, a held ladder
			// would come back off SaveHoldLog with no price and its next
			// entry would price at nothing on the tick after.
			OldPositionPrice: trade.PositionPrice,
			WalletLadders:    ladders,
		},
	}
}

// An add that would break into the deepest ladder's remaining depths waits,
// and the row says which ladder the wallet is being kept for.
func TestDepthPriorityHoldsAnAddThroughShouldHold(t *testing.T) {
	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)

	held, err := ShouldHold(priorityEvent(trade, "buy", priorityWallet(), walletShortOf(t, trade, priorityReserve())))
	if err == nil {
		t.Fatal("an entry the wallet cannot spare must be held")
	}
	if len(held.Trade.Logs) != 1 {
		t.Fatalf("expected one row, got %v", messages(held.Trade.Logs))
	}

	row := held.Trade.Logs[0]
	want := "Hold stopLoss: cooldown: depth priority, LINK/USDT at depth 7 of 8 keeps the wallet for its remaining depths, this ladder waits at depth 4 of 8"
	if row.Message != want {
		t.Fatalf("row = %q, want %q", row.Message, want)
	}
	if row.Type != aggragates.LOG_INFO {
		t.Errorf("row type = %q, want %q", row.Type, aggragates.LOG_INFO)
	}
	if held.Trade.PositionType != "buy" {
		t.Errorf("position restored to %q, want the old position", held.Trade.PositionType)
	}
}

// The same add on a wallet that carries both goes straight through, with no
// row at all: the reserve is a floor under the deepest ladder, not a queue.
func TestDepthPriorityLetsAnAddThroughWhenTheWalletCoversBoth(t *testing.T) {
	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)

	free, err := ShouldHold(priorityEvent(trade, "buy", priorityWallet(), walletCovering(t, trade, priorityReserve())))
	if err != nil {
		t.Fatalf("a wallet that covers both must let the entry through, got %v", err)
	}
	if len(free.Trade.Logs) != 0 {
		t.Fatalf("no row may be written on a released ladder, got %v", messages(free.Trade.Logs))
	}
}

// The first fill is gated too, and by this gate first: the wallet decides
// whether the trade may spend anything at all, so a held entry must not
// consume the sophos verdict the first-fill gate would have activated its own
// hold on. The verdict here refuses the side, so the first-fill gate would
// have written its waiting row if it had been asked.
func TestDepthPriorityHoldsTheFirstFillBeforeTheFirstFillGate(t *testing.T) {
	trade := testutil.LadderDepthTrade(21, "ADA/USDT", 0, walletDepths)
	trade.PositionType = "buy"

	reserve := priorityReserve()
	event := priorityEvent(trade, "new", priorityWallet(), walletShortOf(t, trade, reserve))
	event.Params.CoolDownIndicators = aggragates.CoolDownIndicators{HasFirstFillVerdict: true}

	held, err := ShouldHold(event)
	if err == nil {
		t.Fatal("a first fill the wallet cannot spare must wait")
	}
	if len(held.Trade.Logs) != 1 {
		t.Fatalf("expected one row, got %v", messages(held.Trade.Logs))
	}

	row := held.Trade.Logs[0].Message
	if !strings.HasPrefix(row, "Hold entry: cooldown: depth priority,") {
		t.Fatalf("row = %q, want the entry hold to name the wallet gate", row)
	}
	if strings.Contains(row, cooldown.FirstFillWaitingPrefix) {
		t.Fatalf("row = %q, the first-fill gate must not have been consulted", row)
	}
}

// The close is what refills the wallet every waiting ladder is measured
// against, so it is never gated.
func TestDepthPriorityLeavesACloseAlone(t *testing.T) {
	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	trade.PositionType = "takeProfit"

	held, err := ShouldHold(priorityEvent(trade, "buy", priorityWallet(), 0))
	if err != nil {
		t.Fatalf("a takeProfit must not be held by the wallet gate, got %v", err)
	}
	if len(held.Trade.Logs) != 0 {
		t.Fatalf("no row may be written on a close, got %v", messages(held.Trade.Logs))
	}
}

// Three ways to hold nothing, and all of them are the same posture: the flag
// owns the gate, a wallet view the engine did not build reserves nothing, and
// a balance it could not name is never guessed at.
func TestDepthPriorityIsInertWithoutTheFlagTheViewOrTheBalance(t *testing.T) {
	reserve := priorityReserve()

	withoutFlag := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	withoutFlag.Strategy.Params.Cooldown = false
	if _, err := ShouldHold(priorityEvent(withoutFlag, "buy", priorityWallet(), walletShortOf(t, withoutFlag, reserve))); err != nil {
		t.Fatalf("the wallet gate must not fire without params.Cooldown, got %v", err)
	}

	withoutView := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	if _, err := ShouldHold(priorityEvent(withoutView, "buy", nil, 0)); err != nil {
		t.Fatalf("a nil wallet view must hold nothing, got %v", err)
	}

	withoutBalance := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	if _, err := ShouldHold(blindPriorityEvent(withoutBalance, "buy", priorityWallet())); err != nil {
		t.Fatalf("a tick with no balance must hold nothing, got %v", err)
	}
}

// Both cooldown gates can be true on the same tick. The wallet-level reason
// is the one recorded: it names the situation the operator is looking at,
// while "the last depths were close together" describes only this ladder.
func TestDepthPriorityOutranksDepthSpacing(t *testing.T) {
	trade := testutil.DepthTrade(trade25858[0], trade25858[1])

	// This fixture carries no position price, so its own next entry prices at
	// nothing: what holds it is the reserve alone.
	event := priorityEvent(trade, "active", priorityWallet(), priorityReserve()-1)
	event.Timestamp = trade25858[1].Add(time.Minute).UnixMilli()

	spacingOnly := depthEvent(testutil.DepthTrade(trade25858[0], trade25858[1]), trade25858[1].Add(time.Minute))
	if _, err := ShouldHold(spacingOnly); err == nil {
		t.Fatal("this ladder must be held by depth spacing on its own")
	}

	held, err := ShouldHold(event)
	if err == nil {
		t.Fatal("expected the ladder to be held")
	}
	if len(held.Trade.Logs) != 1 {
		t.Fatalf("expected one row, got %v", messages(held.Trade.Logs))
	}

	row := held.Trade.Logs[0].Message
	if !strings.Contains(row, "cooldown: depth priority,") {
		t.Fatalf("row = %q, want the wallet reason", row)
	}
	if strings.Contains(row, "depths too close") {
		t.Fatalf("row = %q, want the wallet reason instead of the ladder one", row)
	}
}
