package smarttakeloss

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

func TestCapitalProtectionFixture(t *testing.T) {
	trade := lastDepthLadder()
	if ladder.ConfiguredDepths(trade) != lastDepthFills || ladder.CountFilledEntries(trade) != lastDepthFills {
		t.Fatal("fixture drifted: the ladder must hold its last configured depth")
	}
	if !(trade.PositionPrice < capitalProtectionBand && capitalProtectionBand < ladder.AverageEntryPrice(trade)) {
		t.Fatal("fixture drifted: the band must sit over the newest fill and under break even")
	}
}

// Capital protection watches a ladder from its last configured depth — a
// partial fill of that depth's order and a row lowered under the fills
// included — and not one depth short of it. A ladder whose depth count is
// unknown, an inverse ladder, a futures ladder and a ladder under an impasse
// strategy are never watched, and nothing is while CapitalProtectionExit is
// off. rebuildState reads the same watch off its fills.
func TestCapitalProtectionWatchesTheLastDepthOnly(t *testing.T) {
	withCapitalProtectionExit(t, true)
	partial := lastDepthLadder()
	partial.History[lastDepthFills-1].Quantity = 0.4
	lowered := lastDepthLadder()
	lowered.StrategyPair.StrategySettings[0].Depths = lastDepthFills - 1
	unknown := lastDepthLadder()
	unknown.StrategyPair.StrategySettings[0].Depths = 0
	futures := lastDepthLadder()
	futures.Strategy.TradeType = aggragates.Futures
	impasse := lastDepthLadder()
	impasse.Strategy.Params.Impasse = true

	for name, tc := range map[string]struct {
		trade aggragates.Trades
		want  bool
	}{
		"at its last depth":                {lastDepthLadder(), true},
		"a partial fill of the last depth": {partial, true},
		"a row lowered under its fills":    {lowered, true},
		"one depth short":                  {testutil.LadderTrade(false, fills(lastDepthFills-1, "21:30:00")...), false},
		"an unknown depth count":           {unknown, false},
		"an inverse ladder":                {testutil.LadderTrade(true, fills(lastDepthFills, "21:30:00")...), false},
		"a futures ladder":                 {futures, false},
		"an impasse strategy":              {impasse, false},
	} {
		if got := capitalProtectionWatched(tc.trade); got != tc.want {
			t.Errorf("%s: watched %v, want %v", name, got, tc.want)
		}
		if got := rebuildState(tc.trade).capitalProtectionWatched; got != tc.want {
			t.Errorf("%s: rebuildState reads the watch as %v, want %v", name, got, tc.want)
		}
	}

	withCapitalProtectionExit(t, false)
	if capitalProtectionWatched(lastDepthLadder()) || rebuildState(lastDepthLadder()).capitalProtectionWatched {
		t.Fatal("switched off, capital protection watches no ladder")
	}
}

// The band is inclusive and exact: at the band the price sells, one
// representable price under it does not, and no tolerance widens it. It
// needs the SMC trend bearish and a band above zero.
func TestCapitalProtectionReached(t *testing.T) {
	for price, want := range map[float64]bool{
		capitalProtectionBand:                    true,
		capitalProtectionBand + 5:                true,
		math.Nextafter(capitalProtectionBand, 0): false,
	} {
		if got := capitalProtectionReached(price, solBlock()); got != want {
			t.Errorf("price %v against the band %v: %v, want %v", price, capitalProtectionBand, got, want)
		}
	}
	notBearish := solBlock()
	notBearish.CapitalProtectionSmcBearish = false
	if capitalProtectionReached(capitalProtectionBand+5, notBearish) {
		t.Fatal("without the SMC trend bearish nothing sells")
	}
	for _, none := range []float64{0, -capitalProtectionBand} {
		noBand := solBlock()
		noBand.CapitalProtectionUpperBB = none
		if capitalProtectionReached(capitalProtectionBand, noBand) || capitalProtectionReached(0, noBand) {
			t.Fatalf("a band of %v must sell nothing", none)
		}
	}
}

