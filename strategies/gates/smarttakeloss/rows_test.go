package smarttakeloss

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/quietdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The gate, kind and feature names and the keys of the document are what the
// events store, and what every reader of the table finds them by: they must not
// move. A document that carries no features — every one written before the merge,
// the sold event of capital protection and every event of the flat market gate —
// is written byte for byte as the table below pins it.
func TestTheEventNamesAreTheStoredContract(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{GateSlowDecline, "slowDecline"},
		{GateCapitalProtection, "capitalProtection"},
		{GateEntryHold, "entryHold"},
		{GateFlatMarket, "flatMarket"},
		{gateLowInterest, "lowInterest"},         // legacy: the flat market's range as a gate of its own, folded still
		{gateNonInterestArea, "nonInterestArea"}, // legacy: its line as a gate of its own, folded still
		{gateIndecision, "indecision"},
		{gateSlowPattern, "slowPattern"},
		{EventPending, "pending"},
		{EventCancelled, "cancelled"},
		{EventReset, "reset"},
		{EventSold, "sold"},
		{EventLatched, "latched"},
		{EventSaleHeld, "saleHeld"},
		{EventActivated, "activated"},
		{EventReached, "reached"},
		{EventGapHeld, "gapHeld"},
		{EventGapClosed, "gapClosed"},
		{EventTriggered, "triggered"},
		{EventEnded, "ended"}, // legacy: stored by earlier runs and folded still, written by no engine
		{FeatureIndecision, "indecision"},
		{FeatureQuietDecline, "quietDecline"},
		{FeatureSwingDecline, "swingDecline"},
		{FeatureLinearDecline, "linearDecline"}, // names a finding, never a stored event
		{FeatureFlatMarket, "flatMarket"},
		{gates.EventHeld, "held"},
		{aggragates.StrategyParamSmartTakeLoss, "smartTakeLoss"},
	} {
		if tc.got != tc.want {
			t.Errorf("the stored name %q must not move, got %q", tc.want, tc.got)
		}
	}

	trigger := func(pct float64) *float64 { return &pct }
	for name, tc := range map[string]struct {
		data EventData
		want string
	}{
		"the trigger a held row by the profit names, last": {EventData{Event: EventSaleHeld, Price: 184.45, Features: []string{FeatureQuietDecline}, Trigger: trigger(-0.15)}, `{"event":"saleHeld","price":184.45,"features":["quietDecline"],"trigger":-0.15}`},
		"a zero trigger is written":                        {EventData{Event: EventSaleHeld, Price: 184.45, Trigger: trigger(0)}, `{"event":"saleHeld","price":184.45,"trigger":0}`},
		"the document as it always was":                    {EventData{Event: EventPending, Price: 184.45, Reasons: []string{"leg down", "leg 60 bars long"}}, `{"event":"pending","price":184.45,"reasons":["leg down","leg 60 bars long"]}`},
		"a document leaves out the absent":                 {EventData{Event: gates.EventHeld}, `{"event":"held"}`},
		"the sale of capital protection":                   {EventData{Event: EventSold, Price: 192.6543}, `{"event":"sold","price":192.6543}`},
		"features after the reasons":                       {EventData{Event: EventPending, Price: 184.45, Reasons: []string{"a"}, Features: []string{FeatureEarlyTakeProfit, FeatureSwingDecline}}, `{"event":"pending","price":184.45,"reasons":["a"],"features":["earlyTakeProfit","swingDecline"]}`},
		"features without reasons":                         {EventData{Event: EventReset, Price: 184.45, Features: []string{FeatureQuietDecline}}, `{"event":"reset","price":184.45,"features":["quietDecline"]}`},
		"a sale held":                                      {EventData{Event: EventSaleHeld, Price: 184.45, Features: []string{FeatureQuietDecline, FeatureSwingDecline}}, `{"event":"saleHeld","price":184.45,"features":["quietDecline","swingDecline"]}`},
		"a stopLoss activated":                             {EventData{Event: EventActivated, Price: 184.45, Features: []string{FeatureQuietDecline, FeatureSwingDecline}}, `{"event":"activated","price":184.45,"features":["quietDecline","swingDecline"]}`},
		"a trigger reached, its trigger last":              {EventData{Event: EventReached, Price: 184.45, Features: []string{FeatureQuietDecline}, Trigger: trigger(-0.15)}, `{"event":"reached","price":184.45,"features":["quietDecline"],"trigger":-0.15}`},
		"a gap held, with no trigger":                      {EventData{Event: EventGapHeld, Price: 184.45, Features: []string{FeatureQuietDecline}}, `{"event":"gapHeld","price":184.45,"features":["quietDecline"]}`},
		"a gap closed, with no trigger":                    {EventData{Event: EventGapClosed, Price: 184.45, Features: []string{FeatureQuietDecline}}, `{"event":"gapClosed","price":184.45,"features":["quietDecline"]}`},
		"a gap closed, with no feature":                    {EventData{Event: EventGapClosed, Price: 184.45}, `{"event":"gapClosed","price":184.45}`},
		"a flat market reading triggered":                  {EventData{Event: EventTriggered, Price: 184.45, Reasons: []string{"a", "b"}}, `{"event":"triggered","price":184.45,"reasons":["a","b"]}`},
		"a legacy flat market ended event":                 {EventData{Event: EventEnded, Price: 184.45}, `{"event":"ended","price":184.45}`},
		"a flat market episode reset":                      {EventData{Event: EventReset, Price: 184.45}, `{"event":"reset","price":184.45}`},
		"a flat market sale held":                          {EventData{Event: EventSaleHeld, Price: 184.45}, `{"event":"saleHeld","price":184.45}`},
		"a flat market stopLoss activated":                 {EventData{Event: EventActivated, Price: 184.45}, `{"event":"activated","price":184.45}`},
		"a flat market hold":                               {EventData{Event: gates.EventHeld, Reasons: []string{"a"}}, `{"event":"held","reasons":["a"]}`},
	} {
		got, _ := json.Marshal(tc.data)
		if string(got) != tc.want {
			t.Errorf("%s: the document's keys must not move, got %s, want %s", name, got, tc.want)
		}
	}
}

