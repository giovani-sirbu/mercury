package smarttakeloss

import (
	"reflect"
	"testing"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/flatmarket"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The episode of a long ladder tick by tick (engineStep): the first holding tick opens it with the
// triggered row and the held row over the cap, the first tick within the cap activates with the
// activation row alone, from stopLoss a fall under the tolerance sells under the gate's reason, and
// the sold row closes the episode.
func TestFlatMarketLineEpisodeOfALongLadder(t *testing.T) {
	flatMarketLineAlone(t)
	start := flatMarketLadder(false)
	ai, fill := flatMarketLineMarket(start), start.PositionPrice
	atTheCap, oneStepLower := flatMarketPrices(t, start)
	held := FlatMarketSaleHeldRow(fill, -handLossPct(start, oneStepLower))
	activation := FlatMarketStopLossActivatedRow(fill, -handLossPct(start, atTheCap))

	trade, got := engineStep(start, "", oneStepLower, testutil.At("13:00:00"), ai)
	assertWaits(t, got, "", flatMarketLineTriggeredBy(start, ai), held)
	if st := rebuildState(trade); !st.flatMarketWatched || !st.flatMarketActive || !st.flatMarketSaleHeld || st.flatMarketActivated || st.flatMarketFrom != fill {
		t.Fatalf("the first tick opens the episode and the cap holds the action, got %+v", st)
	}
	if !Triggered(trade) {
		t.Fatal("an open episode at the newest fill is a gate triggered")
	}
	trade, got = engineStep(trade, "buy", oneStepLower, testutil.At("13:05:00"), flatMarketLineAgainst(start))
	assertUntouched(t, got, "buy")

	trade, got = engineStep(trade, "stopLoss", atTheCap, testutil.At("13:30:00"), ai)
	assertActivated(t, got, activation)
	if st := rebuildState(trade); trade.PositionType != "stopLoss" || !st.flatMarketActivated || st.flatMarketFrom != fill {
		t.Fatalf("the activation leaves the ladder in stopLoss with the episode open, got %q %+v", trade.PositionType, st)
	}

	falls := priceAtMove(trade, atTheCap, -(trailRow(trade).Tolerance + 1))
	sold := Apply(trade, "buy", falls, flatMarketLineAgainst(start))
	assertSold(t, sold, reasonFlatMarket)
	exit := ExitRow(trade, sold.Reason, falls)
	if exit.Gate != GateFlatMarket || exit.Features != nil || exit.Message != "smartTakeLoss: sell at flat market "+helpers.FormatPriceLevel(trade, falls) {
		t.Fatalf("the exit row is filed under the gate with no feature, got %+v", exit)
	}
	closed := withRow(trade, exit, testutil.At("13:45:00"))
	if st := rebuildState(closed); st.flatMarketActive || st.flatMarketSaleHeld || st.flatMarketActivated || st.flatMarketFrom != 0 {
		t.Fatalf("the sold row closes the episode, got %+v", st)
	}
}

// With the reset on, a new depth closes an open episode with the reset row and reads the market
// afresh: a market that misses leaves the ladder watched and quiet, one that holds opens the next
// episode on the same tick; until that tick the episode at the old fill is no gate triggered.
func TestFlatMarketLineResetsAtTheNewDepth(t *testing.T) {
	flatMarketLineAlone(t)
	withFlatMarketResetOnNewDepth(t, true)
	start := flatMarketLadder(false)
	open := withRows(start, flatMarketLineTriggeredBy(start, flatMarketLineMarket(start)))
	deeper := deeperBy(open, 97)
	_, oneStepLower := flatMarketPrices(t, deeper)
	if Triggered(deeper) {
		t.Fatal("an episode at the old fill is no gate triggered while the reset is due")
	}

	got := Apply(withRestingOrder(deeper), "buy", oneStepLower, flatMarketLineAgainst(deeper))
	assertWaits(t, got, "buy", FlatMarketResetRow(97))
	if st := rebuildState(withRows(deeper, got.Rows...)); st.flatMarketActive || !st.flatMarketWatched {
		t.Fatalf("the reset closes the episode and keeps the watch, got %+v", st)
	}

	again := flatMarketLineMarket(deeper)
	got = Apply(withRestingOrder(deeper), "buy", oneStepLower, again)
	assertWaits(t, got, "buy", FlatMarketResetRow(97), flatMarketLineTriggeredBy(deeper, again))
}

// With the reset off, a new depth leaves an open episode standing: no reset row, no fresh reading,
// the trigger stands for the cooldown (Triggered) and the shared sale acts on at the new fill.
func TestFlatMarketLineEpisodeStandsAtTheNewDepthWithTheResetOff(t *testing.T) {
	flatMarketLineAlone(t)
	withFlatMarketResetOnNewDepth(t, false)
	start := flatMarketLadder(false)
	open := withRows(start, flatMarketLineTriggeredBy(start, flatMarketLineMarket(start)))
	deeper := deeperBy(open, 97)
	_, oneStepLower := flatMarketPrices(t, deeper)

	assertUntouched(t, Apply(withRestingOrder(deeper), "buy", oneStepLower, flatMarketLineAgainst(deeper)), "buy")
	if st := rebuildState(deeper); !st.flatMarketActive || st.flatMarketFrom != start.PositionPrice || !Triggered(deeper) {
		t.Fatalf("the episode stands at its own fill and the gate reads triggered, got %+v", st)
	}
	got := Apply(deeper, "buy", oneStepLower, flatMarketLineAgainst(deeper))
	assertWaits(t, got, "buy", FlatMarketSaleHeldRow(97, -handLossPct(deeper, oneStepLower)))
}

// The trigger latches the early take profit on a watched long ladder with its own row, before
// the gate's triggered row; the latch switch alone drops the latched row, the gate's own switch
// alone drops the episode and its row, the latch reading on.
func TestFlatMarketLineTriggerLatchesTheEarlyTakeProfit(t *testing.T) {
	flatMarketLineAlone(t)
	withEarlyTakeProfit(t, true)
	start := flatMarketLadder(false)
	ai := flatMarketLineMarket(start)

	got := applyAtAGain(start, "buy", ai)
	assertWaits(t, got, "buy", flatMarketLineLatchedBy(start, ai), flatMarketLineTriggeredBy(start, ai))
	if st := rebuildState(withRows(start, got.Rows...)); !st.earlyTakeProfit || !st.flatMarketActive {
		t.Fatalf("the tick latches the ladder and opens the episode, got %+v", st)
	}

	t.Run("latch off", func(t *testing.T) {
		withEarlyTakeProfitFlatMarket(t, false)
		got := applyAtAGain(start, "buy", ai)
		assertWaits(t, got, "buy", flatMarketLineTriggeredBy(start, ai))
		if rebuildState(withRows(start, got.Rows...)).earlyTakeProfit {
			t.Fatal("the latch switch off latches nothing")
		}
	})
	t.Run("gate off", func(t *testing.T) {
		withFlatMarket(t, false)
		got := applyAtAGain(start, "buy", ai)
		assertWaits(t, got, "buy", flatMarketLineLatchedBy(start, ai))
		_, oneStepLower := flatMarketPrices(t, start)
		assertUntouched(t, Apply(withRows(start, got.Rows...), "buy", oneStepLower, ai), "buy")
	})
}

// The gate watches a spot ladder from flatmarket.ArmDepth fills and nothing else: a
// shallower ladder, a futures trade, a child, a trade without the flag or settings get no row and
// arm nothing; an inverse ladder reads the rise and the trend up.
func TestFlatMarketLineWatch(t *testing.T) {
	flatMarketLineAlone(t)
	base := flatMarketLadder(false)
	offTheWatch := flatMarketOffTheWatch(base)
	if flatmarket.ArmDepth >= 2 {
		shallow := withFirstFill(testutil.LadderTrade(false))
		for range flatmarket.ArmDepth - 2 {
			shallow = deeperBy(shallow, 99)
		}
		offTheWatch["under the arm depth"] = shallow
	}
	for name, trade := range offTheWatch {
		if name == "no fill" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			if name != "no settings" && Armed(trade) {
				t.Fatal("an unwatched ladder is not armed")
			}
			assertUntouched(t, Apply(trade, "buy", 90, flatMarketLineMarket(trade)), "buy")
		})
	}
	if !Armed(base) {
		t.Fatal("the fixture ladder is armed by the gate alone")
	}

	inverse := flatMarketLadder(true)
	rising := flatMarketLineMarket(inverse)
	if reading := flatMarketReading(inverse, rising.SmartTakeLoss, rising); !reading.Holds || reading.Line.Side != aggragates.SideShort {
		t.Fatalf("an inverse ladder reads a flat line under the trend up, got %+v", reading)
	}
	if reading := flatMarketReading(inverse, flatMarketLineAgainst(inverse).SmartTakeLoss, rising); reading.Holds {
		t.Fatalf("a line down misses for an inverse ladder, got %+v", reading)
	}
}

