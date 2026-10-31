package smartsell

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The prints that mattered in trade 212714 (HBAR, 2025-10-10), written by hand in the order the
// tape gave them: a free fall prevention episode open with its sale held, then one print of a
// one-millisecond buy sweep (21:17:39.006) that put the profit over the trigger, and the very next
// print (21:17:39.012) back down where the market was. Without the two guards the sweep activated
// the trailingBuy, anchored it at the sweep, and the next print sold ten percent under it.

// spikeFills are the six fills of the ladder: price and units, the tolerance of every depth 0.1.
var spikeFills = []struct{ price, units float64 }{
	{0.21671, 1757}, {0.20511, 3514}, {0.19399, 7028}, {0.17259, 14056}, {0.1651, 28112}, {0.15432, 56224},
}

// spikeLadder is the ladder resting in buy, fee-free, priced to five decimals, every depth's row at tolerance.
func spikeLadder(tolerance float64) aggragates.Trades {
	trade := testutil.LadderTrade(false)
	trade.Symbol = "HBAR/USDT"
	trade.StrategyPair.TradeFilters.PriceFilter = 5
	trade.StrategyPair.StrategySettings[0].Tolerance = tolerance
	for index, fill := range spikeFills {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: "BUY", Quantity: fill.units, Price: fill.price, OrderId: int64(index + 1),
			CreatedAt: time.Date(2025, 10, 10, 13+index, 0, 0, 0, time.UTC),
		})
		trade.PositionPrice = fill.price
	}
	return trade
}

// spikeProfit is the profit, in percent of the amount used, of selling every unit at price: 18208.58
// used for 110691 units, so the average is 0.16450 and the trigger of -0.15 % sits at 0.16425.
func spikeProfit(price float64) float64 {
	var used, units float64
	for _, fill := range spikeFills {
		used += fill.price * fill.units
		units += fill.units
	}
	return (price*units - used) / used * 100
}

// spikeAt is a print's clock on the day of the trade, to the millisecond.
func spikeAt(hour, minute, second, millisecond int) time.Time {
	return time.Date(2025, 10, 10, hour, minute, second, millisecond*int(time.Millisecond), time.UTC)
}

// spikeEpisode is the ladder with the free fall prevention episode open and its sale held under the
// trigger of -0.15 % (the profit then about -5 %), the state the sweep met, at a tolerance of 0.1.
func spikeEpisode(t *testing.T) (aggragates.Trades, aggragates.AIIndicators) {
	t.Helper()
	return spikeEpisodeAt(t, 0.1)
}

// spikeEpisodeAt is spikeEpisode at the tolerance given.
func spikeEpisodeAt(t *testing.T, tolerance float64) (aggragates.Trades, aggragates.AIIndicators) {
	t.Helper()
	freeFallPreventionAlone(t)
	withAllowedLoss(t, 0.15)
	trade := spikeLadder(tolerance)
	if row := trailRow(trade); row.Tolerance != tolerance || ladderFilled(trade) != len(spikeFills) {
		t.Fatalf("fixture drifted: six fills at a tolerance of %v, got %d and %+v", tolerance, ladderFilled(trade), row)
	}
	if sweep, back := spikeProfit(0.16464), spikeProfit(0.14957); sweep < -0.15 || back >= -0.15 {
		t.Fatalf("fixture drifted: the sweep print must reach the trigger and the print after it fall short, got %v and %v", sweep, back)
	}
	fill := trade.PositionPrice
	trade = Append(trade, freeFallTriggeredBy(trade), spikeAt(21, 0, 0, 0))
	trade = Append(trade, FreeFallPreventionSaleHeldRow(fill, -5.05), spikeAt(21, 0, 1, 0))
	return trade, freeFallMarket(trade)
}

// ladderFilled is the entries filled on the trade.
func ladderFilled(trade aggragates.Trades) int {
	return len(rebuildState(trade).fills)
}

// spikeGapWait is the wait of the gap hold the replays switch on.
const spikeGapWait = 5 * time.Second