// Rows builds the pair to write. The row is INFO, on the trade, at the row's
// own price — never the position price a re-anchor moved — with the engine's
// clock on both stamps and nothing else set. The event is the smartTakeLoss
// event of the row's gate and kind, its document the kind, the price and the
// features, stamped with the same clock: the pair is found by (TradeID,
// CreatedAt). What it writes reads back as the ladder pending from that fill.
func TestRowsBuildAnInfoRowAtTheFillPriceAndTheEventBesideIt(t *testing.T) {
	withQuietDecline(t, true)
	trade := watchedTrade()
	trade.PositionPrice = tickPrice
	at := testutil.At("17:45:00")
	row := PendingRow(slowDeclineLastFill, quietFinding(nil))
	logRow, event := Rows(trade, row, at)

	wantRow := aggragates.TradesLogs{
		TradeID:   trade.ID,
		Message:   "smartTakeLoss: quiet decline triggered",
		Type:      aggragates.LOG_INFO,
		Price:     slowDeclineLastFill,
		CreatedAt: at,
		UpdatedAt: at,
	}
	if !reflect.DeepEqual(logRow, wantRow) {
		t.Fatalf("the row carries the text at the fill on the engine's clock and nothing else, got %+v, want %+v", logRow, wantRow)
	}

	if event.ID != 0 || event.TradeID != trade.ID || event.Param != aggragates.StrategyParamSmartTakeLoss || event.Gate != GateSlowDecline {
		t.Fatalf("the event is the trade's smartTakeLoss event of the row's gate, got %+v", event)
	}
	if event.Kind() != EventPending || !event.CreatedAt.Equal(at) {
		t.Fatalf("the event names the row's kind on the row's stamp, got %+v", event)
	}
	if string(event.Data) != `{"event":"pending","price":184.45,"features":["quietDecline"]}` {
		t.Fatalf("the document is the kind, the fill's price and the feature, got %s", event.Data)
	}
	if event.TradeID != logRow.TradeID || !event.CreatedAt.Equal(logRow.CreatedAt) {
		t.Fatalf("row and event are a pair by (trade, stamp), got %+v and %+v", logRow, event)
	}

	written := aggragates.AppendStrategyRow(trade, logRow, event)
	if st := rebuildState(written); !st.slowDeclinePending || st.slowDeclinePendingFrom != slowDeclineLastFill || !reflect.DeepEqual(st.pendingFeatures, []string{FeatureQuietDecline}) {
		t.Fatalf("what Rows writes must read back as the ladder pending from its fill on the quiet decline, got %+v", st)
	}
}