// withFirstFill is a fresh ladder with its first fill at its position price.
func withFirstFill(trade aggragates.Trades) aggragates.Trades {
	trade.History = append(trade.History, aggragates.TradesHistory{Type: "BUY", Quantity: exactUnits, Price: 100, OrderId: 1, CreatedAt: testutil.At("09:00:00")})
	trade.PositionPrice = 100
	return trade
}

// The first fill of a pair is held while the reading holds and HoldInitialDepth is on, under the
// gate with the reading's reasons; the switch off or a market that misses hold nothing, and the
// gate's own switch off leaves the hold reading.
func TestFlatMarketLineEntryHold(t *testing.T) {
	flatMarketLineAlone(t)
	fresh := testutil.LadderTrade(false)
	ai := flatMarketLineMarket(fresh)
	want := gates.Hold{
		Reason: FlatMarketEntryHoldReason,
		Param:  aggragates.StrategyParamSmartTakeLoss,
		Gate:   GateFlatMarket,
		Data:   EventData{Event: gates.EventHeld, Reasons: flatMarketReading(fresh, ai.SmartTakeLoss, ai).Reasons},
	}
	if got := EntryHold(fresh, aggragates.SideLong, ai); !reflect.DeepEqual(got, want) {
		t.Fatalf("EntryHold = %+v, want %+v", got, want)
	}
	if got := EntryHold(fresh, aggragates.SideLong, flatMarketLineAgainst(fresh)); got.Held() {
		t.Fatalf("a market that misses holds nothing, got %+v", got)
	}
	inverse := testutil.LadderTrade(true)
	if got := EntryHold(inverse, aggragates.SideShort, flatMarketLineMarket(inverse)); !got.Held() || got.Gate != GateFlatMarket {
		t.Fatalf("an inverse entry is held on a flat line under the trend up, got %+v", got)
	}
	t.Run("hold off", func(t *testing.T) {
		withFlatMarketHoldInitialDepth(t, false)
		if got := EntryHold(fresh, aggragates.SideLong, ai); got.Held() {
			t.Fatalf("HoldInitialDepth off holds nothing, got %+v", got)
		}
	})
	t.Run("gate off", func(t *testing.T) {
		withFlatMarket(t, false)
		if got := EntryHold(fresh, aggragates.SideLong, ai); !reflect.DeepEqual(got, want) {
			t.Fatalf("the gate off, the hold still reads, got %+v", got)
		}
	})
}