// spikeToTheGap plays the tape of trade 212714 with the three guards on (the confirmation, the gap
// floor, the gap wait at wait) up to the print that opens the gap: the sweep print writes the reached
// row and holds, the print after it, short of the trigger, writes nothing and leaves the stamp; the
// stamp is stale by 22:05, where the first reached print writes a new row and the third print of the
// confirmation activates, anchored at 0.16475; the next print, ten percent under the anchor, is held
// with its one gap row, stamped 22:05:07. It hands back the ladder, the fill and the engine tick for
// the prints after it.
func spikeToTheGap(t *testing.T, wait time.Duration) (*aggragates.Trades, float64, func(time.Time, float64) Result) {
	t.Helper()
	trade, ai := spikeEpisode(t)
	withActivationConfirmation(t, 5*time.Second)
	withActivationConfirmationWindow(t, time.Minute)
	withSaleGapTolerances(t, 5)
	withSaleGapWait(t, wait)
	fill := trade.PositionPrice
	reached := func(price float64) Row {
		return TriggerReachedRow(GateFreeFallPrevention, fill, spikeProfit(price), -0.15)
	}
	var got Result
	step := func(at time.Time, price float64) Result {
		t.Helper()
		trade, got = engineStepAt(trade, "", price, at, ai)
		return got
	}

	assertUntouched(t, step(spikeAt(21, 17, 38, 22), 0.14883), "")

	assertConfirming(t, step(spikeAt(21, 17, 39, 6), 0.16464), reached(0.16464))

	assertUntouched(t, step(spikeAt(21, 17, 39, 12), 0.14957), "")
	if stamp := memoryOf(rebuildState(trade), GateFreeFallPrevention).reachedAt; !stamp.Equal(spikeAt(21, 17, 39, 6)) {
		t.Fatalf("the print after the sweep must leave the sweep's stamp standing, got %v", stamp)
	}

	assertUntouched(t, step(spikeAt(21, 17, 40, 28), 0.15120), "")

	assertConfirming(t, step(spikeAt(22, 5, 0, 0), 0.16480), reached(0.16480))
	assertConfirming(t, step(spikeAt(22, 5, 3, 0), 0.16470))
	assertActivated(t, step(spikeAt(22, 5, 6, 0), 0.16475), FreeFallPreventionActivatedRow(fill, spikeProfit(0.16475)))
	if state, _ := activationPathOf(0 - spikeProfit(0.16475)); trade.PositionType != state || trade.PositionPrice != spikeAnchor {
		t.Fatalf("the activation must leave the ladder in %s anchored at the confirming print, got %q at %v", state, trade.PositionType, trade.PositionPrice)
	}

	assertTrailHeld(t, step(spikeAt(22, 5, 7, 0), 0.14957), spikeGapRow(fill, 0.14957))
	if stamp := memoryOf(rebuildState(trade), GateFreeFallPrevention).gapHeldAt; !stamp.Equal(spikeAt(22, 5, 7, 0)) {
		t.Fatalf("the gap row must stamp the gap at its print's clock, got %v", stamp)
	}
	return &trade, fill, step
}

// spikeAnchor is the print the trailingBuy is anchored at, spikeGapRow the gap row of a print under it.
const spikeAnchor = 0.16475

func spikeGapRow(fill, price float64) Row {
	return SaleHeldByGapRow(GateFreeFallPrevention, fill, (spikeAnchor-price)/price*100, 0.5, 5)
}

// With the guards on the sweep sells nothing, and the gap that follows the activation is held with
// its one row, then SaleGapWait later sells at the print: the print after the row, 22:05:08, holds
// with no row, a print 1 ms short of the wait holds, and the print at 22:05:12 (5 s after the row)
// sells under the gate's reason.
func TestTheSpikeOfTrade212714IsNotSoldWithAllGuardsOnUntilTheGapStandsItsWait(t *testing.T) {
	_, _, step := spikeToTheGap(t, spikeGapWait)

	assertTrailHeld(t, step(spikeAt(22, 5, 8, 0), 0.15000))
	assertTrailHeld(t, step(spikeAt(22, 5, 11, 999), 0.15010))
	assertSold(t, step(spikeAt(22, 5, 12, 0), 0.15020), reasonFreeFallPrevention)
}

// With the wait off the same tape holds the gap without end: the print at 22:05:12 and one an hour
// later hold with no row, and the print back inside the floor, 22:06:00, sells.
func TestTheSpikeOfTrade212714HoldsWithoutEndWithTheWaitOffAndSellsInsideTheFloor(t *testing.T) {
	_, _, step := spikeToTheGap(t, 0)

	assertTrailHeld(t, step(spikeAt(22, 5, 8, 0), 0.15000))
	assertTrailHeld(t, step(spikeAt(22, 5, 12, 0), 0.15020))
	assertTrailHeld(t, step(spikeAt(23, 5, 12, 0), 0.14000))
	assertSold(t, step(spikeAt(23, 6, 0, 0), 0.16420), reasonFreeFallPrevention)
}