// The reasons ride in the message after the head and in the document, in the
// order sophos served them, and the row's price stays the fill's whatever the
// reasons say.
func TestRowsCarryTheReasonsInBothForms(t *testing.T) {
	trade := watchedTrade()
	logRow, event := Rows(trade, PendingRow(slowDeclineLastFill, quietFinding(slowDeclineReasons)), testutil.At("17:45:00"))

	if logRow.Message != "smartTakeLoss: quiet decline triggered (quiet decline: leg down 7.0% from its high close, leg 60 bars long on 1h)" {
		t.Fatalf("the reasons follow the head in parentheses under their feature, got %q", logRow.Message)
	}
	if string(event.Data) != `{"event":"pending","price":184.45,"reasons":["leg down 7.0% from its high close","leg 60 bars long on 1h"],"features":["quietDecline"]}` {
		t.Fatalf("the document names them in the same order, got %s", event.Data)
	}
}

// Append is the pair appended, both slices copied: the trade the caller holds
// is never written through, even where it has spare capacity in its backing
// arrays, and the trade handed back carries the pair last.
func TestAppendWritesThePairAndNeverTheCallersArrays(t *testing.T) {
	trade := withRows(watchedTrade(), PendingRow(slowDeclineLastFill, quietFinding(nil)))
	trade.Logs = append(make([]aggragates.TradesLogs, 0, 8), trade.Logs...)
	trade.StrategyEvents = append(make([]aggragates.TradesStrategyEvents, 0, 8), trade.StrategyEvents...)

	at := testutil.At("18:00:00")
	next := Append(trade, CancelledRow(sixthFill, quietFinding(quietDeclineBreakReasons)), at)

	if len(next.Logs) != 2 || len(next.StrategyEvents) != 2 {
		t.Fatalf("the pair joins the trade, got %d rows and %d events", len(next.Logs), len(next.StrategyEvents))
	}
	row, event := next.Logs[1], next.StrategyEvents[1]
	if row.Message != CancelledRow(sixthFill, quietFinding(quietDeclineBreakReasons)).Message || row.Price != sixthFill || !row.CreatedAt.Equal(at) {
		t.Fatalf("the row is the cancelled row on the engine's clock, got %+v", row)
	}
	if event.Gate != GateSlowDecline || event.Kind() != EventCancelled || !event.CreatedAt.Equal(at) || event.TradeID != row.TradeID {
		t.Fatalf("the event is its pair, got %+v", event)
	}

	if len(trade.Logs) != 1 || len(trade.StrategyEvents) != 1 {
		t.Fatalf("the caller's trade is unchanged, got %d rows and %d events", len(trade.Logs), len(trade.StrategyEvents))
	}
	if !reflect.DeepEqual(trade.Logs[:2][1], aggragates.TradesLogs{}) || !reflect.DeepEqual(trade.StrategyEvents[:2][1], aggragates.TradesStrategyEvents{}) {
		t.Fatal("a copy-on-append must never write into the spare capacity of the caller's arrays")
	}
}

