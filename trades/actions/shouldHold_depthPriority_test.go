package actions

import (
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// walletDepths is the depths every ladder of the fixture wallet is
// configured for.
const walletDepths = 8

// priorityWallet is a wallet whose pairs fell together: several part-filled
// ladders and one short of the last depth it was sized for.
func priorityWallet() []aggragates.LadderDepth {
	return []aggragates.LadderDepth{
		{TradeID: 11, Symbol: "BTC/USDT", Depth: 5, MaxDepth: walletDepths},
		{TradeID: 12, Symbol: "ETH/USDT", Depth: 4, MaxDepth: walletDepths},
		{TradeID: 14, Symbol: "LINK/USDT", Depth: 7, MaxDepth: walletDepths},
	}
}

// priorityEvent runs a trade through ShouldHold on a tick carrying the wallet
// view. The clock is left unknown, which is what keeps depth spacing out of
// the way of these assertions; the one test that wants both gates speaking
// stamps it.
func priorityEvent(trade aggragates.Trades, oldPosition string, ladders []aggragates.LadderDepth) events.Events {
	return events.Events{
		Trade: trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{OldPosition: oldPosition, WalletLadders: ladders},
	}
}

// An add of a shallower ladder waits for the one that is nearly finished, and
// the row says which ladder it is waiting for.
func TestDepthPriorityHoldsAnAddThroughShouldHold(t *testing.T) {
	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)

	held, err := ShouldHold(priorityEvent(trade, "buy", priorityWallet()))
	if err == nil {
		t.Fatal("a ladder shallower than the priority one must be held")
	}
	if len(held.Trade.Logs) != 1 {
		t.Fatalf("expected one row, got %v", messages(held.Trade.Logs))
	}

	row := held.Trade.Logs[0]
	want := "Hold stopLoss: cooldown: depth priority, LINK/USDT at depth 7 of 8 takes the next entry, this ladder waits at depth 4 of 8"
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

// The first fill is gated too, and by this gate first: the wallet decides
// whether the trade may spend anything at all, so a held entry must not
// consume the sophos verdict the first-fill gate would have activated its own
// hold on. The verdict here refuses the side, so the first-fill gate would
// have written its waiting row if it had been asked.
func TestDepthPriorityHoldsTheFirstFillBeforeTheFirstFillGate(t *testing.T) {
	trade := testutil.LadderDepthTrade(21, "ADA/USDT", 0, walletDepths)
	trade.PositionType = "buy"
	trade.PositionPrice = 100

	event := priorityEvent(trade, "new", priorityWallet())
	event.Params.CoolDownIndicators = aggragates.CoolDownIndicators{HasFirstFillVerdict: true}

	held, err := ShouldHold(event)
	if err == nil {
		t.Fatal("a first fill must wait for the ladder that is nearly finished")
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

// The close is what releases every ladder the wallet was reserved against, so
// it is never gated.
func TestDepthPriorityLeavesACloseAlone(t *testing.T) {
	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	trade.PositionType = "takeProfit"

	held, err := ShouldHold(priorityEvent(trade, "buy", priorityWallet()))
	if err != nil {
		t.Fatalf("a takeProfit must not be held by the wallet gate, got %v", err)
	}
	if len(held.Trade.Logs) != 0 {
		t.Fatalf("no row may be written on a close, got %v", messages(held.Trade.Logs))
	}
}

// The flag owns the gate, and a wallet view the engine did not build holds
// nothing: both are the fail-open side.
func TestDepthPriorityIsInertWithoutTheFlagOrAWalletView(t *testing.T) {
	withoutFlag := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	withoutFlag.Strategy.Params.Cooldown = false

	if _, err := ShouldHold(priorityEvent(withoutFlag, "buy", priorityWallet())); err != nil {
		t.Fatalf("the wallet gate must not fire without params.Cooldown, got %v", err)
	}

	withoutView := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	if _, err := ShouldHold(priorityEvent(withoutView, "buy", nil)); err != nil {
		t.Fatalf("a nil wallet view must hold nothing, got %v", err)
	}
}

// Both cooldown gates can be true on the same tick. The wallet-level reason
// is the one recorded: it names the situation the operator is looking at,
// while "the last depths were close together" describes only this ladder.
func TestDepthPriorityOutranksDepthSpacing(t *testing.T) {
	trade := testutil.DepthTrade(trade25858[0], trade25858[1])

	event := priorityEvent(trade, "active", priorityWallet())
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
