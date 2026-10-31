package smartsell

import (
	"reflect"
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/freefallprevention"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// freeFallHold is the hold the gate answers for a first fill: the constant reason, filed under the
// gate, and the held event naming the reasons the reading names.
func freeFallHold(reasons []string) gates.Hold {
	return gates.Hold{
		Reason: FreeFallPreventionEntryHoldReason,
		Key:    aggragates.LogKeyHoldSmartSellEntry,
		Param:  aggragates.StrategyParamSmartSell,
		Gate:   GateFreeFallPrevention,
		Data:   EventData{Event: gates.EventHeld, Reasons: reasons},
	}
}

// The first fill is held while the reading stands, long and inverse (aggragates.SideShort, free
// rise), on the last closed bar or on the window's share, the held event carrying the reasons; the
// row text is one constant, the rows riding in the event alone. The gate's own switch is not asked.
func TestFreeFallPreventionEntryHoldHoldsAFirstFillWhileItsReadingStands(t *testing.T) {
	long, inverse := testutil.LadderTrade(false), testutil.LadderTrade(true)
	bars := freefallprevention.Window
	if FreeFallPreventionEntryHoldReason != "smartSell: free fall prevention, first fill held" {
		t.Fatalf("the reason is byte-stable text, got %q", FreeFallPreventionEntryHoldReason)
	}

	for _, enabled := range []bool{false, true} {
		withFreeFallPrevention(t, enabled)
		if got, want := EntryHold(long, aggragates.SideLong, freeFallMarket(long)), freeFallHold(freeFallLastBar); !got.Held() || !reflect.DeepEqual(got, want) {
			t.Fatalf("enabled %v: a long first fill on a free fall must be held, got %+v want %+v", enabled, got, want)
		}
		want := freeFallHold([]string{"1h in free rise on the last closed bar"})
		if got := EntryHold(inverse, aggragates.SideShort, freeFallMarket(inverse)); !got.Held() || !reflect.DeepEqual(got, want) {
			t.Fatalf("enabled %v: an inverse first fill must be held on the free rise, got %+v want %+v", enabled, got, want)
		}
	}

	share := EntryHold(long, aggragates.SideLong, withBlock(freeFallSeries(aggragates.SideLong, bars)))
	wantShare := freeFallHold([]string{"1h in free fall on 240 of 240 1h bars, over 80%"})
	if bars == 240 && freefallprevention.MinShare == 0.8 && freefallprevention.Timeframe == "1h" {
		if !share.Held() || !reflect.DeepEqual(share, wantShare) {
			t.Errorf("a long first fill on the window's share must be held, got %+v want %+v", share, wantShare)
		}
	} else if !share.Held() || share.Gate != GateFreeFallPrevention || len(share.Data.(EventData).Reasons) != 1 {
		t.Errorf("a long first fill on the window's share must be held on one reason, got %+v", share)
	}
	inverseShare := EntryHold(inverse, aggragates.SideShort, withBlock(freeFallSeries(aggragates.SideShort, bars)))
	if !inverseShare.Held() || inverseShare.Reason != FreeFallPreventionEntryHoldReason || inverseShare.Gate != GateFreeFallPrevention {
		t.Errorf("an inverse first fill on the free rise masks must be held, got %+v", inverseShare)
	}
	if got := EntryHold(long, aggragates.SideLong, withBlock(freeFallSeries(aggragates.SideShort, bars))); got.Held() {
		t.Errorf("a long first fill reads the free fall masks, not the free rise ones, got %+v", got)
	}
}

// The hold reads the side it is handed, the entry's side, not the trade's own flag: a long entry
// asks the free fall and an inverse one the free rise whichever trade carries them.
func TestFreeFallPreventionEntryHoldReadsTheSideItIsHanded(t *testing.T) {
	trade := testutil.LadderTrade(false)
	falling, rising := freeFallMarket(testutil.LadderTrade(false)), freeFallMarket(testutil.LadderTrade(true))
	if got := EntryHold(trade, aggragates.SideShort, rising); !got.Held() || got.Gate != GateFreeFallPrevention {
		t.Errorf("the side handed is short: the free rise holds, got %+v", got)
	}
	if got := EntryHold(trade, aggragates.SideShort, falling); got.Held() {
		t.Errorf("the side handed is short: the free fall holds nothing, got %+v", got)
	}
	if got := EntryHold(trade, aggragates.SideLong, rising); got.Held() {
		t.Errorf("the side handed is long: the free rise holds nothing, got %+v", got)
	}
}

// The first fill goes through when the gate has nothing to say: an unread side, futures, a child, no
// flag, a reading that misses, no row asked, the hold's switch off; the gate's own switch off leaves
// the hold reading. A hold that is not a hold is the zero Hold, whole.
func TestFreeFallPreventionEntryHoldLetsAFirstFillThroughWithoutItsReading(t *testing.T) {
	trade := testutil.LadderTrade(false)
	ai := freeFallMarket(trade)
	if got := EntryHold(trade, aggragates.SideLong, ai); !got.Held() {
		t.Fatalf("fixture drifted: the reading must hold the first fill, got %+v", got)
	}

	futures, child, unflagged := trade, trade, trade
	futures.Strategy.TradeType = aggragates.Futures
	child.ParentID = 7
	unflagged.Strategy.Params.SmartSell = false
	for name, variant := range map[string]aggragates.Trades{"futures": futures, "child": child, "no flag": unflagged} {
		if got := EntryHold(variant, aggragates.SideLong, freeFallMarket(variant)); got.Held() {
			t.Errorf("%s: must go through, got %+v", name, got)
		}
	}
	for _, side := range []string{"", "flat"} {
		if got := EntryHold(trade, side, ai); got.Held() {
			t.Errorf("side %q is not read, got %+v", side, got)
		}
	}
	misses := freeFallMisses(aggragates.SideLong)
	misses["no dashboard"] = withBlock(aggragates.SmartSellIndicators{})
	for name, miss := range misses {
		if got := EntryHold(trade, aggragates.SideLong, miss); !reflect.DeepEqual(got, gates.Hold{}) {
			t.Errorf("%s: the reading misses, the hold is the zero Hold, got %+v", name, got)
		}
	}

	previousMin, previousTimeframes := freefallprevention.MinFreeFallTimeframes, freefallprevention.FreeFallTimeframes
	t.Cleanup(func() {
		freefallprevention.MinFreeFallTimeframes, freefallprevention.FreeFallTimeframes = previousMin, previousTimeframes
	})
	freefallprevention.MinFreeFallTimeframes = 0
	if got := EntryHold(trade, aggragates.SideLong, ai); got.Held() {
		t.Errorf("no row asked reads nothing, got %+v", got)
	}
	freefallprevention.MinFreeFallTimeframes, freefallprevention.FreeFallTimeframes = previousMin, previousTimeframes

	withFreeFallPreventionHoldInitialDepth(t, false)
	if got := EntryHold(trade, aggragates.SideLong, ai); got.Held() {
		t.Errorf("the hold's switch off, the first fill goes through, got %+v", got)
	}
	withFreeFallPrevention(t, true)
	if got := EntryHold(trade, aggragates.SideLong, ai); got.Held() {
		t.Errorf("the gate on does not turn the hold on, got %+v", got)
	}
	withFreeFallPreventionHoldInitialDepth(t, true)
	withFreeFallPrevention(t, false)
	if got := EntryHold(trade, aggragates.SideLong, ai); got.Reason != FreeFallPreventionEntryHoldReason {
		t.Errorf("the gate off, the hold still reads, got %+v", got)
	}
}

// The hold is released the moment the reading misses and held again when it returns: every call
// reads the block it is handed, nothing is remembered between them.
func TestFreeFallPreventionEntryHoldIsReleasedWhenTheReadingMisses(t *testing.T) {
	trade := testutil.LadderTrade(false)
	held, missed := freeFallMarket(trade), freeFallMisses(aggragates.SideLong)["no row in free fall"]
	for step, ai := range []aggragates.AIIndicators{held, missed, held, missed, missed, held} {
		wantHeld := step == 0 || step == 2 || step == 5
		if got := EntryHold(trade, aggragates.SideLong, ai); got.Held() != wantHeld {
			t.Fatalf("step %d: held %v, want %v (%+v)", step, got.Held(), wantHeld, got)
		}
	}
}

// The hold is one constant whatever the reasons the reading names, so tradelog.SaveHoldLog
// collapses it tick to tick: the reason, the gate and the event's kind are the same on the last bar,
// on the share and on both, and only the event's reasons differ.
func TestFreeFallPreventionEntryHoldIsConstant(t *testing.T) {
	trade := testutil.LadderTrade(false)
	lastBar := EntryHold(trade, aggragates.SideLong, freeFallMarket(trade))
	shareBlock := freeFallSeries(aggragates.SideLong, freefallprevention.Window)
	share := EntryHold(trade, aggragates.SideLong, withBlock(shareBlock))
	bothBlock := freeFallBlockOf(aggragates.SideLong, true, false, false)
	bothBlock.FreeFallOpens, bothBlock.FreeFallMasks, bothBlock.FreeRiseMasks = shareBlock.FreeFallOpens, shareBlock.FreeFallMasks, shareBlock.FreeRiseMasks
	both := EntryHold(trade, aggragates.SideLong, withBlock(bothBlock))
	if len(both.Data.(EventData).Reasons) != 2 {
		t.Fatalf("fixture drifted: the last bar and the share both name a reason, got %+v", both)
	}
	for name, hold := range map[string]gates.Hold{"the share": share, "both": both} {
		if hold.Reason != lastBar.Reason || hold.Param != lastBar.Param || hold.Gate != lastBar.Gate || hold.Data.(EventData).Event != lastBar.Data.(EventData).Event {
			t.Errorf("%s: the hold must differ from the last bar's by its reasons alone, got %+v and %+v", name, hold, lastBar)
		}
	}
	if reflect.DeepEqual(share.Data, lastBar.Data) {
		t.Error("the reasons ride in the event: the share names its own")
	}
}

// The holds come in order, the quiet decline's verdict first, then the flat market's reading, then
// the free fall prevention's: each speaks only when the ones before it do not, and the free fall
// prevention holds on its own when the others are off or miss.
func TestFreeFallPreventionEntryHoldComesAfterTheFlatMarketsHold(t *testing.T) {
	flatMarketAlone(t)
	trade := testutil.LadderTrade(false)
	all := flatMarketMarket(trade)
	all.SmartSell = inFreeFall(all.SmartSell, []bool{true, false, false}, nil)
	if !flatMarketReading(trade, all.SmartSell, all).Holds || !freefallprevention.Trigger(all.SmartSell, aggragates.SideLong).Holds {
		t.Fatal("fixture drifted: both gates must read the market")
	}
	verdict := all
	verdict.SmartSell.QuietDeclineVerdict = true

	withQuietDecline(t, true)
	if got := EntryHold(trade, aggragates.SideLong, verdict); got.Reason != SlowDeclineEntryHoldReason || got.Gate != GateEntryHold {
		t.Errorf("the quiet decline holds first, got %+v", got)
	}
	withQuietDecline(t, false)
	if got := EntryHold(trade, aggragates.SideLong, verdict); got.Reason != FlatMarketEntryHoldReason || got.Gate != GateFlatMarket {
		t.Errorf("with the quiet decline off the flat market holds, got %+v", got)
	}
	if got := EntryHold(trade, aggragates.SideLong, all); got.Reason != FlatMarketEntryHoldReason {
		t.Errorf("both read: the flat market holds before the free fall prevention, got %+v", got)
	}
	withFlatMarketHoldInitialDepth(t, false)
	if got := EntryHold(trade, aggragates.SideLong, all); !reflect.DeepEqual(got, freeFallHold(freeFallLastBar)) {
		t.Errorf("with the flat market's hold off the free fall prevention holds, got %+v", got)
	}
	withFlatMarketHoldInitialDepth(t, true)

	onlyFree := freeFallMarket(trade)
	if got := EntryHold(trade, aggragates.SideLong, onlyFree); !reflect.DeepEqual(got, freeFallHold(freeFallLastBar)) {
		t.Errorf("the flat market missing, the free fall prevention holds, got %+v", got)
	}
	onlyFlat := flatMarketMarket(trade)
	if got := EntryHold(trade, aggragates.SideLong, onlyFlat); got.Reason != FlatMarketEntryHoldReason {
		t.Errorf("the free fall prevention missing, the flat market holds, got %+v", got)
	}
}
