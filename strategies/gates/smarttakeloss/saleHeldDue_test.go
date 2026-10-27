package smarttakeloss

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// A sale held by a depth bound or by the trend writes its held row once per episode (rowDue); a sale
// held by the profit writes it when the episode has none naming a trigger yet, or the trigger in
// force differs from the newest held row's (profitRowDue), the row carrying that trigger.

// The rules the tests below move the trigger between, read off the dashboards that follow: the
// first rule alone holds on readsA (0.70 %), the second on readsB (-0.15 %), the third on readsC
// (-0.20 %), and none on readsNone (the dashboard served and meeting no rule).
var (
	movingRules = []SellLossTrigger{
		{MinTrendDirection: 2, MinReads: []int{1, 8, 8}, Percentage: 0.7},
		{MinTrendDirection: 3, MinReads: []int{9, 8, 8}, Percentage: -0.15},
		{MinTrendDirection: 3, MinReads: []int{9, 9, 9}, Percentage: -0.2},
	}
	readsA, readsB, readsC, readsNone = []int{1, 8, 8}, []int{9, 8, 8}, []int{9, 9, 9}, []int{0, 0, 0}
)

// heldRowsOf are the held rows among a tick's rows.
func heldRowsOf(rows []Row) []Row {
	var held []Row
	for _, row := range rows {
		if row.Event == EventSaleHeld {
			held = append(held, row)
		}
	}
	return held
}

// heldUnderOf is the trigger the gate's episode was last held under, and saleHeldOf whether its sale was held at all.
func heldUnderOf(st state, gate string) heldTrigger {
	switch gate {
	case GateFlatMarket:
		return st.flatMarketHeldUnder
	case GateFreeFallPrevention:
		return st.freeFallPreventionHeldUnder
	case GateCapitalProtection:
		return st.capitalProtectionHeldUnder
	}
	return st.saleHeldUnder
}

func saleHeldOf(st state, gate string) bool {
	switch gate {
	case GateFlatMarket:
		return st.flatMarketSaleHeld
	case GateFreeFallPrevention:
		return st.freeFallPreventionSaleHeld
	case GateCapitalProtection:
		return st.capitalProtectionSaleHeld
	}
	return st.saleHeld
}

// holdsTick is one engine tick on the dashboard ai at index minutes past the hour, which must come
// back as position with the sale held and exactly these held rows.
func holdsTick(t *testing.T, trade aggragates.Trades, position string, price float64, ai aggragates.AIIndicators, index int, want ...Row) aggragates.Trades {
	t.Helper()
	next, got := engineStep(trade, position, price, testutil.At("13:00:00").Add(time.Duration(index)*time.Minute), ai)
	if got.Position != position || got.Reason != "" || got.Activated {
		t.Fatalf("tick %d: the sale must be held with %q standing, got %+v", index, position, got)
	}
	if held := heldRowsOf(got.Rows); len(held) != len(want) || (len(want) > 0 && !reflect.DeepEqual(held, want)) {
		t.Fatalf("tick %d: held rows = %+v, want exactly %+v", index, held, want)
	}
	return next
}