// Every row of the gate's own that Apply hands back is filed under its gate and
// kind, whatever the ladder does next, with the features it is about: a ladder
// going pending on the quiet decline, a fill the gate confirms, a fill it cancels,
// a pending ladder a depth priority holds, a ladder the swing decline makes pending
// and latches, a fill the swing decline confirms, a ladder the indecision latches.
// Each is stated here with the literal names that are stored; the reasons are
// the ones the readings serve, and the texts are pinned in
// slowDeclineRow_test.go. The two declines are switched on for it: it reads the
// gate's rows, not the switches the package is tuned to. Every tick runs at a print
// over the ladder's break even (gainPrint) with a buy order resting, where the ladders, resting in buy, are
// activated nothing and the tick writes the gate's row alone: the rows of the shared
// action are filed in the tests after this one.
func TestApplyHandsBackEveryRowFiledUnderItsGateAndKind(t *testing.T) {
	withQuietDecline(t, true)
	withSwingDecline(t, true)
	withEarlyTakeProfit(t, true)
	withDepthPriorityPause(t, true)
	pendingStair := withRow(stairFourth(), stairPendingRow(t), testutil.At("00:30:00"))
	fifthFill := withFill(pendingStair, stairPrices[4], stairOpen(30).Add(17*time.Minute+9*time.Second))
	fourth := stairReading(t, stairFourth(), stairFourthBlock(), true).Reasons
	fifth := stairReading(t, fifthFill, stairFifthBlock(), true).Reasons
	tick := func(trade aggragates.Trades, ai aggragates.AIIndicators) *Row {
		return firstRow(applyAtAGain(trade, "", ai))
	}
	for name, tc := range map[string]struct {
		got                *Row
		gate, event        string
		features, reasons  []string
		price              float64
		messagePrefix, end string
	}{
		"a ladder going pending on the quiet decline": {
			tick(watchedTrade(), slowDeclineBlock(true)), "slowDecline", "pending",
			[]string{"quietDecline"}, slowDeclineReasons, slowDeclineLastFill, "smartTakeLoss: quiet decline triggered (quiet decline: ", ")",
		},
		"a fill the gate confirms": {
			tick(pendingAtFive(), legOnAndQuiet()), "slowDecline", "pending",
			[]string{"quietDecline"}, slowDeclineReasons, sixthFill, "smartTakeLoss: quiet decline confirmed at the new fill (quiet decline: ", ")",
		},
		"a fill the gate cancels": {
			tick(pendingAtFive(), brokenReading()), "slowDecline", "cancelled",
			[]string{"quietDecline"}, quietDeclineBreakReasons, sixthFill, "smartTakeLoss: slow decline cancelled at the new fill (quiet decline: ", ")",
		},
		"a pending ladder a depth priority holds": {
			tick(heldBy(pendingTrade(), testutil.At("18:05:00")), slowDeclineBlock(true)), "slowDecline", "reset",
			[]string{"quietDecline"}, nil, slowDeclineLastFill, "smartTakeLoss: slow decline reset, depth priority holds this ladder", "",
		},
		"a ladder the swing decline makes pending and latches": {
			tick(stairFourth(), withBlock(stairFourthBlock())), "slowDecline", "pending",
			[]string{"earlyTakeProfit", "swingDecline"}, fourth, stairPrices[3], "smartTakeLoss: early take profit latched, swing decline triggered (swing decline: ", ")",
		},
		"a fill the swing decline confirms": {
			tick(fifthFill, withBlock(stairFifthBlock())), "slowDecline", "pending",
			[]string{"swingDecline"}, fifth, stairPrices[4], "smartTakeLoss: swing decline confirmed at the new fill (swing decline: ", ")",
		},
		"a ladder the indecision latches": {
			tick(indecisionLadder(), indecisionReading()), "slowDecline", "latched",
			[]string{"earlyTakeProfit"}, indecisionReasons, indecisionLadder().PositionPrice, "smartTakeLoss: early take profit latched (indecision: ", ")",
		},
	} {
		if tc.got == nil {
			t.Errorf("%s: no row", name)
			continue
		}
		row := *tc.got
		if row.Gate != tc.gate || row.Event != tc.event || row.Price != tc.price || !reflect.DeepEqual(row.Features, tc.features) || !reflect.DeepEqual(row.Reasons, tc.reasons) {
			t.Errorf("%s: filed %q/%q at %v with features %v and reasons %q, want %q/%q at %v, %v, %q", name, row.Gate, row.Event, row.Price, row.Features, row.Reasons, tc.gate, tc.event, tc.price, tc.features, tc.reasons)
		}
		if got := row.Message; len(got) < len(tc.messagePrefix)+len(tc.end) || got[:len(tc.messagePrefix)] != tc.messagePrefix || got[len(got)-len(tc.end):] != tc.end {
			t.Errorf("%s: the text %q does not read %q … %q", name, got, tc.messagePrefix, tc.end)
		}
	}
}

// firstRow is the one row Apply handed back, nil when it handed back none or
// more than one.
func firstRow(got Result) *Row {
	if len(got.Rows) != 1 {
		return nil
	}
	return &got.Rows[0]
}

