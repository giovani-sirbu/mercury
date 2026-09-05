package actions

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The first-fill gate's contract with the chain: a refused verdict holds the
// new trade through gates.SaveHoldLog at the tick price, and the release
// through the reference hands the chain an event carrying the entered row —
// shouldHoldEntry must pass that event on, or the row NextDepthDoubled reads
// never reaches updateTrade.

func firstFillEvent(price float64, verdict aggragates.CoolDownIndicators) events.Events {
	trade := withCooldown(testutil.NewHoldTrade("buy", false))
	trade.PositionPrice = price
	return events.Events{
		Trade: trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params:    aggragates.Params{OldPosition: "new", CoolDownIndicators: verdict},
		Timestamp: testutil.At("09:00:00").UnixMilli(),
	}
}

func TestShouldHoldFirstFillActivatesAtTheTickPrice(t *testing.T) {
	held, err := ShouldHold(firstFillEvent(100, aggragates.CoolDownIndicators{HasFirstFillVerdict: true}))
	if err == nil {
		t.Fatal("a refused verdict must hold the first fill")
	}
	want := "Hold entry: cooldown: trying to get a better entry price: reference 100.0000, enters above 102.5641 or below 97.4184 after a bounce"
	if len(held.Trade.Logs) != 1 || held.Trade.Logs[0].Message != want {
		t.Fatalf("rows = %v, want %q", messages(held.Trade.Logs), want)
	}
	if held.Trade.Logs[0].Price != 100 || held.Trade.PositionPrice != 0 {
		t.Fatalf("the row carries the tick (%v) and PositionPrice stays 0 (%v)", held.Trade.Logs[0].Price, held.Trade.PositionPrice)
	}
	if cooldown.FirstFillVerdictNeeded(held.Trade, "new") {
		t.Fatal("once the hold stands the engines stop fetching the verdict")
	}
}

func TestShouldHoldFirstFillCarriesTheEnteredRowToTheChain(t *testing.T) {
	held, err := ShouldHold(firstFillEvent(100, aggragates.CoolDownIndicators{HasFirstFillVerdict: true}))
	if err == nil {
		t.Fatal("a refused verdict must hold the first fill")
	}

	// The next tick prints above up(R) = 102.5641; the verdict is no longer
	// fetched, so the payload is empty.
	held.Trade.PositionPrice = 103
	held.Params.CoolDownIndicators = aggragates.CoolDownIndicators{}
	released, err := ShouldHold(held)
	if err != nil {
		t.Fatalf("above the reference the entry must proceed, got %v", err)
	}
	if len(released.Trade.Logs) != 2 || !strings.HasPrefix(released.Trade.Logs[1].Message, cooldown.FirstFillEnteredPrefix) {
		t.Fatalf("the returned event must carry the entered row, got %v", messages(released.Trade.Logs))
	}
	if released.Trade.Logs[1].Price != 103 {
		t.Fatalf("entered row price = %v, want the tick 103", released.Trade.Logs[1].Price)
	}

	// The market fill that follows is the one depth the correction applies to.
	released.Trade.History = []aggragates.TradesHistory{{Type: "BUY", Quantity: 1, Price: 103, OrderId: 1}}
	if !cooldown.NextDepthDoubled(released.Trade) {
		t.Fatal("the depth after an entry above the reference arms at double the step")
	}
}