// profitRowDue is true while no held row names a trigger and when the trigger in force is not the
// one the newest names; the held rows of a bound or the trend stay once per episode (heldRows), and
// capital protection's, read off its own drawdown, never move (profitHeldRows).
func TestProfitRowDue(t *testing.T) {
	for _, tc := range []struct {
		name string
		held heldTrigger
		pct  float64
		want bool
	}{
		{"nothing held", heldTrigger{}, 0.7, true},
		{"nothing held, a trigger at break even", heldTrigger{}, 0, true},
		{"held by a bound or the trend, which name none", heldTrigger{pct: 0, named: false}, 0, true},
		{"held by a bound or the trend, under another trigger", heldTrigger{pct: 0, named: false}, -0.15, true},
		{"held under the trigger in force", heldTrigger{pct: 0.7, named: true}, 0.7, false},
		{"held under another, a loss it takes", heldTrigger{pct: 0.7, named: true}, -0.15, true},
		{"held under a loss, the trigger is a profit again", heldTrigger{pct: -0.15, named: true}, 0.7, true},
		{"held under another loss", heldTrigger{pct: -0.15, named: true}, -0.2, true},
		{"held at break even, the trigger still at break even", heldTrigger{pct: 0, named: true}, 0, false},
		{"held at break even, the trigger moves off it", heldTrigger{pct: 0, named: true}, 0.7, true},
	} {
		if got := (saleTrigger{heldUnder: tc.held}).profitRowDue(tc.pct); got != tc.want {
			t.Errorf("%s: profitRowDue(%v) = %v, want %v", tc.name, tc.pct, got, tc.want)
		}
	}

	const fill, why = 150.48, "why"
	named := heldTrigger{pct: 0.7, named: true}
	slow := func(rowDue bool, under heldTrigger) saleTrigger {
		return saleTrigger{gate: GateSlowDecline, rowDue: rowDue, heldUnder: under}
	}
	bound := heldRows([]saleTrigger{slow(true, heldTrigger{}), slow(false, heldTrigger{}), slow(false, named), slow(true, named)}, fill, why)
	if want := []Row{saleHeldRow(GateSlowDecline, fill, why, nil), saleHeldRow(GateSlowDecline, fill, why, nil)}; !reflect.DeepEqual(bound, want) {
		t.Errorf("heldRows writes the rows no held row stands for, whatever trigger the newest names: got %+v, want %+v", bound, want)
	}

	shared := sharedTrigger{pct: 0.7, on: true}
	capital := saleTrigger{gate: GateCapitalProtection, terms: capitalProtectionTerms}
	drawdown := heldTrigger{pct: 0 - capitalProtectionMaxDrawdown, named: true}
	profit := profitHeldRows([]saleTrigger{
		{gate: GateSlowDecline, heldUnder: named},
		{gate: GateFlatMarket, heldUnder: heldTrigger{pct: -0.15, named: true}},
		{gate: GateFreeFallPrevention, rowDue: true},
		{gate: GateSlowDecline, rowDue: true, heldUnder: named},
	}, fill, -3.1, shared)
	want := []Row{
		saleHeldByProfitRow(GateFlatMarket, fill, -3.1, 0.7, nil),
		saleHeldByProfitRow(GateFreeFallPrevention, fill, -3.1, 0.7, nil),
	}
	if !reflect.DeepEqual(profit, want) {
		t.Errorf("profitHeldRows writes a row for the gates not held under the trigger in force: got %+v, want %+v", profit, want)
	}
	capital.heldUnder = drawdown
	if rows := profitHeldRows([]saleTrigger{capital}, fill, -3.1, shared); len(rows) != 0 {
		t.Errorf("capital protection's trigger is its drawdown, never the dashboard's: got %+v", rows)
	}
	capital.heldUnder = heldTrigger{}
	if rows := profitHeldRows([]saleTrigger{capital}, fill, -3.1, shared); !reflect.DeepEqual(rows, []Row{CapitalProtectionSaleHeldRow(fill, -3.1)}) || *rows[0].Trigger != 0-capitalProtectionMaxDrawdown {
		t.Errorf("capital protection's first row names its drawdown, got %+v", rows)
	}
}