// The rows of a sale the shared action holds are rows Apply hands back like any
// other, and filed the same way: under the slow decline gate, as the sale held kind,
// at the newest fill and never the price of the tick, naming the decline features
// the ladder is pending on and no reasons. They go out on the first tick of the
// episode the action is asked and refuses, whatever the proposal and whether the ladder
// rests in buy or in stopLoss, and on no later one; in any state the action does nothing
// in they are not written at all. The text names the bound that held it: the loss over the
// shared cap for a ladder within the depth bound, the depth over the deepest depth the
// sale is made at for a ladder one depth past it, whatever the loss. Both bounds are
// switched on at their shipped values, and a bound a retune switches off or leaves
// nothing to hold drops its case.
func TestApplyHandsBackTheHeldRowsFiledUnderTheirGateAndKind(t *testing.T) {
	deepest := lastDepthFills - SellLossIgnoreLastDepths
	withQuietDecline(t, true)
	sellLossOn(t)
	pendingAt := func(depth int) aggragates.Trades {
		trade := depthLadder(depth)
		return withRows(trade, quietPending(trade.PositionPrice))
	}

	type heldCase struct {
		name    string
		trade   aggragates.Trades
		message string
	}
	var cases []heldCase
	within := pendingAt(min(max(deepest, quietdecline.ArmDepth), lastDepthFills))
	if lossPct := handLossPct(within, armingPrice(within)); shippedLoss > 0 && lossPct > shippedLoss && (SellLossIgnoreLastDepths <= 0 || ladder.CountFilledEntries(within) <= deepest) {
		message := "smartTakeLoss: slow decline sale held, " + byLossText(lossPct, float64(shippedLoss))
		cases = append(cases, heldCase{"over the cap", within, message})
	}
	if SellLossIgnoreLastDepths > 0 && deepest >= quietdecline.ArmDepth {
		message := fmt.Sprintf("smartTakeLoss: slow decline sale held, depth %d of %d over %d", deepest+1, lastDepthFills, deepest)
		cases = append(cases, heldCase{"past the depth bound", pendingAt(deepest + 1), message})
	}
	if len(cases) == 0 {
		t.Skip("SellLossTriggerPercentage and SellLossIgnoreLastDepths leave no sale to be held")
	}
	nothing := withBlock(withEveryRowTrend(aggragates.SmartTakeLossIndicators{}))
	for _, tc := range cases {
		armed := armingPrice(tc.trade)
		for _, state := range []string{"buy", "stopLoss"} {
			resting := restingIn(tc.trade, state)
			for _, position := range addSideProposals {
				got := Apply(resting, position, armed, nothing)
				assertNoSale(t, got, position)
				held := firstRow(got)
				if held == nil {
					t.Errorf("%s, in %s, on %q: no single row: %+v", tc.name, state, position, got.Rows)
					continue
				}
				if held.Gate != "slowDecline" || held.Event != "saleHeld" || held.Price != tc.trade.PositionPrice || !reflect.DeepEqual(held.Features, []string{"quietDecline"}) || held.Reasons != nil {
					t.Errorf("%s, in %s, on %q: filed %q/%q at %v with %v and %v, want slowDecline/saleHeld at the newest fill on the quiet decline and no reasons", tc.name, state, position, held.Gate, held.Event, held.Price, held.Features, held.Reasons)
				}
				if held.Message != tc.message {
					t.Errorf("%s, in %s, on %q:\n got  %q\n want %q", tc.name, state, position, held.Message, tc.message)
				}
			}
			written, _ := engineTick(resting, "", armed, testutil.At("22:00:00"), nothing)
			for _, position := range addSideProposals {
				assertUntouched(t, Apply(written, position, armed, nothing), position)
			}
		}
		for _, state := range inertStates {
			assertUntouched(t, Apply(restingIn(tc.trade, state), "", armed, nothing), "")
		}
	}
}

