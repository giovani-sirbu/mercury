package smartsell

import (
	"reflect"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// mergeReasons are the reasons the triggered rows of the merge tests carry.
var mergeReasons = []string{"1h bars 2022-04-21 15:00 to 2022-04-23 02:00 UTC", "smc trend direction down 3/3 of 3 needed"}

// messagesOf is the messages of the log lines a tick's rows write.
func messagesOf(rows ...Row) []string {
	var messages []string
	for _, line := range logLines(rows) {
		messages = append(messages, line.Message)
	}
	return messages
}

// A gate writes one log line a tick: its reset and its triggered row read re-armed, a held sale
// reads pending with its reason, an activation carries the trigger's reasons; the slow decline
// gate's pending row takes its held or activation row the same way. Rows that open nothing stay
// as they are, in their order.
func TestATicksRowsMergeIntoOneLogLineAGate(t *testing.T) {
	const fill = 39998.56
	held := heldUnderDepth(3, 4)
	profit := activatedByProfit(-0.5, -0.5)
	linear := Finding{Feature: FeatureLinearDecline, Reasons: []string{"down 7.0% past 5.0%"}}
	for name, tc := range map[string]struct {
		rows []Row
		want []string
	}{
		"triggered and held read pending": {
			[]Row{FlatMarketTriggeredRow(fill, mergeReasons), saleHeldRow(GateFlatMarket, fill, held, nil)},
			[]string{"smartSell: flat market pending (1h bars 2022-04-21 15:00 to 2022-04-23 02:00 UTC, smc trend direction down 3/3 of 3 needed), sale held: depth 3 under the 4 depths the activation needs"},
		},
		"reset and triggered read re-armed": {
			[]Row{FlatMarketResetRow(fill), FlatMarketTriggeredRow(fill, mergeReasons[:1])},
			[]string{"smartSell: flat market re-armed at the new depth (1h bars 2022-04-21 15:00 to 2022-04-23 02:00 UTC)"},
		},
		"reset, triggered and held read re-armed and pending": {
			[]Row{FlatMarketResetRow(fill), FlatMarketTriggeredRow(fill, mergeReasons[:1]), saleHeldRow(GateFlatMarket, fill, held, nil)},
			[]string{"smartSell: flat market re-armed at the new depth, pending (1h bars 2022-04-21 15:00 to 2022-04-23 02:00 UTC), sale held: depth 3 under the 4 depths the activation needs"},
		},
		"triggered and activated read activated with the reasons": {
			[]Row{CapitalProtectionTriggeredRow(fill, 9, 9, false), activatedRow(GateCapitalProtection, fill, -0.5, -0.5, nil)},
			[]string{"smartSell: capital protection take loss activated (" + strings.Join(capitalProtectionReasons(9, 9, false), ", ") + "), " + profit},
		},
		"triggered and activated at a profit read take profit activated": {
			[]Row{CapitalProtectionTriggeredRow(fill, 9, 9, false), activatedRow(GateCapitalProtection, fill, 0.5, -0.5, nil)},
			[]string{"smartSell: capital protection take profit activated (" + strings.Join(capitalProtectionReasons(9, 9, false), ", ") + "), " + activatedByProfit(0.5, -0.5)},
		},
		"the slow decline gate's pending and held rows, the latch joined": {
			[]Row{PendingRow(fill, indecisionFinding(nil), linear), saleHeldRow(GateSlowDecline, fill, held, []string{FeatureLinearDecline})},
			[]string{"smartSell: early take profit latched, linear decline pending (linear decline: down 7.0% past 5.0%), sale held: depth 3 under the 4 depths the activation needs"},
		},
		"the slow decline gate's confirmation and activation": {
			[]Row{ConfirmedRow(fill, linear), activatedRow(GateSlowDecline, fill, -0.5, -0.5, []string{FeatureLinearDecline})},
			[]string{"smartSell: linear decline confirmed at the new fill, take loss activated (linear decline: down 7.0% past 5.0%), " + profit},
		},
		"the latch keeps its own line before the gate's": {
			[]Row{LatchedRow(fill, Finding{Feature: FeatureFlatMarket, Reasons: mergeReasons}), FlatMarketTriggeredRow(fill, mergeReasons), saleHeldRow(GateFlatMarket, fill, held, nil)},
			[]string{
				"smartSell: early take profit latched (flat market: 1h bars 2022-04-21 15:00 to 2022-04-23 02:00 UTC, smc trend direction down 3/3 of 3 needed)",
				"smartSell: flat market pending (1h bars 2022-04-21 15:00 to 2022-04-23 02:00 UTC, smc trend direction down 3/3 of 3 needed), sale held: depth 3 under the 4 depths the activation needs",
			},
		},
		"each gate merges at its first row, in order": {
			[]Row{PendingRow(fill, linear), CapitalProtectionTriggeredRow(fill, 9, 9, false), saleHeldRow(GateSlowDecline, fill, held, []string{FeatureLinearDecline}), saleHeldRow(GateCapitalProtection, fill, held, nil)},
			[]string{
				"smartSell: linear decline pending (linear decline: down 7.0% past 5.0%), sale held: depth 3 under the 4 depths the activation needs",
				"smartSell: capital protection pending (" + strings.Join(capitalProtectionReasons(9, 9, false), ", ") + "), sale held: depth 3 under the 4 depths the activation needs",
			},
		},
		"rows that open nothing stay": {
			[]Row{FlatMarketResetRow(fill), saleHeldRow(GateFlatMarket, fill, held, nil), LatchedRow(fill, linear), saleHeldRow(GateSlowDecline, fill, held, []string{FeatureLinearDecline})},
			[]string{
				"smartSell: flat market reset at the new depth",
				"smartSell: flat market sale held, depth 3 under the 4 depths the activation needs",
				"smartSell: early take profit latched (linear decline: down 7.0% past 5.0%)",
				"smartSell: slow decline sale held, depth 3 under the 4 depths the activation needs",
			},
		},
		"a triggered row alone stays": {
			[]Row{FlatMarketTriggeredRow(fill, mergeReasons[:1])},
			[]string{"smartSell: flat market triggered (1h bars 2022-04-21 15:00 to 2022-04-23 02:00 UTC)"},
		},
		"a reached row stays beside the opening row": {
			[]Row{FlatMarketTriggeredRow(fill, mergeReasons[:1]), TriggerReachedRow(GateFlatMarket, fill, -0.1, -0.15)},
			[]string{
				"smartSell: flat market triggered (1h bars 2022-04-21 15:00 to 2022-04-23 02:00 UTC)",
				"smartSell: flat market trigger reached, profit -0.10% of the used amount at or over -0.15%, the activation confirms after 0s",
			},
		},
		"a held row and a reached row merge only the held one": {
			[]Row{FlatMarketTriggeredRow(fill, mergeReasons[:1]), saleHeldRow(GateFlatMarket, fill, held, nil), TriggerReachedRow(GateCapitalProtection, fill, -0.1, -1)},
			[]string{
				"smartSell: flat market pending (1h bars 2022-04-21 15:00 to 2022-04-23 02:00 UTC), sale held: depth 3 under the 4 depths the activation needs",
				"smartSell: capital protection trigger reached, profit -0.10% of the used amount at or over -1.00%, the activation confirms after 0s",
			},
		},
		"a gap row stays apart": {
			[]Row{SaleHeldByGapRow(GateFlatMarket, fill, 2.25, 1.25, 5)},
			[]string{"smartSell: flat market sale held, print 2.25% under the anchor, over the gap of 5 tolerances (1.25%)"},
		},
		"a gap closed row stays apart": {
			[]Row{GapClosedRow(GateFlatMarket, fill, 0.5)},
			[]string{"smartSell: flat market gap closed, print 0.50% over the anchor"},
		},
		"a gap closed row beside a gate's rows merges nothing": {
			[]Row{GapClosedRow(GateFlatMarket, fill, -0.1), FlatMarketTriggeredRow(fill, mergeReasons[:1]), saleHeldRow(GateFlatMarket, fill, held, nil), GapClosedRow(GateFlatMarket, fill, 1.5)},
			[]string{
				"smartSell: flat market gap closed, print 0.10% under the anchor",
				"smartSell: flat market pending (1h bars 2022-04-21 15:00 to 2022-04-23 02:00 UTC), sale held: depth 3 under the 4 depths the activation needs",
				"smartSell: flat market gap closed, print 1.50% over the anchor",
			},
		},
		"a gap closed row beside the opening row stays apart": {
			[]Row{FlatMarketTriggeredRow(fill, mergeReasons[:1]), GapClosedRow(GateFlatMarket, fill, 0.5)},
			[]string{
				"smartSell: flat market triggered (1h bars 2022-04-21 15:00 to 2022-04-23 02:00 UTC)",
				"smartSell: flat market gap closed, print 0.50% over the anchor",
			},
		},
		"a held row of another grammar stays apart": {
			[]Row{FlatMarketTriggeredRow(fill, mergeReasons[:1]), {Message: "smartSell: held by hand", Price: fill, Gate: GateFlatMarket, Event: EventSaleHeld}},
			[]string{"smartSell: flat market triggered (1h bars 2022-04-21 15:00 to 2022-04-23 02:00 UTC)", "smartSell: held by hand"},
		},
	} {
		if got := messagesOf(tc.rows...); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s:\n got %q\nwant %q", name, got, tc.want)
		}
	}
}

