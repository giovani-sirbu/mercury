package cooldown

import (
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The smart take loss finds this gate's rows by DepthPriorityHoldMarker
// anywhere in their message, so the marker and both hold messages are
// schema: the marker pinned byte for byte, and each message byte-identical
// to the text the gate wrote before the marker was exported — a full ladder
// in front from its last depth on, with depths left one under it.
func TestDepthPriorityHoldMessagesAreByteStable(t *testing.T) {
	if DepthPriorityHoldMarker != "cooldown: depth priority" {
		t.Fatalf("the marker is schema and must not move, got %q", DepthPriorityHoldMarker)
	}
	own := aggragates.LadderDepth{Symbol: "ETH/USDT", Depth: 4, MaxDepth: 8}
	for _, tc := range []struct {
		priority aggragates.LadderDepth
		want     string
	}{
		{aggragates.LadderDepth{Symbol: "LINK/USDT", Depth: 8, MaxDepth: 8}, "cooldown: depth priority, LINK/USDT at depth 8 of 8 holds the wallet until it closes, this ladder waits at depth 4 of 8"},
		{aggragates.LadderDepth{Symbol: "LINK/USDT", Depth: 9, MaxDepth: 8}, "cooldown: depth priority, LINK/USDT at depth 9 of 8 holds the wallet until it closes, this ladder waits at depth 4 of 8"},
		{aggragates.LadderDepth{Symbol: "LINK/USDT", Depth: 7, MaxDepth: 8}, "cooldown: depth priority, LINK/USDT at depth 7 of 8 keeps the wallet for its remaining depths, this ladder waits at depth 4 of 8"},
	} {
		got := depthPriorityHoldMessage(tc.priority, own)
		if got != tc.want {
			t.Errorf("message = %q, want %q", got, tc.want)
		}
		if !strings.HasPrefix(got, DepthPriorityHoldMarker+", ") {
			t.Errorf("%q must open with the marker", got)
		}
	}
}

// The row the gate really writes carries the marker and the tick clock: the
// reason DepthPriorityHoldReason gives on a wallet that cannot spare the
// entry opens with the marker, and gates.SaveHoldLog frames it as
// "Hold <position>: …" stamped with the tick the chain ran on — the stamp the
// smart take loss weighs against the ladder's newest fill.
func TestTheDepthPriorityHoldRowCarriesTheMarkerAndTheTick(t *testing.T) {
	requireDepthPriority(t)

	wallet := walletView()
	trade := testutil.LadderDepthTrade(12, "ETH/USDT", 4, walletDepths)
	reserve := viewReserve(t, wallet, walletAsset)
	event := priorityEvent(trade, "buy", wallet, walletShortFor(t, trade, reserve))
	reason := DepthPriorityHoldReason(event, "stopLoss")
	if want := "cooldown: depth priority, LINK/USDT at depth 7 of 8 keeps the wallet for its remaining depths, this ladder waits at depth 4 of 8"; reason != want {
		t.Fatalf("reason = %q, want %q", reason, want)
	}

	tick := time.Date(2022, time.May, 9, 14, 5, 0, 0, time.UTC)
	event.Timestamp = tick.UnixMilli()
	event.Events = map[string]func(events.Events) (events.Events, error){"updateTrade": testutil.NopUpdateTrade}
	held, err := gates.SaveHoldLog(event, "stopLoss", reason)
	if err == nil || len(held.Trade.Logs) != 1 {
		t.Fatalf("the hold must stop the chain and write one row, got %v and %+v", err, held.Trade.Logs)
	}
	row := held.Trade.Logs[0]
	if row.Message != "Hold stopLoss: "+reason || !strings.Contains(row.Message, DepthPriorityHoldMarker) || !row.CreatedAt.Equal(tick) {
		t.Fatalf("the row must frame the reason, carry the marker and the tick %v, got %+v", tick, row)
	}
}

// No other row this package writes names the marker, so the smart take loss
// never reads another cooldown gate's row as a depth priority hold: not the
// first-fill gate's rows, and not depth spacing's hold, whose text shares the
// marker's opening words.
func TestNoOtherCooldownRowNamesTheDepthPriorityMarker(t *testing.T) {
	first := testutil.At("09:00:00")
	spaced := testutil.DepthTrade(first, first.Add(5*time.Minute))
	spaced.PositionPrice = spaced.History[1].Price
	spacing := DepthSpacingHoldReason(events.Events{Trade: spaced, Timestamp: first.Add(10 * time.Minute).UnixMilli()}, "stopLoss")
	if !strings.HasPrefix(spacing, "cooldown: depth") {
		t.Fatalf("fixture drifted: two depths five minutes apart must be held by depth spacing, got %q", spacing)
	}
	for _, text := range []string{FirstFillWaitingPrefix, FirstFillArmedPrefix, FirstFillEnteredPrefix, spacing} {
		if strings.Contains(text, DepthPriorityHoldMarker) {
			t.Errorf("%q names the depth priority marker", text)
		}
	}
}
