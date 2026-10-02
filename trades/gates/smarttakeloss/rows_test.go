package smarttakeloss

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The gate, kind and feature names and the keys of the document are what the
// events store, and what every reader of the table finds them by: they must not
// move. A document that carries no features — every one written before the merge,
// and the sold event of capital protection — is byte for byte what it was.
func TestTheEventNamesAreTheStoredContract(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{GateSlowDecline, "slowDecline"},
		{GateCapitalProtection, "capitalProtection"},
		{GateEntryHold, "entryHold"},
		{gateIndecision, "indecision"},
		{gateSlowPattern, "slowPattern"},
		{EventPending, "pending"},
		{EventCancelled, "cancelled"},
		{EventReset, "reset"},
		{EventSold, "sold"},
		{EventLatched, "latched"},
		{EventSaleHeld, "saleHeld"},
		{FeatureIndecision, "indecision"},
		{FeatureQuietDecline, "quietDecline"},
		{FeatureSwingDecline, "swingDecline"},
		{gates.EventHeld, "held"},
		{aggragates.StrategyParamSmartTakeLoss, "smartTakeLoss"},
	} {
		if tc.got != tc.want {
			t.Errorf("the stored name %q must not move, got %q", tc.want, tc.got)
		}
	}

	for name, tc := range map[string]struct {
		data EventData
		want string
	}{
		"the document as it always was":    {EventData{Event: EventPending, Price: 184.45, Reasons: []string{"leg down", "leg 60 bars long"}}, `{"event":"pending","price":184.45,"reasons":["leg down","leg 60 bars long"]}`},
		"a document leaves out the absent": {EventData{Event: gates.EventHeld}, `{"event":"held"}`},
		"the sale of capital protection":   {EventData{Event: EventSold, Price: 192.6543}, `{"event":"sold","price":192.6543}`},
		"features after the reasons":       {EventData{Event: EventPending, Price: 184.45, Reasons: []string{"a"}, Features: []string{FeatureIndecision, FeatureSwingDecline}}, `{"event":"pending","price":184.45,"reasons":["a"],"features":["indecision","swingDecline"]}`},
		"features without reasons":         {EventData{Event: EventReset, Price: 184.45, Features: []string{FeatureQuietDecline}}, `{"event":"reset","price":184.45,"features":["quietDecline"]}`},
		"a sale held":                      {EventData{Event: EventSaleHeld, Price: 184.45, Features: []string{FeatureQuietDecline, FeatureSwingDecline}}, `{"event":"saleHeld","price":184.45,"features":["quietDecline","swingDecline"]}`},
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
	trade := watchedTrade()
	trade.PositionPrice = underTheBand
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
	next := Append(trade, CancelledRow(sixthFill, quietFinding(slowDeclineBreakReasons)), at)

	if len(next.Logs) != 2 || len(next.StrategyEvents) != 2 {
		t.Fatalf("the pair joins the trade, got %d rows and %d events", len(next.Logs), len(next.StrategyEvents))
	}
	row, event := next.Logs[1], next.StrategyEvents[1]
	if row.Message != CancelledRow(sixthFill, quietFinding(slowDeclineBreakReasons)).Message || row.Price != sixthFill || !row.CreatedAt.Equal(at) {
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

// Every row Apply hands back is filed under its gate and kind, whatever the
// ladder does next, with the features it is about: a ladder going pending on
// the quiet decline, a fill the gate confirms, a fill it cancels, a pending
// ladder a depth priority holds, a ladder the swing decline makes pending and
// latches, a fill the swing decline confirms, a ladder the indecision latches.
// Each is stated here with the literal names that are stored; the reasons are
// the ones the readings serve, and the texts are pinned in
// slowDeclineRow_test.go.
func TestApplyHandsBackEveryRowFiledUnderItsGateAndKind(t *testing.T) {
	fourth := stairReading(t, stairFourth(), stairFourthBlock(), true).Reasons
	fifth := stairReading(t, stairFifthFill(t), stairFifthBlock(), true).Reasons
	for name, tc := range map[string]struct {
		got                *Row
		gate, event        string
		features, reasons  []string
		price              float64
		messagePrefix, end string
	}{
		"a ladder going pending on the quiet decline": {
			firstRow(Apply(watchedTrade(), "", underTheBand, slowDeclineBlock(true))), "slowDecline", "pending",
			[]string{"quietDecline"}, slowDeclineReasons, slowDeclineLastFill, "smartTakeLoss: quiet decline triggered (quiet decline: ", ")",
		},
		"a fill the gate confirms": {
			firstRow(Apply(pendingAtFive(), "", underJudgeBand, legOnAndQuiet())), "slowDecline", "pending",
			[]string{"quietDecline"}, slowDeclineReasons, sixthFill, "smartTakeLoss: quiet decline confirmed at the new fill (quiet decline: ", ")",
		},
		"a fill the gate cancels": {
			firstRow(Apply(pendingAtFive(), "", underJudgeBand, brokenReading())), "slowDecline", "cancelled",
			[]string{"quietDecline"}, slowDeclineBreakReasons, sixthFill, "smartTakeLoss: slow decline cancelled at the new fill (quiet decline: ", ")",
		},
		"a pending ladder a depth priority holds": {
			firstRow(Apply(heldBy(pendingTrade(), testutil.At("18:05:00")), "", underTheBand, slowDeclineBlock(true))), "slowDecline", "reset",
			[]string{"quietDecline"}, nil, slowDeclineLastFill, "smartTakeLoss: slow decline reset, depth priority holds this ladder", "",
		},
		"a ladder the swing decline makes pending and latches": {
			firstRow(Apply(stairFourth(), "", underTheBand, withBlock(stairFourthBlock()))), "slowDecline", "pending",
			[]string{"indecision", "swingDecline"}, fourth, stairPrices[3], "smartTakeLoss: indecision direction and swing decline triggered (swing decline: ", ")",
		},
		"a fill the swing decline confirms": {
			firstRow(Apply(stairFifthFill(t), "", underTheBand, withBlock(stairFifthBlock()))), "slowDecline", "pending",
			[]string{"swingDecline"}, fifth, stairPrices[4], "smartTakeLoss: swing decline confirmed at the new fill (swing decline: ", ")",
		},
		"a ladder the indecision latches": {
			firstRow(Apply(indecisionLadder(), "", underTheBand, indecisionReading())), "slowDecline", "latched",
			[]string{"indecision"}, indecisionReasons, indecisionLadder().PositionPrice, "smartTakeLoss: indecision direction triggered (indecision direction: ", ")",
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

// The exit row of a forced sale is filed under the gate of the rule that sold
// — the slow decline gate's for its sell band, naming the decline features the
// ladder was pending on, capital protection's for its upper band — as a sold
// event, at the level of the sale and not rounded, with the message the
// engines write beside a sellLoss, the level printed with the pair's decimals.
func TestExitRowFilesTheSaleUnderTheRuleThatSold(t *testing.T) {
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
		"the sell band, the quiet decline pending": {
			pendingOnQuiet, reasonSellBand, 175.39,
			Row{Message: "smartTakeLoss: sell at slow decline bollinger band 175.39 (quiet decline)", Price: 175.39, Gate: "slowDecline", Event: "sold", Features: []string{"quietDecline"}},
		},
		"the sell band, the swing decline pending": {
			pendingOnSwing, reasonSellBand, 175.39,
			Row{Message: "smartTakeLoss: sell at slow decline bollinger band 175.39 (swing decline)", Price: 175.39, Gate: "slowDecline", Event: "sold", Features: []string{"swingDecline"}},
		},
		"the sell band, both pending": {
			pendingOnBoth, reasonSellBand, 151.2,
			Row{Message: "smartTakeLoss: sell at slow decline bollinger band 151.20 (quiet decline and swing decline)", Price: 151.2, Gate: "slowDecline", Event: "sold", Features: []string{"quietDecline", "swingDecline"}},
		},
		"the sell band, nothing pending": {
			trade, reasonSellBand, 175.39,
			Row{Message: "smartTakeLoss: sell at slow decline bollinger band 175.39", Price: 175.39, Gate: "slowDecline", Event: "sold"},
		},
		"capital protection's upper band": {
			pendingOnBoth, reasonCapitalProtection, 192.6543,
			Row{Message: "smartTakeLoss: sell at upper bollinger band (capital protection) 192.65", Price: 192.6543, Gate: "capitalProtection", Event: "sold"},
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
	_, quietSold := Rows(pendingOnQuiet, ExitRow(pendingOnQuiet, reasonSellBand, 175.39), at)
	if quietSold.Gate != GateSlowDecline || quietSold.Kind() != EventSold || string(quietSold.Data) != `{"event":"sold","price":175.39,"features":["quietDecline"]}` {
		t.Fatalf("the gate's sale carries the features it ended, got %+v: %s", quietSold, quietSold.Data)
	}
}

// The exit row names the rule that sold and the level the sellLoss chain
// places its limit at, with the pair's PriceFilter decimals, and the decline
// features a pending ladder is read pending on.
func TestExitMessagePrintsTheReasonAndTheLevel(t *testing.T) {
	trade := lastDepthLadder()
	if got := ExitMessage(trade, reasonCapitalProtection, 192.6543); got != "smartTakeLoss: sell at upper bollinger band (capital protection) 192.65" {
		t.Fatalf("unexpected exit message %q", got)
	}
	trade.StrategyPair.TradeFilters.PriceFilter = 0
	if got := ExitMessage(trade, reasonSellBand, 175.39); got != "smartTakeLoss: sell at slow decline bollinger band 175.39" {
		t.Fatalf("without a price filter the shortest exact form is printed, got %q", got)
	}
	pending := withRows(trade, PendingRow(trade.PositionPrice, swingFinding(nil)))
	if got := ExitMessage(pending, reasonSellBand, 175.39); got != "smartTakeLoss: sell at slow decline bollinger band 175.39 (swing decline)" {
		t.Fatalf("the sale names the feature the ladder is pending on, got %q", got)
	}
	if got := ExitMessage(pending, reasonCapitalProtection, 175.39); got != "smartTakeLoss: sell at upper bollinger band (capital protection) 175.39" {
		t.Fatalf("capital protection's sale names no feature, got %q", got)
	}
}
