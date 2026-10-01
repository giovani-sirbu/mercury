package smarttakeloss

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The gate and kind names and the keys of the document are what the events
// store, and what every reader of the table finds them by: they must not move.
func TestTheEventNamesAreTheStoredContract(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{GateSlowDecline, "slowDecline"},
		{GateCapitalProtection, "capitalProtection"},
		{GateIndecision, "indecision"},
		{GateEntryHold, "entryHold"},
		{EventPending, "pending"},
		{EventCancelled, "cancelled"},
		{EventReset, "reset"},
		{EventSold, "sold"},
		{EventLatched, "latched"},
		{gates.EventHeld, "held"},
		{aggragates.StrategyParamSmartTakeLoss, "smartTakeLoss"},
	} {
		if tc.got != tc.want {
			t.Errorf("the stored name %q must not move, got %q", tc.want, tc.got)
		}
	}

	full, _ := json.Marshal(EventData{Event: EventPending, Price: 184.45, Reasons: []string{"leg down", "leg 60 bars long"}})
	if string(full) != `{"event":"pending","price":184.45,"reasons":["leg down","leg 60 bars long"]}` {
		t.Errorf("the document's keys must not move, got %s", full)
	}
	bare, _ := json.Marshal(EventData{Event: gates.EventHeld})
	if string(bare) != `{"event":"held"}` {
		t.Errorf("a document leaves out what it does not carry, got %s", bare)
	}
}

// Rows builds the pair to write. The row is INFO, on the trade, at the row's
// own price — never the position price a re-anchor moved — with the engine's
// clock on both stamps and nothing else set. The event is the smartTakeLoss
// event of the row's gate and kind, its document the kind, the price and the
// reasons, stamped with the same clock: the pair is found by (TradeID,
// CreatedAt). What it writes reads back as the ladder pending from that fill.
func TestRowsBuildAnInfoRowAtTheFillPriceAndTheEventBesideIt(t *testing.T) {
	trade := watchedTrade()
	trade.PositionPrice = underTheBand
	at := testutil.At("17:45:00")
	row := PendingRow(trade.PositionType, slowDeclineLastFill, nil)
	logRow, event := Rows(trade, row, at)

	wantRow := aggragates.TradesLogs{
		TradeID:   trade.ID,
		Message:   "Hold buy: smartTakeLoss: quiet slow decline, sell at the bollinger band",
		Type:      aggragates.LOG_INFO,
		Price:     slowDeclineLastFill,
		CreatedAt: at,
		UpdatedAt: at,
	}
	if !reflect.DeepEqual(logRow, wantRow) {
		t.Fatalf("the row carries the marker at the fill on the engine's clock and nothing else, got %+v, want %+v", logRow, wantRow)
	}

	if event.ID != 0 || event.TradeID != trade.ID || event.Param != aggragates.StrategyParamSmartTakeLoss || event.Gate != GateSlowDecline {
		t.Fatalf("the event is the trade's smartTakeLoss event of the row's gate, got %+v", event)
	}
	if event.Kind() != EventPending || !event.CreatedAt.Equal(at) {
		t.Fatalf("the event names the row's kind on the row's stamp, got %+v", event)
	}
	if string(event.Data) != `{"event":"pending","price":184.45}` {
		t.Fatalf("the document is the kind and the fill's price, got %s", event.Data)
	}
	if event.TradeID != logRow.TradeID || !event.CreatedAt.Equal(logRow.CreatedAt) {
		t.Fatalf("row and event are a pair by (trade, stamp), got %+v and %+v", logRow, event)
	}

	written := aggragates.AppendStrategyRow(trade, logRow, event)
	if st := rebuildState(written); !st.slowDeclinePending || st.slowDeclinePendingFrom != slowDeclineLastFill {
		t.Fatalf("what Rows writes must read back as the ladder pending from its fill, got %+v", st)
	}
}

// The reasons ride in the message after the marker and in the document, in the
// order sophos served them, and the row's price stays the fill's whatever the
// reasons say.
func TestRowsCarryTheReasonsInBothForms(t *testing.T) {
	trade := watchedTrade()
	logRow, event := Rows(trade, PendingRow("stopLoss", slowDeclineLastFill, slowDeclineReasons), testutil.At("17:45:00"))

	if logRow.Message != "Hold stopLoss: smartTakeLoss: quiet slow decline, sell at the bollinger band (leg down 7.0% from its high close, leg 60 bars long on 1h)" {
		t.Fatalf("the reasons follow the marker in parentheses, got %q", logRow.Message)
	}
	if string(event.Data) != `{"event":"pending","price":184.45,"reasons":["leg down 7.0% from its high close","leg 60 bars long on 1h"]}` {
		t.Fatalf("the document names them in the same order, got %s", event.Data)
	}
}