// A sale the profit holds writes its held row again whenever the trigger in force moves to another
// rule's and not while it stays: on the dashboards A, A, B, C, B, none, C the rows come out at A, B,
// C, B and C, each carrying its trigger in the stored event, and the fold keeps the newest. A tick
// on a dashboard meeting no rule writes nothing (the episode's held row stands), and the take
// profit the ladder proposes beside it makes no difference.
func TestAHeldRowIsWrittenAgainWhenTheTriggerMoves(t *testing.T) {
	sequence := []struct {
		dashboard []int
		trigger   float64
		row       bool
	}{
		{readsA, 0.7, true}, {readsA, 0.7, false}, {readsB, -0.15, true}, {readsC, -0.2, true},
		{readsB, -0.15, true}, {readsNone, 0, false}, {readsC, -0.2, true},
	}
	for _, gate := range saleGates {
		if !gate.bounded {
			continue
		}
		for _, position := range []string{"", "takeProfit"} {
			t.Run(fmt.Sprintf("%s, proposing %q", gate.name, position), func(t *testing.T) {
				c := gate.build(t)
				withTable(t, movingRules)
				price := ladder.AverageEntryPrice(c.trade) * lossFactor
				short := profitAt(c.trade, price)
				trade := c.trade
				var triggers []float64
				for index, step := range sequence {
					var want []Row
					if step.row {
						want = append(want, saleHeldByProfitRow(c.gate, c.fill, short, step.trigger, c.features))
						triggers = append(triggers, step.trigger)
					}
					trade = holdsTick(t, trade, position, price, c.reads(step.dashboard), index, want...)
				}

				var stored []float64
				for _, event := range trade.StrategyEventsOf(aggragates.StrategyParamSmartTakeLoss, c.gate) {
					var data EventData
					if err := event.DecodeData(&data); err != nil {
						t.Fatalf("event %+v does not decode: %v", event, err)
					}
					if data.Event == EventSaleHeld {
						if data.Trigger == nil {
							t.Fatalf("a held row by the profit must store its trigger, got %s", event.Data)
						}
						stored = append(stored, *data.Trigger)
					}
				}
				if !reflect.DeepEqual(stored, triggers) {
					t.Errorf("stored triggers = %v, want %v", stored, triggers)
				}
				if st := rebuildState(trade); heldUnderOf(st, c.gate) != (heldTrigger{pct: -0.2, named: true}) || !saleHeldOf(st, c.gate) {
					t.Errorf("the fold must keep the newest held row's trigger, got %+v", heldUnderOf(st, c.gate))
				}
			})
		}
	}
}

// The held row of a bound or the trend names no trigger: once the bound lifts or the dashboard
// meets a rule, the sale held by the profit is followed by exactly one held row naming the trigger
// (and a second once the trigger moves), never by one per tick. The bound or the trend itself
// writes its row once per episode, whatever the dashboard does after.
func TestATrendOrBoundHeldRowIsFollowedByOneProfitHeldRow(t *testing.T) {
	for _, gate := range saleGates {
		if !gate.bounded {
			continue
		}
		t.Run(gate.name+", the trend", func(t *testing.T) {
			c := gate.build(t)
			withTable(t, movingRules)
			opened := c.opened()
			price := ladder.AverageEntryPrice(c.trade) * lossFactor
			short := profitAt(c.trade, price)
			byTrend := saleHeldRow(c.gate, c.fill, heldByTrendOf(c.reads(readsNone).SmartTakeLoss, false), c.features)
			if byTrend.Trigger != nil {
				t.Fatalf("the trend's row names no trigger, got %+v", byTrend)
			}

			trade := holdsTick(t, opened, "", price, c.reads(readsNone), 0, byTrend)
			trade = holdsTick(t, trade, "", price, c.reads(readsNone), 1)
			trade = holdsTick(t, trade, "", price, c.reads(readsA), 2, saleHeldByProfitRow(c.gate, c.fill, short, 0.7, c.features))
			trade = holdsTick(t, trade, "", price, c.reads(readsA), 3)
			trade = holdsTick(t, trade, "", price, c.reads(readsNone), 4)
			holdsTick(t, trade, "", price, c.reads(readsB), 5, saleHeldByProfitRow(c.gate, c.fill, short, -0.15, c.features))
		})
		t.Run(gate.name+", a depth bound", func(t *testing.T) {
			c := gate.build(t)
			withTable(t, movingRules)
			opened := c.opened()
			price := ladder.AverageEntryPrice(c.trade) * lossFactor
			short := profitAt(c.trade, price)
			ceiling := ladder.ConfiguredDepths(c.trade)
			withSellLossIgnoreLastDepths(t, ceiling)
			byDepth := saleHeldRow(c.gate, c.fill, heldAtDepth(len(rebuildState(opened).fills), ceiling), c.features)

			trade := holdsTick(t, opened, "", price, c.reads(readsA), 0, byDepth)
			trade = holdsTick(t, trade, "", price, c.reads(readsA), 1)
			trade = holdsTick(t, trade, "", price, c.reads(readsB), 2)
			withSellLossIgnoreLastDepths(t, 0)
			trade = holdsTick(t, trade, "", price, c.reads(readsB), 3, saleHeldByProfitRow(c.gate, c.fill, short, -0.15, c.features))
			holdsTick(t, trade, "", price, c.reads(readsB), 4)
		})
	}
}

