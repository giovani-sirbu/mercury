package smartsell

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The sale's terminal is its realised result at the print, whichever state it sells from: a gain
// or break even ends in sell, a loss in takeLoss, from a committed takeProfit and from a committed
// trailingBuy alike, and the exit row says which.
func TestTheSaleEndsInTheTerminalOfItsRealisedResult(t *testing.T) {
	flatMarketAlone(t)
	withAllowedLoss(t, anyLoss)
	gate := pathGates()[1]
	for activatedAt, state := range map[float64]string{pathGain: "takeProfit", pathUnder: "trailingBuy"} {
		committed := gate.committedAt(t, activatedAt)
		ai := gate.ai(committed)
		for _, c := range []struct {
			price    float64
			terminal string
			words    string
		}{
			{pathGain, "sell", "smartSell: sell at flat market "},
			{pathBreakEven, "sell", "smartSell: sell at flat market "},
			{pathLoss, "takeLoss", "smartSell: take loss at flat market "},
		} {
			got := Apply(anchoredToSell(committed, c.price), "", c.price, ai)
			assertSoldAs(t, got, reasonFlatMarket, c.terminal)
			if exit := ExitRow(committed, got.Reason, c.price); !strings.HasPrefix(exit.Message, c.words) || exit.Event != EventSold {
				t.Errorf("from %s at %v: the exit row must read %q..., got %+v", state, c.price, c.words, exit)
			}
		}
	}
}

// The other ways a sale is made are named the same way: a self-armed trailingBuy sold on its
// trigger reached, a committed sale sold once its gap stood SaleGapWait, at a gain and at a loss.
func TestEverySaleIsNamedByItsRealisedResult(t *testing.T) {
	flatMarketAlone(t)
	withAllowedLoss(t, anyLoss)
	trade := flatMarketLadder(false)
	ai := flatMarketMarket(trade)
	selfArmed := restingIn(withRows(trade, triggeredBy(trade, ai)), "trailingBuy")
	for price, terminal := range map[float64]string{pathGain: "sell", pathLoss: "takeLoss"} {
		assertSoldAs(t, Apply(anchoredToHold(selfArmed, price), "", price, ai), reasonFlatMarket, terminal)
	}

	withSaleGapTolerances(t, 5)
	withSaleGapWait(t, 5*time.Second)
	gate := pathGates()[1]
	committed := gate.committedAt(t, pathUnder)
	for price, terminal := range map[float64]string{pathGain: "sell", pathLoss: "takeLoss"} {
		anchored := anchoredFor(committed, price, -2*5*trailRow(committed).Tolerance)
		gapped, got := engineStepAt(anchored, "", price, testutil.At("14:00:00"), ai)
		assertTrailHeld(t, got, got.Rows...)
		if !got.Held || len(got.Rows) != 1 || got.Rows[0].Event != EventGapHeld {
			t.Fatalf("at %v: the gapped print must hold with its gap row, got %+v", price, got)
		}
		waited := ApplyAt(gapped, "", price, ai, testutil.At("14:00:05"))
		assertSoldAs(t, waited, reasonFlatMarket, terminal)
	}
}

// saleTerminal reads the realised result alone: a ladder that used nothing has no result to book
// and ends in takeLoss, as the sale of nothing used always did.
func TestSaleTerminalOfALadderThatUsedNothingIsTakeLoss(t *testing.T) {
	if got := saleTerminal(testutil.LadderTrade(false), pathGain); got != "takeLoss" {
		t.Fatalf("saleTerminal of nothing used = %q, want takeLoss", got)
	}
	trade := flatMarketLadder(false)
	for price, want := range map[float64]string{pathGain: "sell", pathBreakEven: "sell", pathUnder: "takeLoss"} {
		if got := saleTerminal(trade, price); got != want {
			t.Errorf("saleTerminal at %v = %q, want %q", price, got, want)
		}
	}
}

// The committed sale's chain is one for both terminals: it books the result whatever its sign
// (acceptLoss) and never asks the minimum profit (hasProfit); a fresh slice on every call.
func TestSaleActionsAreOneChainWithNoProfitFloor(t *testing.T) {
	want := []string{"cancelPendingOrder", "acceptLoss", "sell", "updateTrade"}
	got := SaleActions()
	if !slices.Equal(got, want) || slices.Contains(got, "hasProfit") {
		t.Fatalf("SaleActions = %v, want %v and no hasProfit", got, want)
	}
	got[0] = "changed"
	if SaleActions()[0] != want[0] {
		t.Fatal("SaleActions must hand back a fresh slice on every call")
	}
}

// A sale whose chain failed after its sold row was written leaves the ladder back in takeProfit with
// the gate's episode closed: the take profit is the ladder's own again. A second gate activated in
// the same state keeps it committed, and its trail resumes.
func TestASaleRestoredToTakeProfitIsTheLaddersUnlessAnotherGateActivated(t *testing.T) {
	capitalProtectionAlone(t)
	withFlatMarket(t, true)
	withAllowedLoss(t, anyLoss)
	trade := capitalProtectionLadder(false)
	block := flatMarketMarket(trade).SmartSell
	block.CapitalProtectionSmcBearish = true
	ai, fill := withBlock(block), trade.PositionPrice
	flat := []Row{triggeredBy(trade, ai), FlatMarketActivatedRow(fill, 1)}
	sold := ExitRow(trade, reasonFlatMarket, pathGain)

	alone := restingIn(withRows(trade, append(flat, sold)...), "takeProfit")
	if got := Apply(anchoredToHold(alone, pathGain), "sell", pathGain, ai); got.Position != "sell" || got.Reason != "" || got.Activated {
		t.Fatalf("with the episode closed by its sold row the take profit is the ladder's own, got %+v", got)
	}

	capital := []Row{capitalProtectionTriggered(trade), CapitalProtectionActivatedRow(fill, 1)}
	both := restingIn(withRows(trade, append(append(capital, flat...), sold)...), "takeProfit")
	if st := rebuildState(both); !st.committedIn(aggragates.PositionTakeProfit) || st.flatMarketActive {
		t.Fatalf("fixture drifted: capital protection must stand committed in takeProfit, the flat market closed, got %+v", st)
	}
	// The flat market's reading opens a new episode beside the trail, its triggered row alone.
	assertTrailHeld(t, Apply(anchoredToHold(both, pathGain), "sell", pathGain, ai), triggeredBy(trade, ai))
	assertSoldAs(t, Apply(anchoredToSell(both, pathGain), "sell", pathGain, ai), reasonCapitalProtection, "sell")
}