// Append is the pair appended, both slices copied: the trade the caller holds
// is never written through, even where it has spare capacity in its backing
// arrays, and the trade handed back carries the pair last.
func TestAppendWritesThePairAndNeverTheCallersArrays(t *testing.T) {
	trade := withRows(watchedTrade(), PendingRow("buy", slowDeclineLastFill, nil))
	trade.Logs = append(make([]aggragates.TradesLogs, 0, 8), trade.Logs...)
	trade.StrategyEvents = append(make([]aggragates.TradesStrategyEvents, 0, 8), trade.StrategyEvents...)

	at := testutil.At("18:00:00")
	next := Append(trade, CancelledRow("buy", sixthFill, slowDeclineBreakReasons), at)

	if len(next.Logs) != 2 || len(next.StrategyEvents) != 2 {
		t.Fatalf("the pair joins the trade, got %d rows and %d events", len(next.Logs), len(next.StrategyEvents))
	}
	row, event := next.Logs[1], next.StrategyEvents[1]
	if row.Message != SlowDeclineCancelMessage("buy", slowDeclineBreakReasons) || row.Price != sixthFill || !row.CreatedAt.Equal(at) {
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
// ladder does next: the pending row of a ladder going pending and of a fill
// the exit confirms, the cancelled row, the reset row, the latched row. Each is
// stated here with the literal names that are stored.
func TestApplyHandsBackEveryRowFiledUnderItsGateAndKind(t *testing.T) {
	for name, tc := range map[string]struct {
		got  *Row
		want Row
	}{
		"a ladder going pending": {
			Apply(watchedTrade(), "", underTheBand, slowDeclineBlock(true)).SlowDecline,
			Row{Message: SlowDeclineMessage("buy", slowDeclineReasons), Price: slowDeclineLastFill, Gate: "slowDecline", Event: "pending", Reasons: slowDeclineReasons},
		},
		"a fill the exit confirms": {
			Apply(pendingAtFive(), "", underJudgeBand, legOnAndQuiet()).SlowDecline,
			Row{Message: SlowDeclineMessage("buy", slowDeclineReasons), Price: sixthFill, Gate: "slowDecline", Event: "pending", Reasons: slowDeclineReasons},
		},
		"a fill the exit breaks": {
			Apply(pendingAtFive(), "", underJudgeBand, brokenReading()).SlowDecline,
			Row{Message: SlowDeclineCancelMessage("buy", slowDeclineBreakReasons), Price: sixthFill, Gate: "slowDecline", Event: "cancelled", Reasons: slowDeclineBreakReasons},
		},
		"a pending ladder a depth priority holds": {
			Apply(heldBy(pendingTrade(), testutil.At("18:05:00")), "", underTheBand, slowDeclineBlock(true)).SlowDecline,
			Row{Message: SlowDeclineResetMessage("buy"), Price: slowDeclineLastFill, Gate: "slowDecline", Event: "reset"},
		},
		"a ladder the indecision latches": {
			Apply(indecisionLadder(), "", underTheBand, indecisionReading()).Indecision,
			Row{Message: IndecisionMessage("buy", indecisionReasons), Price: indecisionLadder().PositionPrice, Gate: "indecision", Event: "latched", Reasons: indecisionReasons},
		},
	} {
		if tc.got == nil || !reflect.DeepEqual(*tc.got, tc.want) {
			t.Errorf("%s: got %+v, want %+v", name, tc.got, tc.want)
		}
	}
}

// The exit row of a forced sale is filed under the gate of the rule that sold
// — the slow decline's for its sell band, capital protection's for its upper
// band — as a sold event, at the level of the sale and not rounded, with the
// message the engines write beside a sellLoss, the level printed with the
// pair's decimals.
func TestExitRowFilesTheSaleUnderTheRuleThatSold(t *testing.T) {
	trade := lastDepthLadder()
	for name, tc := range map[string]struct {
		reason string
		level  float64
		want   Row
	}{
		"the slow decline's sell band": {
			reasonSellBand, 175.39,
			Row{Message: "smartTakeLoss: sell at slow-decline bollinger band 175.39", Price: 175.39, Gate: "slowDecline", Event: "sold"},
		},
		"capital protection's upper band": {
			reasonCapitalProtection, 192.6543,
			Row{Message: "smartTakeLoss: sell at upper bollinger band (capital protection) 192.65", Price: 192.6543, Gate: "capitalProtection", Event: "sold"},
		},
	} {
		got := ExitRow(trade, tc.reason, tc.level)
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
}

// The exit row names the rule that sold and the level the sellLoss chain
// places its limit at, with the pair's PriceFilter decimals.
func TestExitMessagePrintsTheReasonAndTheLevel(t *testing.T) {
	trade := lastDepthLadder()
	if got := ExitMessage(trade, reasonCapitalProtection, 192.6543); got != "smartTakeLoss: sell at upper bollinger band (capital protection) 192.65" {
		t.Fatalf("unexpected exit message %q", got)
	}
	trade.StrategyPair.TradeFilters.PriceFilter = 0
	if got := ExitMessage(trade, reasonSellBand, 175.39); got != "smartTakeLoss: sell at slow-decline bollinger band 175.39" {
		t.Fatalf("without a price filter the shortest exact form is printed, got %q", got)
	}
}