// A held event stored before the held row named its trigger (no "trigger" in its document) reads as
// held under none: the next tick the sale is held by the profit writes exactly one held row naming
// the trigger, and none after it while the trigger stays.
func TestALegacyHeldEventIsFollowedByOneHeldRowNamingItsTrigger(t *testing.T) {
	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			document, err := json.Marshal(EventData{Event: EventSaleHeld, Price: c.fill, Features: c.features})
			if err != nil {
				t.Fatal(err)
			}
			legacy := withEvents(c.opened(), eventOf(c.gate, string(document)))
			if st := rebuildState(legacy); !saleHeldOf(st, c.gate) || heldUnderOf(st, c.gate) != (heldTrigger{}) {
				t.Fatalf("fixture drifted: the legacy event must hold the sale and name no trigger, got %+v", st)
			}

			for _, position := range []string{"", "takeProfit"} {
				got := Apply(legacy, position, c.over, c.ai)
				assertWaits(t, got, position, c.held(c.over))
				if got.Rows[0].Trigger == nil {
					t.Fatalf("%q: the row must name its trigger, got %+v", position, got.Rows[0])
				}
				written := withRows(legacy, got.Rows...)
				for _, price := range []float64{c.over, c.over - 1} {
					assertUntouched(t, Apply(written, position, price, c.ai), position)
				}
			}
		})
	}
}

// Capital protection's trigger is its drawdown, whatever the dashboard meets: its held row goes out
// once per episode across dashboards that meet different rules, and again only when the drawdown
// itself moves.
func TestCapitalProtectionsHeldRowStaysOncePerEpisode(t *testing.T) {
	c := saleGates[3].build(t)
	if c.gate != GateCapitalProtection {
		t.Fatalf("fixture drifted: the fourth gate must be capital protection, got %q", c.gate)
	}
	withTable(t, movingRules)
	price := c.over
	short := profitAt(c.trade, price)
	drawdown := 0 - capitalProtectionMaxDrawdown

	trade := holdsTick(t, c.trade, "", price, c.reads(readsA), 0, saleHeldByProfitRow(GateCapitalProtection, c.fill, short, drawdown, nil))
	for index, reads := range [][]int{readsA, readsB, readsC, readsNone, readsB, readsA} {
		trade = holdsTick(t, trade, "", price, c.reads(reads), index+1)
	}
	trade = holdsTick(t, trade, "takeProfit", price, c.reads(readsC), 8)
	if st := rebuildState(trade); st.capitalProtectionHeldUnder != (heldTrigger{pct: drawdown, named: true}) {
		t.Fatalf("the episode must stand held under the drawdown, got %+v", st.capitalProtectionHeldUnder)
	}

	withCapitalProtectionMaxDrawdown(t, capitalProtectionMaxDrawdown/2)
	holdsTick(t, trade, "", price, c.reads(readsA), 9, saleHeldByProfitRow(GateCapitalProtection, c.fill, short, drawdown/2, nil))
}