// A gap that closes and opens again waits again: the print back over the floor at 22:05:09 writes the
// gapClosed row and holds, the gap that follows at 22:05:10 writes a new gap row, and the sale comes
// SaleGapWait after that row (22:05:15), not after the first one (22:05:12 would have sold).
func TestTheSpikeOfTrade212714WaitsAgainOnAGapThatClosedAndOpenedAgain(t *testing.T) {
	trade, fill, step := spikeToTheGap(t, spikeGapWait)

	back := 0.16480
	assertTrailHeld(t, step(spikeAt(22, 5, 9, 0), back), GapClosedRow(GateFreeFallPrevention, fill, (back-spikeAnchor)/back*100))
	if memory := memoryOf(rebuildState(*trade), GateFreeFallPrevention); memory.gapHeld || !memory.activated {
		t.Fatalf("the close must zero the gap stamp and leave the gate activated, got %+v", memory)
	}

	assertTrailHeld(t, step(spikeAt(22, 5, 10, 0), 0.14900), spikeGapRow(fill, 0.14900))
	assertTrailHeld(t, step(spikeAt(22, 5, 12, 0), 0.15020))
	assertTrailHeld(t, step(spikeAt(22, 5, 14, 999), 0.15030))
	assertSold(t, step(spikeAt(22, 5, 15, 0), 0.15040), reasonFreeFallPrevention)
}

// The same tape with the guards off is the trade as it happened: the sweep print activates, anchored at
// the sweep, and the very next print sells at once, ten percent under the anchor.
func TestTheSameTapeSellsAtTheSpikeWithBothGuardsOff(t *testing.T) {
	trade, ai := spikeEpisode(t)
	fill := trade.PositionPrice

	trade, got := engineStepAt(trade, "", 0.16464, spikeAt(21, 17, 39, 6), ai)
	assertActivated(t, got, FreeFallPreventionActivatedRow(fill, spikeProfit(0.16464)))
	_, got = engineStepAt(trade, "", 0.14957, spikeAt(21, 17, 39, 12), ai)
	assertSold(t, got, reasonFreeFallPrevention)
}

// Each guard stops the sale alone: the confirmation, the sweep never activating, and the gap floor,
// the sweep activating and the print under its anchor held; a row with no tolerance has no floor, so
// the same sweep sells at the next print even with the floor on.
func TestEachGuardStopsTheSpikeOnItsOwn(t *testing.T) {
	t.Run("the confirmation alone", func(t *testing.T) {
		trade, ai := spikeEpisode(t)
		withConfirmation(t)
		fill := trade.PositionPrice

		trade, got := engineStepAt(trade, "", 0.16464, spikeAt(21, 17, 39, 6), ai)
		assertConfirming(t, got, TriggerReachedRow(GateFreeFallPrevention, fill, spikeProfit(0.16464), -0.15))
		trade, got = engineStepAt(trade, "", 0.14957, spikeAt(21, 17, 39, 12), ai)
		assertUntouched(t, got, "")
		if trade.PositionType != "buy" {
			t.Errorf("nothing was activated, the ladder must rest in buy, got %q", trade.PositionType)
		}
	})

	t.Run("the gap floor alone", func(t *testing.T) {
		trade, ai := spikeEpisode(t)
		withSaleGapTolerances(t, 5)
		fill := trade.PositionPrice

		trade, got := engineStepAt(trade, "", 0.16464, spikeAt(21, 17, 39, 6), ai)
		assertActivated(t, got, FreeFallPreventionActivatedRow(fill, spikeProfit(0.16464)))
		trade, got = engineStepAt(trade, "", 0.14957, spikeAt(21, 17, 39, 12), ai)
		assertTrailHeld(t, got, SaleHeldByGapRow(GateFreeFallPrevention, fill, 10.08, 0.5, 5))
		_, got = engineStepAt(trade, "", 0.16420, spikeAt(21, 18, 0, 0), ai)
		assertSold(t, got, reasonFreeFallPrevention)
	})

	t.Run("a row with no tolerance has no floor", func(t *testing.T) {
		trade, ai := spikeEpisodeAt(t, 0)
		withSaleGapTolerances(t, 5)
		fill := trade.PositionPrice

		trade, got := engineStepAt(trade, "", 0.16464, spikeAt(21, 17, 39, 6), ai)
		assertActivated(t, got, FreeFallPreventionActivatedRow(fill, spikeProfit(0.16464)))
		_, got = engineStepAt(trade, "", 0.14957, spikeAt(21, 17, 39, 12), ai)
		assertSold(t, got, reasonFreeFallPrevention)
	})
}