// The rows of the shared action that activates the stopLoss, and the flat market gate's
// reset at a new depth, are rows Apply hands back like any other, and filed the same way:
// the activation under the gate whose trigger stood, as the activated kind, at the newest
// fill and never the price of the tick, naming the decline features the slow decline
// gate's ladder is pending on and none for the flat market gate's, and no reasons; the
// reset under the flat market gate, as the reset kind, at the new fill, with neither. Rows
// writes each as the pair the folds read back, the document the kind, the fill's price and
// the features. The trigger takes any loss (anyLoss), so the text names the loss as the signed
// profit at or over -100%; the shipped trigger's own wording is pinned with the rows' grammar.
func TestApplyHandsBackTheActivationAndResetRowsFiledUnderTheirGateAndKind(t *testing.T) {
	flatMarketAlone(t)
	withAllowedLoss(t, anyLoss)
	withQuietDecline(t, true)
	slow, low := pendingTrade(), flatMarketLadder(false)
	armed, fill, market := armingPrice(slow), low.PositionPrice, flatMarketMarket(low)
	const deeperFill = 96.0
	deeper := withFill(withRows(low, FlatMarketTriggeredRow(fill, flatMarketReading(low, market.SmartTakeLoss, market).Reasons)), deeperFill, testutil.At("14:00:00"))

	slowRows := Apply(slow, "", armed, nothingServed()).Rows
	lowRows := Apply(low, "", fill, market).Rows
	resetRows := Apply(deeper, "", deeperFill, market).Rows
	if len(slowRows) != 1 || len(lowRows) != 2 || len(resetRows) != 3 {
		t.Fatalf("fixture drifted: the activation alone, the episode's opening and its activation, a reset then a new opening and its activation, got %+v, %+v and %+v", slowRows, lowRows, resetRows)
	}
	anyLossText := func(gate string, trade aggragates.Trades, price float64) string {
		return fmt.Sprintf("smartTakeLoss: %s stopLoss activated, %s", gate, activatedText(handLossPct(trade, price), anyLoss))
	}
	at := testutil.At("21:45:00")
	for name, tc := range map[string]struct {
		trade       aggragates.Trades
		row         Row
		gate, event string
		features    []string
		price       float64
		message     string
		document    string
	}{
		"the slow decline's activation": {
			slow, slowRows[0], GateSlowDecline, EventActivated, []string{FeatureQuietDecline}, slowDeclineLastFill,
			anyLossText("slow decline", slow, armed), `{"event":"activated","price":184.45,"features":["quietDecline"]}`,
		},
		"the flat market gate's activation": {
			low, lowRows[1], GateFlatMarket, EventActivated, nil, fill,
			anyLossText("flat market", low, fill), `{"event":"activated","price":98}`,
		},
		"the flat market gate's reset at a new depth": {
			deeper, resetRows[0], GateFlatMarket, EventReset, nil, deeperFill,
			"smartTakeLoss: flat market reset at the new depth", `{"event":"reset","price":96}`,
		},
		"the activation of the new episode at that depth": {
			deeper, resetRows[2], GateFlatMarket, EventActivated, nil, deeperFill,
			anyLossText("flat market", deeper, deeperFill), `{"event":"activated","price":96}`,
		},
	} {
		row := tc.row
		if row.Message != tc.message || row.Gate != tc.gate || row.Event != tc.event || row.Price != tc.price || !reflect.DeepEqual(row.Features, tc.features) || len(row.Reasons) != 0 {
			t.Errorf("%s: the row is %+v, want %q filed under %q/%q at %v with %v and no reasons", name, row, tc.message, tc.gate, tc.event, tc.price, tc.features)
		}
		logRow, event := Rows(tc.trade, row, at)
		if logRow.Message != tc.message || logRow.Price != tc.price || logRow.Type != aggragates.LOG_INFO || !logRow.CreatedAt.Equal(at) {
			t.Errorf("%s: the log row is %+v, want an INFO row of %q at %v", name, logRow, tc.message, tc.price)
		}
		if event.Param != aggragates.StrategyParamSmartTakeLoss || event.Gate != tc.gate || event.Kind() != tc.event || !event.CreatedAt.Equal(at) || string(event.Data) != tc.document {
			t.Errorf("%s: the event is %q/%q/%q with %s, want %q/%q/%q with %s", name, event.Param, event.Gate, event.Kind(), event.Data, aggragates.StrategyParamSmartTakeLoss, tc.gate, tc.event, tc.document)
		}
	}
	if !strings.HasPrefix(lowRows[0].Message, "smartTakeLoss: flat market triggered") || !strings.HasPrefix(resetRows[1].Message, "smartTakeLoss: flat market triggered") || resetRows[1].Price != deeperFill {
		t.Errorf("the gate's own rows come before the activation, the reset before the new opening, got %+v and %+v", lowRows, resetRows)
	}
}