// The fold reads the gate's events in order: triggered opens, sale held and activated keep the
// episode open and remember themselves, reset and sold close it and clear both.
func TestFlatMarketLineFold(t *testing.T) {
	for name, tc := range map[string]struct {
		events                      []EventData
		active, saleHeld, activated bool
		from                        float64
	}{
		"triggered":         {[]EventData{{Event: EventTriggered, Price: 98}}, true, false, false, 98},
		"held then active":  {[]EventData{{Event: EventTriggered, Price: 98}, {Event: EventSaleHeld, Price: 98}, {Event: EventActivated, Price: 98}}, true, true, true, 98},
		"reset":             {[]EventData{{Event: EventTriggered, Price: 98}, {Event: EventReset, Price: 97}}, false, false, false, 0},
		"sold":              {[]EventData{{Event: EventTriggered, Price: 98}, {Event: EventActivated, Price: 98}, {Event: EventSold, Price: 95}}, false, false, false, 0},
		"held with no open": {[]EventData{{Event: EventSaleHeld, Price: 98}}, true, true, false, 98},
	} {
		t.Run(name, func(t *testing.T) {
			var st state
			for _, data := range tc.events {
				st = st.foldFlatMarket(data)
			}
			if st.flatMarketActive != tc.active || st.flatMarketSaleHeld != tc.saleHeld || st.flatMarketActivated != tc.activated || st.flatMarketFrom != tc.from {
				t.Fatalf("got %+v, want active %v held %v activated %v from %v", st, tc.active, tc.saleHeld, tc.activated, tc.from)
			}
		})
	}
}
