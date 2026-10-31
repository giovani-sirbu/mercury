package smartsell

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// Every row's key is SMART_SELL_<GATE>_<EVENT>, the activation naming its path, the exit SMART_SELL_SOLD
// and the latch SMART_SELL_EARLY_TAKE_PROFIT_LATCHED; an activation written before the state reads
// as the take loss path. Every key fits the column.
func TestEveryRowIsKeyedByItsGateAndEvent(t *testing.T) {
	const fill = 98.0
	reasons := []string{"read"}
	finding := quietFinding(reasons)
	trade := flatMarketLadder(false)
	legacy := ActivatedRow(fill, -0.5)
	legacy.State = ""
	for want, row := range map[string]Row{
		"SMART_SELL_SLOW_DECLINE_PENDING":                       PendingRow(fill, finding),
		"SMART_SELL_SLOW_DECLINE_CANCELLED":                     CancelledRow(fill, finding),
		"SMART_SELL_SLOW_DECLINE_RESET":                         ResetRow(fill, finding),
		"SMART_SELL_EARLY_TAKE_PROFIT_LATCHED":                  LatchedRow(fill, indecisionFinding(reasons)),
		"SMART_SELL_SLOW_DECLINE_SALE_HELD":                     SaleHeldRow(fill, -1, FeatureQuietDecline),
		"SMART_SELL_SLOW_DECLINE_TAKE_PROFIT_ACTIVATED":         ActivatedRow(fill, 0, FeatureQuietDecline),
		"SMART_SELL_SLOW_DECLINE_TAKE_LOSS_ACTIVATED":           legacy,
		"SMART_SELL_FLAT_MARKET_TRIGGERED":                      FlatMarketTriggeredRow(fill, reasons),
		"SMART_SELL_FLAT_MARKET_RESET":                          FlatMarketResetRow(fill),
		"SMART_SELL_FLAT_MARKET_TAKE_LOSS_ACTIVATED":            FlatMarketActivatedRow(fill, -0.5),
		"SMART_SELL_FREE_FALL_PREVENTION_TRIGGERED":             FreeFallPreventionTriggeredRow(fill, reasons),
		"SMART_SELL_FREE_FALL_PREVENTION_TAKE_PROFIT_ACTIVATED": FreeFallPreventionActivatedRow(fill, 0.5),
		"SMART_SELL_FREE_FALL_PREVENTION_SALE_HELD":             FreeFallPreventionSaleHeldAtDepthRow(fill, 7, 8),
		"SMART_SELL_CAPITAL_PROTECTION_TRIGGERED":               CapitalProtectionTriggeredRow(fill, 9, 9, false),
		"SMART_SELL_CAPITAL_PROTECTION_SALE_HELD":               CapitalProtectionSaleHeldRow(fill, -3),
		"SMART_SELL_CAPITAL_PROTECTION_REACHED":                 TriggerReachedRow(GateCapitalProtection, fill, -0.5, -1),
		"SMART_SELL_CAPITAL_PROTECTION_GAP_HELD":                SaleHeldByGapRow(GateCapitalProtection, fill, 2, 1.25, 5),
		"SMART_SELL_CAPITAL_PROTECTION_GAP_CLOSED":              GapClosedRow(GateCapitalProtection, fill, 0.5),
		"SMART_SELL_CAPITAL_PROTECTION_RESET":                   CapitalProtectionResetRow(fill),
		"SMART_SELL_SOLD":                                       ExitRow(trade, reasonFlatMarket, fill),
	} {
		got := keyOf(row)
		if got != want {
			t.Errorf("keyOf(%q) = %q, want %q", row.Message, got, want)
		}
		if len(got) > 64 {
			t.Errorf("%q is longer than the column's 64 characters", got)
		}
		if logRow, _ := Rows(trade, row, testutil.At("13:00:00")); logRow.MessageKey != want {
			t.Errorf("the log row of %q is keyed %q, want %q", row.Message, logRow.MessageKey, want)
		}
	}
}

// A merged line takes the key of its newest event: the opening and its held row read as held, the
// opening and its activation as the activation's path, a reset and its re-opening as triggered.
func TestAMergedLineTakesTheKeyOfItsNewestEvent(t *testing.T) {
	const fill = 98.0
	reasons := []string{"read"}
	opened := FlatMarketTriggeredRow(fill, reasons)
	trade := flatMarketLadder(false)
	for want, rows := range map[string][]Row{
		"SMART_SELL_FLAT_MARKET_SALE_HELD":             {opened, FlatMarketSaleHeldAtDepthRow(fill, 7, 8)},
		"SMART_SELL_FLAT_MARKET_TAKE_PROFIT_ACTIVATED": {opened, FlatMarketActivatedRow(fill, 0.25)},
		"SMART_SELL_FLAT_MARKET_TAKE_LOSS_ACTIVATED":   {FlatMarketResetRow(fill), opened, FlatMarketActivatedRow(fill, -0.25)},
		"SMART_SELL_FLAT_MARKET_TRIGGERED":             {FlatMarketResetRow(fill), opened},
	} {
		logs, events := TickRows(trade, rows, testutil.At("13:00:00"))
		if len(logs) != 1 || logs[0].MessageKey != want || len(events) != len(rows) {
			t.Errorf("rows %+v: want one line keyed %q beside an event per row, got %+v and %d events", rows, want, logs, len(events))
		}
	}
}