// The exit row of a forced sale is filed under the gate of the rule that sold
// — the slow decline gate's for the shared sale, naming the decline features the
// ladder was pending on, capital protection's and the flat market gate's for
// their own sales — as a sold event, at the level of the sale and not
// rounded, with the message the engines write beside a sellLoss, the level printed
// with the pair's decimals. The sale of the slow decline gate reads "sell at slow
// decline", the retired band in none of its words.
func TestExitRowFilesTheSaleUnderTheRuleThatSold(t *testing.T) {
	withQuietDecline(t, true)
	withSwingDecline(t, true)
	trade := lastDepthLadder()
	pendingOnQuiet := withRows(trade, PendingRow(trade.PositionPrice, quietFinding(nil)))
	pendingOnSwing := withRows(trade, PendingRow(trade.PositionPrice, swingFinding(nil)))
	pendingOnBoth := withRows(trade, PendingRow(trade.PositionPrice, indecisionFinding(nil), quietFinding(nil), swingFinding(nil)))
	for name, tc := range map[string]struct {
		trade  aggragates.Trades
		reason string
		level  float64
		want   Row
	}{
		"the shared sale, the quiet decline pending": {
			pendingOnQuiet, reasonSlowDecline, 175.39,
			Row{Message: "smartTakeLoss: sell at slow decline 175.39 (quiet decline)", Price: 175.39, Gate: "slowDecline", Event: "sold", Features: []string{"quietDecline"}},
		},
		"the shared sale, the swing decline pending": {
			pendingOnSwing, reasonSlowDecline, 175.39,
			Row{Message: "smartTakeLoss: sell at slow decline 175.39 (swing decline)", Price: 175.39, Gate: "slowDecline", Event: "sold", Features: []string{"swingDecline"}},
		},
		"the shared sale, both pending": {
			pendingOnBoth, reasonSlowDecline, 151.2,
			Row{Message: "smartTakeLoss: sell at slow decline 151.20 (quiet decline and swing decline)", Price: 151.2, Gate: "slowDecline", Event: "sold", Features: []string{"quietDecline", "swingDecline"}},
		},
		"the shared sale, nothing pending": {
			trade, reasonSlowDecline, 175.39,
			Row{Message: "smartTakeLoss: sell at slow decline 175.39", Price: 175.39, Gate: "slowDecline", Event: "sold"},
		},
		"capital protection's upper band": {
			pendingOnBoth, reasonCapitalProtection, 192.6543,
			Row{Message: "smartTakeLoss: sell at capital protection 192.65", Price: 192.6543, Gate: "capitalProtection", Event: "sold"},
		},
		"the flat market gate's sale, the ladder pending on a decline": {
			pendingOnBoth, reasonFlatMarket, 175.39,
			Row{Message: "smartTakeLoss: sell at flat market 175.39", Price: 175.39, Gate: "flatMarket", Event: "sold"},
		},
	} {
		got := ExitRow(tc.trade, tc.reason, tc.level)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %+v, want %+v", name, got, tc.want)
		}
	}

	at := testutil.At("21:45:00")
	logRow, event := Rows(trade, ExitRow(trade, reasonCapitalProtection, 192.6543), at)
	if logRow.Price != 192.6543 || logRow.Type != aggragates.LOG_INFO || !logRow.CreatedAt.Equal(at) {
		t.Fatalf("the sale's row is an INFO row at the level, got %+v", logRow)
	}
	if event.Gate != GateCapitalProtection || event.Kind() != EventSold || string(event.Data) != `{"event":"sold","price":192.6543}` {
		t.Fatalf("the sale's event names the rule and the level, got %+v: %s", event, event.Data)
	}
	_, quietSold := Rows(pendingOnQuiet, ExitRow(pendingOnQuiet, reasonSlowDecline, 175.39), at)
	if quietSold.Gate != GateSlowDecline || quietSold.Kind() != EventSold || string(quietSold.Data) != `{"event":"sold","price":175.39,"features":["quietDecline"]}` {
		t.Fatalf("the gate's sale carries the features it ended, got %+v: %s", quietSold, quietSold.Data)
	}
}

// The exit row names the rule that sold and the level the sellLoss chain
// places its limit at, with the pair's PriceFilter decimals, and the decline
// features a pending ladder is read pending on: the slow decline gate's sale
// names them, capital protection's and the flat market gate's none.
func TestExitMessagePrintsTheReasonAndTheLevel(t *testing.T) {
	withQuietDecline(t, true)
	withSwingDecline(t, true)
	trade := lastDepthLadder()
	if got := ExitMessage(trade, reasonCapitalProtection, 192.6543); got != "smartTakeLoss: sell at capital protection 192.65" {
		t.Fatalf("unexpected exit message %q", got)
	}
	trade.StrategyPair.TradeFilters.PriceFilter = 0
	if got := ExitMessage(trade, reasonSlowDecline, 175.39); got != "smartTakeLoss: sell at slow decline 175.39" {
		t.Fatalf("without a price filter the shortest exact form is printed, got %q", got)
	}
	pending := withRows(trade, PendingRow(trade.PositionPrice, swingFinding(nil)))
	if got := ExitMessage(pending, reasonSlowDecline, 175.39); got != "smartTakeLoss: sell at slow decline 175.39 (swing decline)" {
		t.Fatalf("the sale names the feature the ladder is pending on, got %q", got)
	}
	if got := ExitMessage(pending, reasonCapitalProtection, 175.39); got != "smartTakeLoss: sell at capital protection 175.39" {
		t.Fatalf("capital protection's sale names no feature, got %q", got)
	}
	if got := ExitMessage(pending, reasonFlatMarket, 175.39); got != "smartTakeLoss: sell at flat market 175.39" {
		t.Fatalf("the flat market gate's sale names no feature, got %q", got)
	}
}