// TickRows writes one event per row, in order and as Rows builds it, beside the merged log lines,
// every one stamped at the tick and priced at the fill: the state the folds read is the rows'.
func TestTickRowsKeepsAnEventPerRowBesideTheMergedLines(t *testing.T) {
	const fill = 39998.56
	trade := flatMarketLadder(false)
	at := testutil.At("03:00:00")
	rows := []Row{
		LatchedRow(fill, Finding{Feature: FeatureFlatMarket, Reasons: mergeReasons}),
		FlatMarketTriggeredRow(fill, mergeReasons),
		saleHeldRow(GateFlatMarket, fill, heldUnderDepth(3, 4), nil),
	}
	logs, events := TickRows(trade, rows, at)
	if len(logs) != 2 || len(events) != len(rows) {
		t.Fatalf("want 2 log lines beside %d events, got %d and %d", len(rows), len(logs), len(events))
	}
	for index, row := range rows {
		_, want := Rows(trade, row, at)
		if !reflect.DeepEqual(events[index], want) {
			t.Errorf("event %d: got %+v, want %+v", index, events[index], want)
		}
	}
	for index, line := range logLines(rows) {
		want, _ := Rows(trade, line, at)
		if !reflect.DeepEqual(logs[index], want) || logs[index].Price != fill || logs[index].Type != aggragates.LOG_INFO {
			t.Errorf("log %d: got %+v, want %+v", index, logs[index], want)
		}
	}
	if st := rebuildState(aggragates.AppendStrategyRows(trade, logs, events)); !st.flatMarketActive || !st.flatMarketSaleHeld {
		t.Fatalf("the events fold the episode open and its sale held, got %+v", st)
	}
}