// Every row the flag writes opens with RowPrefix: the episodes', the shared action's, the slow decline
// gate's, the exit's and the first-fill holds' texts.
func TestEveryRowOpensWithTheRowPrefix(t *testing.T) {
	const fill = 98.0
	reasons := []string{"read"}
	trade := flatMarketLadder(false)
	unread := aggragates.SmartSellIndicators{}
	rows := []Row{
		PendingRow(fill, quietFinding(reasons)),
		ConfirmedRow(fill, quietFinding(reasons)),
		CancelledRow(fill, quietFinding(reasons)),
		ResetRow(fill, quietFinding(reasons)),
		LatchedRow(fill, indecisionFinding(reasons)),
		SaleHeldRow(fill, -1, FeatureQuietDecline),
		SaleHeldAtDepthRow(fill, 7, 8, FeatureQuietDecline),
		SaleHeldByTrendRow(fill, unread, false, FeatureQuietDecline),
		ActivatedRow(fill, 0.5, FeatureQuietDecline),
		ActivatedRow(fill, -0.5, FeatureQuietDecline),
		TriggerReachedRow(GateFlatMarket, fill, -0.5, -1),
		FlatMarketTriggeredRow(fill, reasons),
		FlatMarketSaleHeldRow(fill, -1),
		FlatMarketSaleHeldAtDepthRow(fill, 7, 8),
		FlatMarketSaleHeldByTrendRow(fill, unread, false),
		FlatMarketActivatedRow(fill, 0.5),
		FlatMarketResetRow(fill),
		FreeFallPreventionTriggeredRow(fill, reasons),
		FreeFallPreventionSaleHeldRow(fill, -1),
		FreeFallPreventionSaleHeldAtDepthRow(fill, 7, 8),
		FreeFallPreventionSaleHeldByTrendRow(fill, unread, false),
		FreeFallPreventionActivatedRow(fill, -0.5),
		FreeFallPreventionResetRow(fill),
		CapitalProtectionTriggeredRow(fill, 9, 9, false),
		CapitalProtectionSaleHeldRow(fill, -3),
		CapitalProtectionActivatedRow(fill, 0.5),
		CapitalProtectionResetRow(fill),
		SaleHeldByGapRow(GateFlatMarket, fill, 2, 1.25, 5),
		GapClosedRow(GateFlatMarket, fill, 0.5),
		ExitRow(trade, reasonFlatMarket, fill),
	}
	for _, row := range rows {
		if !strings.HasPrefix(row.Message, RowPrefix) {
			t.Errorf("%q does not open with %q", row.Message, RowPrefix)
		}
	}
	for _, text := range []string{SlowDeclineEntryHoldReason, FlatMarketEntryHoldReason, FreeFallPreventionEntryHoldReason, ExitMessage(trade, reasonCapitalProtection, fill)} {
		if !strings.HasPrefix(text, RowPrefix) {
			t.Errorf("%q does not open with %q", text, RowPrefix)
		}
	}
}

// pendingMessage reads back the pending row buildRow writes, triggered or confirmed: the verb after
// the declines and the suffix after the reasons, the prefix kept; a row of another grammar is refused.
func TestPendingMessageReadsBackThePendingRowsGrammar(t *testing.T) {
	const fill = 98.0
	finding := quietFinding([]string{"read"})
	for _, c := range []struct {
		row  Row
		want string
	}{
		{PendingRow(fill, finding), RowPrefix + "quiet decline pending (quiet decline: read), sale held: x"},
		{ConfirmedRow(fill, finding), RowPrefix + "quiet decline confirmed at the new fill, pending (quiet decline: read), sale held: x"},
	} {
		got, ok := pendingMessage(c.row, "pending", ", sale held: x")
		if !ok || got != c.want {
			t.Errorf("pendingMessage(%q) = %q, %v, want %q", c.row.Message, got, ok, c.want)
		}
	}
	if _, ok := pendingMessage(Row{Message: "other: quiet decline triggered"}, "pending", ""); ok {
		t.Error("a row of another prefix is not the pending grammar")
	}
	if _, ok := pendingMessage(PendingRow(fill, finding), "", ""); ok {
		t.Error("no verb merges nothing")
	}
}