// The flat market gate's rows are worded and filed as stored, the sold row of its
// sale with them: each message in its byte-stable text, at the newest fill's price
// or the level of the sale, and beside it the event of GateFlatMarket whose
// document is the kind and the price, the reasons after them on the triggered row
// alone, and no feature anywhere — not even on a sale of a ladder the slow decline
// gate holds pending, which this gate's sale does not end. The triggered row lists
// its reasons in parentheses and prints none when it has none, and takes a copy of
// them; the held sale's names the bound that held it, the loss and the shared cap
// with two decimals or the depth, in the same event either way; the activation names
// the loss it is made at beside the cap it is within, and the reset the new depth. The
// depth bound the pins run under is one they name.
func TestTheFlatMarketRowsAreWordedAndFiledAsStored(t *testing.T) {
	withAllowedLoss(t, shippedLoss)
	withSellLossIgnoreLastDepths(t, 3)
	withQuietDecline(t, true)
	const fill = 184.45
	trade := flatMarketLadder(false)
	pending := withRows(trade, PendingRow(trade.PositionPrice, quietFinding(nil)))
	if len(ExitRow(pending, reasonSlowDecline, fill).Features) == 0 {
		t.Fatal("fixture drifted: the shared sale of this ladder must name the decline it is pending on")
	}
	reasons := []string{"first reason", "second reason"}
	triggered := FlatMarketTriggeredRow(fill, reasons)
	reasons[0] = "changed"
	loss := 2 * float64(shippedLoss)

	at := testutil.At("21:45:00")
	for name, tc := range map[string]struct {
		row      Row
		message  string
		kind     string
		document string
	}{
		"triggered": {
			triggered,
			"smartTakeLoss: flat market triggered (first reason, second reason)", EventTriggered,
			`{"event":"triggered","price":184.45,"reasons":["first reason","second reason"]}`,
		},
		"triggered, no reason": {
			FlatMarketTriggeredRow(fill, nil),
			"smartTakeLoss: flat market triggered", EventTriggered,
			`{"event":"triggered","price":184.45}`,
		},
		"sale held over the cap": {
			FlatMarketSaleHeldRow(fill, -loss),
			"smartTakeLoss: flat market sale held, " + byLossText(loss, float64(shippedLoss)), EventSaleHeld,
			`{"event":"saleHeld","price":184.45,"trigger":` + strconv.FormatFloat(FullTrendTrigger(), 'f', -1, 64) + `}`,
		},
		"sale held past the depth bound": {
			FlatMarketSaleHeldAtDepthRow(fill, 7, 8),
			"smartTakeLoss: flat market sale held, depth 7 of 8 over 5", EventSaleHeld,
			`{"event":"saleHeld","price":184.45}`,
		},
		"sale held with no known ceiling": {
			FlatMarketSaleHeldAtDepthRow(fill, 7, 0),
			"smartTakeLoss: flat market sale held, depth 7 of no known ceiling", EventSaleHeld,
			`{"event":"saleHeld","price":184.45}`,
		},
		"stopLoss activated": {
			FlatMarketStopLossActivatedRow(fill, -0.25),
			"smartTakeLoss: flat market stopLoss activated, " + activatedText(0.25, float64(shippedLoss)), EventActivated,
			`{"event":"activated","price":184.45}`,
		},
		"reset at the new depth": {
			FlatMarketResetRow(fill),
			"smartTakeLoss: flat market reset at the new depth", EventReset,
			`{"event":"reset","price":184.45}`,
		},
		"sold": {
			ExitRow(trade, reasonFlatMarket, fill),
			"smartTakeLoss: sell at flat market 184.45", EventSold,
			`{"event":"sold","price":184.45}`,
		},
		"sold, the ladder pending on a decline": {
			ExitRow(pending, reasonFlatMarket, fill),
			"smartTakeLoss: sell at flat market 184.45", EventSold,
			`{"event":"sold","price":184.45}`,
		},
	} {
		if tc.row.Message != tc.message || tc.row.Gate != GateFlatMarket || tc.row.Event != tc.kind || tc.row.Price != fill || len(tc.row.Features) != 0 {
			t.Errorf("%s: the row is %+v, want %q filed under %q/%q at %v with no feature", name, tc.row, tc.message, GateFlatMarket, tc.kind, fill)
		}
		logRow, event := Rows(trade, tc.row, at)
		if logRow.Message != tc.message || logRow.Price != fill || logRow.Type != aggragates.LOG_INFO || !logRow.CreatedAt.Equal(at) {
			t.Errorf("%s: the log row is %+v, want an INFO row of %q at %v", name, logRow, tc.message, fill)
		}
		if event.Param != aggragates.StrategyParamSmartTakeLoss || event.Gate != GateFlatMarket || event.Kind() != tc.kind || !event.CreatedAt.Equal(at) || string(event.Data) != tc.document {
			t.Errorf("%s: the event is %q/%q/%q with %s, want %q/%q/%q with %s", name, event.Param, event.Gate, event.Kind(), event.Data, aggragates.StrategyParamSmartTakeLoss, GateFlatMarket, tc.kind, tc.document)
		}
	}
}
