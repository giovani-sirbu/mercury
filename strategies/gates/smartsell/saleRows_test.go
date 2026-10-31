package smartsell

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The held row of the floor of the activation (heldUnderDepth, through saleHeldRow): its
// text, its filing, how it folds back, and that it goes out once per episode and gate.

// The reason text names the entries the ladder has filled and the least number the
// activation needs, and through saleHeldRow it is the one held row of either gate: the
// newest fill's price, the gate's filing, the event of a held sale, and the slow decline
// gate's row carries its own copy of the features while the flat market gate's carries none.
func TestTheFloorRowIsTheHeldRowGrammarOfEachGate(t *testing.T) {
	for _, c := range []struct {
		filled, need int
		text         string
	}{
		{0, 3, "depth 0 under the 3 depths the activation needs"},
		{1, 3, "depth 1 under the 3 depths the activation needs"},
		{2, 3, "depth 2 under the 3 depths the activation needs"},
		{2, 5, "depth 2 under the 5 depths the activation needs"},
	} {
		if got := heldUnderDepth(c.filled, c.need); got != c.text {
			t.Errorf("heldUnderDepth(%d, %d) = %q, want %q", c.filled, c.need, got, c.text)
		}
	}

	const fill = 168.2
	features := []string{FeatureQuietDecline, FeatureSwingDecline}
	slow := saleHeldRow(GateSlowDecline, fill, heldUnderDepth(2, 3), features)
	want := Row{
		Message: "smartSell: slow decline sale held, depth 2 under the 3 depths the activation needs",
		Price:   fill, Gate: GateSlowDecline, Event: EventSaleHeld, Features: []string{FeatureQuietDecline, FeatureSwingDecline},
	}
	if !reflect.DeepEqual(slow, want) {
		t.Errorf("slow decline row = %+v, want %+v", slow, want)
	}
	features[0] = "changed after the row was built"
	if slow.Features[0] != FeatureQuietDecline {
		t.Error("the slow decline row must carry its own copy of the features")
	}

	low := saleHeldRow(GateFlatMarket, fill, heldUnderDepth(1, 3), nil)
	want = Row{Message: "smartSell: flat market sale held, depth 1 under the 3 depths the activation needs", Price: fill, Gate: GateFlatMarket, Event: EventSaleHeld}
	if !reflect.DeepEqual(low, want) {
		t.Errorf("flat market row = %+v, want %+v", low, want)
	}
}

// The floor's row folds as the sale held of its own gate, the way the rows of the other two
// bounds do, each into its gate's flag: one flag per gate covers all three reasons, so the
// episode's row is written once whichever bound held it.
func TestTheFloorRowFoldsAsTheSaleHeldOfItsGate(t *testing.T) {
	withQuietDecline(t, true)
	withFlatMarket(t, true)
	trade := depthLadder(2)
	fill, at := trade.PositionPrice, testutil.At("21:45:00")

	slow := rebuildState(withRow(trade, saleHeldRow(GateSlowDecline, fill, heldUnderDepth(2, 3), []string{FeatureQuietDecline}), at))
	if !slow.saleHeld || slow.saleHeldRowDue() {
		t.Errorf("the slow decline floor row must fold as the sale held, got %+v", slow)
	}
	low := rebuildState(withRow(trade, saleHeldRow(GateFlatMarket, fill, heldUnderDepth(2, 3), nil), at))
	if !low.flatMarketActive || !low.flatMarketSaleHeld {
		t.Errorf("the flat market floor row must fold as the sale held of an open episode, got %+v", low)
	}
}

// Every trigger whose row is due is owed the floor's held row, in the order the triggers come
// (both gates standing give two rows, the slow decline's first when it comes first), one whose
// row is not due is owed none, and the step is none either way.
func TestTheFloorWritesTheDueRowsInTriggerOrder(t *testing.T) {
	floorOn(t)
	trade := depthLadder(SaleStartAtDepth - 1)
	price := aLossWithinTheCap(t, trade)
	why := underDepthText(SaleStartAtDepth-1, SaleStartAtDepth)
	slow, low := slowTrigger(true), lowTrigger(true)

	var cases []saleCase
	for _, set := range []struct {
		name     string
		triggers []saleTrigger
		owed     []saleTrigger
	}{
		{"both due, the slow decline first", []saleTrigger{slow, low}, []saleTrigger{slow, low}},
		{"both due, the flat market first", []saleTrigger{low, slow}, []saleTrigger{low, slow}},
		{"only the slow decline's row due", []saleTrigger{slowTrigger(true), lowTrigger(false)}, []saleTrigger{slow}},
		{"only the flat market's row due", []saleTrigger{slowTrigger(false), lowTrigger(true)}, []saleTrigger{low}},
		{"no row due", []saleTrigger{slowTrigger(false), lowTrigger(false)}, nil},
	} {
		c := onLadder(set.name, trade, "buy", price, set.triggers...)
		for _, trigger := range set.owed {
			c.rows = append(c.rows, heldRowOf(trigger, trade.PositionPrice, why))
		}
		cases = append(cases, c)
	}
	runSale(t, cases)
}