// The held event names its trigger as a number, a zero trigger included ("trigger":0), after the
// features; the rows of a bound or the trend carry none. The fold reads it back per gate, and a
// reset, a sale, a reopened episode and a ladder going pending again or not pending forget it.
func TestTheHeldEventNamesItsTriggerAndTheFoldReadsIt(t *testing.T) {
	trade := testutil.LadderTrade(false)
	at := testutil.At("13:00:00")
	unread := withTrendCounts(aggragates.SmartTakeLossIndicators{}, 0, 0)
	for name, tc := range map[string]struct {
		row  Row
		want string
	}{
		"a trigger under zero":         {saleHeldByProfitRow(GateFlatMarket, 150.48, -0.3, -0.15, nil), `{"event":"saleHeld","price":150.48,"trigger":-0.15}`},
		"a trigger over zero":          {saleHeldByProfitRow(GateFreeFallPrevention, 150.48, -3.1, 0.7, nil), `{"event":"saleHeld","price":150.48,"trigger":0.7}`},
		"a zero trigger is written":    {saleHeldByProfitRow(GateCapitalProtection, 150.48, -3.1, 0, nil), `{"event":"saleHeld","price":150.48,"trigger":0}`},
		"the features come first":      {saleHeldByProfitRow(GateSlowDecline, 150.48, -3.1, -0.2, []string{FeatureQuietDecline}), `{"event":"saleHeld","price":150.48,"features":["quietDecline"],"trigger":-0.2}`},
		"a depth bound names none":     {SaleHeldAtDepthRow(150.48, 7, 8, FeatureQuietDecline), `{"event":"saleHeld","price":150.48,"features":["quietDecline"]}`},
		"the trend names none":         {FlatMarketSaleHeldByTrendRow(150.48, unread, false), `{"event":"saleHeld","price":150.48}`},
		"an activation names none":     {StopLossActivatedRow(150.48, -0.1, FeatureQuietDecline), `{"event":"activated","price":150.48,"features":["quietDecline"]}`},
		"the slow decline's builder":   {SaleHeldRow(150.48, -3.1, FeatureQuietDecline), `{"event":"saleHeld","price":150.48,"features":["quietDecline"],"trigger":` + strconv.FormatFloat(FullTrendTrigger(), 'f', -1, 64) + `}`},
		"the capital protection one":   {CapitalProtectionSaleHeldRow(150.48, -3.1), fmt.Sprintf(`{"event":"saleHeld","price":150.48,"trigger":%v}`, 0-capitalProtectionMaxDrawdown)},
		"the free fall prevention one": {FreeFallPreventionSaleHeldRow(150.48, -3.1), `{"event":"saleHeld","price":150.48,"trigger":` + strconv.FormatFloat(FullTrendTrigger(), 'f', -1, 64) + `}`},
	} {
		_, event := Rows(trade, tc.row, at)
		if string(event.Data) != tc.want {
			t.Errorf("%s: document %s, want %s", name, event.Data, tc.want)
		}
		var data EventData
		if err := json.Unmarshal(event.Data, &data); err != nil || (data.Trigger == nil) != (tc.row.Trigger == nil) || (data.Trigger != nil && *data.Trigger != *tc.row.Trigger) {
			t.Errorf("%s: the document decodes to %+v (%v), want the row's trigger %v", name, data, err, tc.row.Trigger)
		}
	}

	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			opened := c.opened()
			document := func(kind string, trigger *float64) string {
				raw, err := json.Marshal(EventData{Event: kind, Price: c.fill, Features: c.features, Trigger: trigger})
				if err != nil {
					t.Fatal(err)
				}
				return string(raw)
			}
			under := func(trigger float64) *float64 { return &trigger }
			held := func(trigger *float64) state {
				return rebuildState(withEvents(opened, eventOf(c.gate, document(EventSaleHeld, trigger))))
			}

			for name, tc := range map[string]struct {
				trigger *float64
				want    heldTrigger
			}{
				"under a loss":           {under(-0.15), heldTrigger{pct: -0.15, named: true}},
				"under a profit":         {under(0.7), heldTrigger{pct: 0.7, named: true}},
				"under break even":       {under(0), heldTrigger{pct: 0, named: true}},
				"under none (a bound's)": {nil, heldTrigger{}},
			} {
				if st := held(tc.trigger); !saleHeldOf(st, c.gate) || heldUnderOf(st, c.gate) != tc.want {
					t.Errorf("%s: held %v under %+v, want held under %+v", name, saleHeldOf(st, c.gate), heldUnderOf(st, c.gate), tc.want)
				}
			}

			forgetting := map[string]string{
				"a sale":  document(EventSold, nil),
				"a reset": document(EventReset, nil),
			}
			if c.gate == GateSlowDecline {
				forgetting["the ladder going pending again"] = document(EventPending, nil)
				forgetting["the ladder cancelled"] = document(EventCancelled, nil)
			} else {
				forgetting["the episode reopened"] = document(EventTriggered, nil)
			}
			for name, forget := range forgetting {
				st := rebuildState(withEvents(opened, eventOf(c.gate, document(EventSaleHeld, under(-0.15))), eventOf(c.gate, forget)))
				if saleHeldOf(st, c.gate) || heldUnderOf(st, c.gate) != (heldTrigger{}) {
					t.Errorf("%s: the held row's trigger must be forgotten, got held %v under %+v", name, saleHeldOf(st, c.gate), heldUnderOf(st, c.gate))
				}
			}
		})
	}
}