// A watched ladder sells at the band from the dead zone and from every
// proposal that adds or re-arms an add, in every state that is not a close,
// and writes no row: capital protection keeps no state. One representable
// price under the band the proposal passes.
func TestApplyCapitalProtectionSellsFromEveryAddSideProposal(t *testing.T) {
	withCapitalProtectionExit(t, true)
	for _, state := range []string{"buy", "stopLoss", "forceTrailingStopLoss"} {
		trade := lastDepthLadder()
		trade.PositionType = state
		for _, position := range []string{"", "buy", "stopLoss", "update_stopLoss", "update_buy", "forceTrailingStopLoss"} {
			got := Apply(trade, position, capitalProtectionBand, withBlock(solBlock()))
			assertForced(t, got, reasonCapitalProtection)
			if got.SlowDecline != nil {
				t.Fatalf("capital protection writes no row, got %+v", got)
			}
			assertUntouched(t, Apply(trade, position, math.Nextafter(capitalProtectionBand, 0), withBlock(solBlock())), position)
		}
	}
	got := Apply(lastDepthLadder(), "", capitalProtectionBand, withBlock(solBlock()))
	if msg := ExitMessage(lastDepthLadder(), got.Reason, capitalProtectionBand); msg != "smartTakeLoss: sell at upper bollinger band (capital protection) 172.40" {
		t.Fatalf("unexpected exit row %q", msg)
	}
}

// It never replaces a close: one the ladder proposes, or the trade's own
// state when that is a close — the trailing take profit included — passes
// through at and over the band. It leaves the take profit reading alone and
// holds nothing: the first fill of a new ladder is not held on it.
func TestApplyCapitalProtectionNeverReplacesAClose(t *testing.T) {
	withCapitalProtectionExit(t, true)
	for _, position := range []string{"sell", "takeProfit", "update_takeProfit", "sellParent", "impasse", "sellLoss", "forceTrailingTakeProfit"} {
		assertUntouched(t, Apply(lastDepthLadder(), position, capitalProtectionBand+5, withBlock(solBlock())), position)
	}
	for _, state := range []string{"sell", "takeProfit", "sellParent", "impasse", "sellLoss", "forceTrailingTakeProfit"} {
		trade := lastDepthLadder()
		trade.PositionType = state
		for _, position := range []string{"", "stopLoss"} {
			assertUntouched(t, Apply(trade, position, capitalProtectionBand+5, withBlock(solBlock())), position)
		}
	}
	for _, price := range []float64{capitalProtectionBand, 2 * capitalProtectionBand} {
		if got := TakeProfitPercentage(lastDepthLadder(), price, breakEvenReading); got != breakEvenReading {
			t.Fatalf("at %v the take profit must read its input, got %v", price, got)
		}
	}
	if got := EntryHoldReason(testutil.LadderTrade(false), aggragates.SideLong, withBlock(solBlock())); got != "" {
		t.Fatalf("capital protection holds no first fill, got %q", got)
	}
}

// A pending slow decline and capital protection can both reach their bands on
// one tick: it is the same sale, and the sell band names it. Under the sell
// band capital protection names it, and so it does with the slow decline
// switched off.
func TestApplySellBandNamesTheSaleWhenBothBandsAreReached(t *testing.T) {
	withCapitalProtectionExit(t, true)
	pending := lastDepthLadder()
	pending.Logs = []aggragates.TradesLogs{{Message: SlowDeclineMessage("buy", nil), Price: pending.PositionPrice}}
	block := solBlock()
	block.SlowDeclineSellBand = capitalProtectionBand
	assertForced(t, Apply(pending, "", capitalProtectionBand, withBlock(block)), reasonSellBand)

	block.SlowDeclineSellBand = capitalProtectionBand + 1
	assertForced(t, Apply(pending, "", capitalProtectionBand, withBlock(block)), reasonCapitalProtection)

	withQuietSlowDeclineExit(t, false)
	block.SlowDeclineSellBand = capitalProtectionBand
	assertForced(t, Apply(pending, "", capitalProtectionBand, withBlock(block)), reasonCapitalProtection)
}