// flatMarketLadderOf is the round ladder (flatMarketLadder) with its first filled entries
// only, resting in buy at the newest of them.
func flatMarketLadderOf(t *testing.T, filled int) aggragates.Trades {
	t.Helper()
	trade := flatMarketLadder(false)
	if filled > len(roundPrices) {
		t.Skipf("the round ladder holds %d fills, fewer than the %d asked for", len(roundPrices), filled)
	}
	if filled < 1 {
		t.Fatalf("fixture drifted: a ladder of %d fills asked for", filled)
	}
	trade.History = slices.Clone(trade.History[:len(trade.History)-len(roundPrices)+filled])
	trade.PositionPrice = roundPrices[filled-1]
	if got := ladder.CountFilledEntries(trade); got != filled {
		t.Fatalf("fixture drifted: want %d filled entries, got %d", filled, got)
	}
	return trade
}

// On a tick the whole way through Apply, the flat market gate's episode OPENS at the first
// fill under the floor and STANDS under it, only its activation waits: the first tick writes
// the triggered row and the floor's held row, activates nothing, and the second writes nothing
// at all, the held row being due once per episode, with the ladder still in buy and the
// episode open and held. ExitReached reads false on both ticks and writes nothing.
func TestTheFloorHoldsTheFlatMarketActivationOnceAnEpisodeAndTheEpisodeStands(t *testing.T) {
	flatMarketAlone(t)
	if SaleStartAtDepth <= 1 {
		t.Skip("SaleStartAtDepth is off or leaves no entry count under it")
	}
	withSaleStartAtDepth(t, SaleStartAtDepth)
	for filled := 1; filled < SaleStartAtDepth && filled <= len(roundPrices); filled++ {
		fresh := flatMarketLadderOf(t, filled)
		ai, fill, price := flatMarketMarket(fresh), fresh.PositionPrice, aLossWithinTheCap(t, fresh)
		opened := testutil.At("22:00:00")

		trade, got := engineStep(fresh, "", price, opened, ai)
		assertWaits(t, got, "", triggeredBy(fresh, ai), heldRowOf(lowTrigger(true), fill, underDepthText(filled, SaleStartAtDepth)))
		if state := rebuildState(trade); !state.flatMarketActive || !state.flatMarketSaleHeld || state.flatMarketActivated || state.flatMarketFrom != fill {
			t.Fatalf("%d fills: the episode stands open and held at the newest fill, not activated, got %+v", filled, state)
		}
		rows, events := len(trade.Logs), len(trade.StrategyEvents)
		if ExitReached(trade, price, ai.SmartSell) {
			t.Errorf("%d fills: a ladder under the floor is no exit", filled)
		}

		trade, got = engineStep(trade, "", price, opened.Add(time.Minute), ai)
		assertUntouched(t, got, "")
		if trade.PositionType != "buy" || len(trade.Logs) != rows || len(trade.StrategyEvents) != events {
			t.Errorf("%d fills: the second tick under the floor writes nothing and leaves the ladder in buy, got %q with %d rows and %d events after %d and %d", filled, trade.PositionType, len(trade.Logs), len(trade.StrategyEvents), rows, events)
		}
	}

	// The control: the same market on the ladder at the floor opens its episode and activates.
	atTheFloor := flatMarketLadderOf(t, SaleStartAtDepth)
	ai, price := flatMarketMarket(atTheFloor), aLossWithinTheCap(t, atTheFloor)
	assertActivated(t, Apply(atTheFloor, "", price, ai), triggeredBy(atTheFloor, ai), FlatMarketActivatedRow(atTheFloor.PositionPrice, -handLossPct(atTheFloor, price)))
}
