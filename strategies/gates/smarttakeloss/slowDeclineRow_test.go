package smarttakeloss

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The reasons the grammar tests feed the builders are data, as sophos, the swing
// decline and the linear decline serve them: the grammar prints what it is given, so
// the windows below are literals and none of these tests leans on a constant of a rule.
var (
	windowReasons = []string{
		"depth 1 to 5, 1h bars 2021-10-08 11:00 to 2021-10-10 21:00 UTC",
		"high 171.4 at 2021-10-08 09:00 UTC, low 152.38 at 2021-10-10 05:00 UTC, down 11.1%",
		"3 legs down of 2 needed, each 6 bars and 5.0% down at least",
		"2 legs up of 1 needed, each 3.0% up at most, 1 over it",
		"smc trend direction down 2/3 of 2 needed",
	}
	lineReasons = []string{
		"depth 1 to 5, 1h bars 2021-10-08 11:00 to 2021-10-10 21:00 UTC",
		"slope -0.16%/h against -0.20%/h at depth 4, within 30%",
		"closes within 3.1% of the line, at most 6.0%",
		"down 9.4% past 6.0%",
		"smc trend direction down 2/3 of 2 needed",
	}
	windowBreaks   = []string{"depth 1 to 5, 1h bars 2021-10-08 11:00 to 2021-10-10 21:00 UTC", "high 171.4 at 2021-10-08 09:00 UTC, low 165.2 at 2021-10-09 20:00 UTC, down 3.6%", "0 legs down short of 2 needed, each 6 bars and 5.0% down at least"}
	exitReasons    = []string{"leg down 7.0% from its high close", "leg 60 bars long on 1h"}
	breakReasons   = []string{"NATR(14) 1.04x its median sampled every 24 bars, over 1.00x", "2 of 4 readings hold with a smooth ladder, 3 needed"}
	windowText     = "depth 1 to 5, 1h bars 2021-10-08 11:00 to 2021-10-10 21:00 UTC, high 171.4 at 2021-10-08 09:00 UTC, low 152.38 at 2021-10-10 05:00 UTC, down 11.1%, 3 legs down of 2 needed, each 6 bars and 5.0% down at least, 2 legs up of 1 needed, each 3.0% up at most, 1 over it, smc trend direction down 2/3 of 2 needed"
	lineText       = "depth 1 to 5, 1h bars 2021-10-08 11:00 to 2021-10-10 21:00 UTC, slope -0.16%/h against -0.20%/h at depth 4, within 30%, closes within 3.1% of the line, at most 6.0%, down 9.4% past 6.0%, smc trend direction down 2/3 of 2 needed"
	windowBreakTxt = "depth 1 to 5, 1h bars 2021-10-08 11:00 to 2021-10-10 21:00 UTC, high 171.4 at 2021-10-08 09:00 UTC, low 165.2 at 2021-10-09 20:00 UTC, down 3.6%, 0 legs down short of 2 needed, each 6 bars and 5.0% down at least"
	exitText       = "leg down 7.0% from its high close, leg 60 bars long on 1h"
	breakText      = "NATR(14) 1.04x its median sampled every 24 bars, over 1.00x, 2 of 4 readings hold with a smooth ladder, 3 needed"
)

// The eight rows of the grammar, byte for byte: the swing decline triggering
// beside the indecision it latches, the indecision, the quiet decline and the
// swing decline all triggering on one row, the quiet decline alone, the
// indecision alone, the same latch named by the linear decline, a confirmation, a
// cancellation of both declines, and a reset the indecision latched on. Each is
// filed under the gate as the kind and the features it is about — the linear
// decline's latch under the indecision's, the line's name in no event — and none
// carries the hold frame.
func TestTheRowsOfTheGateReadAsTheGrammarPins(t *testing.T) {
	for name, tc := range map[string]struct {
		row      Row
		message  string
		event    string
		features []string
		reasons  []string
	}{
		"the swing decline and the indecision it latches": {
			PendingRow(150.48, indecisionFinding(nil), swingFinding(windowReasons)),
			"smartTakeLoss: early take profit latched, swing decline triggered (swing decline: " + windowText + ")",
			EventPending, []string{FeatureEarlyTakeProfit, FeatureSwingDecline}, windowReasons,
		},
		"the indecision, the quiet decline and the swing decline": {
			PendingRow(150.48, indecisionFinding(breakReasons), quietFinding(exitReasons), swingFinding(windowReasons)),
			"smartTakeLoss: early take profit latched, quiet decline and swing decline triggered (indecision: " + breakText + "; quiet decline: " + exitText + "; swing decline: " + windowText + ")",
			EventPending, []string{FeatureEarlyTakeProfit, FeatureQuietDecline, FeatureSwingDecline}, concat(breakReasons, exitReasons, windowReasons),
		},
		"the quiet decline": {
			PendingRow(150.48, quietFinding(exitReasons)),
			"smartTakeLoss: quiet decline triggered (quiet decline: " + exitText + ")",
			EventPending, []string{FeatureQuietDecline}, exitReasons,
		},
		"the indecision": {
			LatchedRow(150.48, indecisionFinding(breakReasons)),
			"smartTakeLoss: early take profit latched (indecision: " + breakText + ")",
			EventLatched, []string{FeatureEarlyTakeProfit}, breakReasons,
		},
		"the indecision the linear decline latches": {
			LatchedRow(150.48, linearFinding(lineReasons)),
			"smartTakeLoss: early take profit latched (linear decline: " + lineText + ")",
			EventLatched, []string{FeatureEarlyTakeProfit}, lineReasons,
		},
		"a confirmation": {
			ConfirmedRow(150.48, swingFinding(windowReasons)),
			"smartTakeLoss: swing decline confirmed at the new fill (swing decline: " + windowText + ")",
			EventPending, []string{FeatureSwingDecline}, windowReasons,
		},
		"a cancellation of both declines": {
			CancelledRow(150.48, quietFinding(breakReasons), swingFinding(windowBreaks)),
			"smartTakeLoss: slow decline cancelled at the new fill (quiet decline: " + breakText + "; swing decline: " + windowBreakTxt + ")",
			EventCancelled, []string{FeatureQuietDecline, FeatureSwingDecline}, concat(breakReasons, windowBreaks),
		},
		"a reset the indecision latched on": {
			ResetRow(150.48, indecisionFinding(breakReasons), quietFinding(nil)),
			"smartTakeLoss: early take profit latched, slow decline reset, depth priority holds this ladder (indecision: " + breakText + ")",
			EventReset, []string{FeatureEarlyTakeProfit, FeatureQuietDecline}, breakReasons,
		},
	} {
		if tc.row.Message != tc.message {
			t.Errorf("%s:\n got  %q\n want %q", name, tc.row.Message, tc.message)
		}
		if tc.row.Gate != GateSlowDecline || tc.row.Event != tc.event || tc.row.Price != 150.48 {
			t.Errorf("%s: filed under %q/%q at %v, want the slow decline gate's %q at 150.48", name, tc.row.Gate, tc.row.Event, tc.row.Price, tc.event)
		}
		if !reflect.DeepEqual(tc.row.Features, tc.features) || !reflect.DeepEqual(tc.row.Reasons, tc.reasons) {
			t.Errorf("%s: features %v and reasons %q, want %v and %q", name, tc.row.Features, tc.row.Reasons, tc.features, tc.reasons)
		}
		if strings.Contains(tc.row.Message, "Hold ") {
			t.Errorf("%s: no row of the gate carries the hold frame, got %q", name, tc.row.Message)
		}
	}
}

// concat is the lists one after the other, as the event flattens the groups.
func concat(lists ...[]string) []string {
	var all []string
	for _, list := range lists {
		all = append(all, list...)
	}
	return all
}

// The rest of the grammar: the list of features in the head, the latch the
// swing decline names alone and the one the linear decline does, groups that name
// the same reasons printed once under every label that names them, the order of
// the features whatever order the findings come in — the linear decline's last —
// and what a finding with no reasons or no feature at all prints. A latch is one
// head, "early take profit latched", whichever readings it names, and its
// event names the indecision alone.
func TestTheGrammarOfTheGatesRows(t *testing.T) {
	same := []string{"the same reasons", "named twice"}
	for name, tc := range map[string]struct {
		row      Row
		message  string
		features []string
	}{
		"the latch the swing decline names alone": {
			LatchedRow(150.48, swingFinding(windowReasons)),
			"smartTakeLoss: early take profit latched (swing decline: " + windowText + ")", []string{FeatureEarlyTakeProfit},
		},
		"the latch the linear decline names alone": {
			LatchedRow(150.48, linearFinding(lineReasons)),
			"smartTakeLoss: early take profit latched (linear decline: " + lineText + ")", []string{FeatureEarlyTakeProfit},
		},
		"the latch the linear decline names, no reasons": {
			LatchedRow(150.48, linearFinding(nil)),
			"smartTakeLoss: early take profit latched", []string{FeatureEarlyTakeProfit},
		},
		"the latch two readings name, the line's after the swing decline's whatever the order": {
			LatchedRow(150.48, linearFinding(lineReasons), swingFinding(windowReasons)),
			"smartTakeLoss: early take profit latched (swing decline: " + windowText + "; linear decline: " + lineText + ")", []string{FeatureEarlyTakeProfit},
		},
		"the latch the swing decline and the linear decline name alike": {
			LatchedRow(150.48, linearFinding(same), swingFinding(same)),
			"smartTakeLoss: early take profit latched (swing decline and linear decline: the same reasons, named twice)", []string{FeatureEarlyTakeProfit},
		},
		"the latch alone, no reasons": {
			LatchedRow(150.48, indecisionFinding(nil)),
			"smartTakeLoss: early take profit latched", []string{FeatureEarlyTakeProfit},
		},
		"two declines trigger": {
			PendingRow(150.48, quietFinding(exitReasons), swingFinding(windowReasons)),
			"smartTakeLoss: quiet decline and swing decline triggered (quiet decline: " + exitText + "; swing decline: " + windowText + ")", []string{FeatureQuietDecline, FeatureSwingDecline},
		},
		"the findings come in the wrong order": {
			PendingRow(150.48, swingFinding(windowReasons), quietFinding(exitReasons), indecisionFinding(breakReasons)),
			"smartTakeLoss: early take profit latched, quiet decline and swing decline triggered (indecision: " + breakText + "; quiet decline: " + exitText + "; swing decline: " + windowText + ")",
			[]string{FeatureEarlyTakeProfit, FeatureQuietDecline, FeatureSwingDecline},
		},
		"the indecision and the quiet decline name the same reasons": {
			PendingRow(150.48, indecisionFinding(same), quietFinding(same)),
			"smartTakeLoss: early take profit latched, quiet decline triggered (indecision and quiet decline: the same reasons, named twice)",
			[]string{FeatureEarlyTakeProfit, FeatureQuietDecline},
		},
		"all three name the same reasons": {
			PendingRow(150.48, swingFinding(same), indecisionFinding(same), quietFinding(same)),
			"smartTakeLoss: early take profit latched, quiet decline and swing decline triggered (indecision, quiet decline and swing decline: the same reasons, named twice)",
			[]string{FeatureEarlyTakeProfit, FeatureQuietDecline, FeatureSwingDecline},
		},
		"two share, one differs, in the order of the first": {
			PendingRow(150.48, indecisionFinding(same), quietFinding(exitReasons), swingFinding(same)),
			"smartTakeLoss: early take profit latched, quiet decline and swing decline triggered (indecision and swing decline: the same reasons, named twice; quiet decline: " + exitText + ")",
			[]string{FeatureEarlyTakeProfit, FeatureQuietDecline, FeatureSwingDecline},
		},
		"a cancellation the latch landed on, sharing reasons": {
			CancelledRow(150.48, indecisionFinding(breakReasons), quietFinding(breakReasons)),
			"smartTakeLoss: early take profit latched, slow decline cancelled at the new fill (indecision and quiet decline: " + breakText + ")",
			[]string{FeatureEarlyTakeProfit, FeatureQuietDecline},
		},
		"a cancellation with nothing named": {
			CancelledRow(150.48, quietFinding(nil)),
			"smartTakeLoss: slow decline cancelled at the new fill", []string{FeatureQuietDecline},
		},
		"a confirmation of two declines the latch landed on": {
			ConfirmedRow(150.48, swingFinding(windowReasons), indecisionFinding(nil), quietFinding(exitReasons)),
			"smartTakeLoss: early take profit latched, quiet decline and swing decline confirmed at the new fill (quiet decline: " + exitText + "; swing decline: " + windowText + ")",
			[]string{FeatureEarlyTakeProfit, FeatureQuietDecline, FeatureSwingDecline},
		},
		"a confirmation of one": {
			ConfirmedRow(150.48, quietFinding(exitReasons)),
			"smartTakeLoss: quiet decline confirmed at the new fill (quiet decline: " + exitText + ")", []string{FeatureQuietDecline},
		},
		"a reset of both declines": {
			ResetRow(150.48, quietFinding(nil), swingFinding(nil)),
			"smartTakeLoss: slow decline reset, depth priority holds this ladder", []string{FeatureQuietDecline, FeatureSwingDecline},
		},
		"a finding with no reasons prints no group": {
			PendingRow(150.48, indecisionFinding(nil), quietFinding(exitReasons)),
			"smartTakeLoss: early take profit latched, quiet decline triggered (quiet decline: " + exitText + ")", []string{FeatureEarlyTakeProfit, FeatureQuietDecline},
		},
		"an unknown feature is no feature": {
			PendingRow(150.48, Finding{Feature: "unknown", Reasons: same}, quietFinding(nil)),
			"smartTakeLoss: quiet decline triggered", []string{FeatureQuietDecline},
		},
		"a feature named twice keeps the first": {
			PendingRow(150.48, quietFinding(exitReasons), quietFinding(breakReasons)),
			"smartTakeLoss: quiet decline triggered (quiet decline: " + exitText + ")", []string{FeatureQuietDecline},
		},
		"no finding at all, a trigger": {PendingRow(150.48), "smartTakeLoss: slow decline triggered", nil},
		"no finding at all, a confirmation": {
			ConfirmedRow(150.48), "smartTakeLoss: slow decline confirmed at the new fill", nil,
		},
		"no finding at all, a reset": {
			ResetRow(150.48), "smartTakeLoss: slow decline reset, depth priority holds this ladder", nil,
		},
	} {
		if tc.row.Message != tc.message {
			t.Errorf("%s:\n got  %q\n want %q", name, tc.row.Message, tc.message)
		}
		if !reflect.DeepEqual(tc.row.Features, tc.features) {
			t.Errorf("%s: features %v, want %v", name, tc.row.Features, tc.features)
		}
	}
}

// labelList lists labels as a sentence does: one, two with "and", more with
// commas and a closing "and"; none reads as the gate.
func TestLabelListReadsAsASentence(t *testing.T) {
	for _, tc := range []struct {
		features []string
		want     string
	}{
		{nil, "slow decline"},
		{[]string{FeatureQuietDecline}, "quiet decline"},
		{[]string{FeatureEarlyTakeProfit, FeatureSwingDecline}, "early take profit and swing decline"},
		{[]string{FeatureEarlyTakeProfit, FeatureQuietDecline, FeatureSwingDecline}, "early take profit, quiet decline and swing decline"},
		{[]string{FeatureLinearDecline}, "linear decline"},
		{[]string{FeatureEarlyTakeProfit, FeatureLinearDecline}, "early take profit and linear decline"},
		{[]string{FeatureEarlyTakeProfit, FeatureQuietDecline, FeatureSwingDecline, FeatureLinearDecline}, "early take profit, quiet decline, swing decline and linear decline"},
		{[]string{"unknown"}, "unknown"},
	} {
		if got := labelList(tc.features); got != tc.want {
			t.Errorf("%v: %q, want %q", tc.features, got, tc.want)
		}
	}
}

// heldText is one pin of the rows of a held sale: the shared cap and depth bound the
// row is built under, the builder, and the text and filing it must read as.
type heldText struct {
	capPct   float64
	left     int
	build    func() Row
	message  string
	gate     string
	features []string
}

// The rows of a held sale, byte for byte, in each gate's words — slow decline and
// flat market — and for each bound that holds it: the profit under the shared trigger
// (at a loss, at zero, at a profit) and the depth over the deepest depth the sale is
// made at. The profit and the trigger print signed with two decimals; the depth names the entries filled, the ceiling and that
// deepest depth, the ceiling less the depths the bound needs left, or no known
// ceiling where there is none. Each opens with the words of the hold and none of a
// sold row's, names no Bollinger band and carries no hold frame, and is filed under
// its gate as the sale held kind at the fill, naming the features it is given (the
// slow decline's) and no reasons. The trigger and the bound are the package's, so the
// pins run under values they name.
func TestTheHeldSaleRowReadsAsThePinnedText(t *testing.T) {
	const slow, low = GateSlowDecline, GateFlatMarket
	quiet, both := []string{FeatureQuietDecline}, []string{FeatureQuietDecline, FeatureSwingDecline}
	for name, tc := range map[string]heldText{
		"the slow decline over the cap, the quiet decline": {
			capPct: 2,
			build: func() Row {
				return SaleHeldRow(184.45, -3.1, FeatureQuietDecline)
			},
			message:  "smartTakeLoss: slow decline sale held, profit -3.10% of the used amount under -2.00%",
			gate:     slow,
			features: quiet,
		},
		"the slow decline over the cap, both declines, the loss and the cap to two decimals": {
			capPct: 2.5,
			build: func() Row {
				return SaleHeldRow(184.45, -12.3456, FeatureQuietDecline, FeatureSwingDecline)
			},
			message:  "smartTakeLoss: slow decline sale held, profit -12.35% of the used amount under -2.50%",
			gate:     slow,
			features: both,
		},
		"the slow decline over a small cap by a hair": {
			capPct: 0.75,
			build: func() Row {
				return SaleHeldRow(184.45, -0.8, FeatureSwingDecline)
			},
			message:  "smartTakeLoss: slow decline sale held, profit -0.80% of the used amount under -0.75%",
			gate:     slow,
			features: []string{FeatureSwingDecline},
		},
		"the slow decline over the cap, no feature at all": {
			capPct: 2,
			build: func() Row {
				return SaleHeldRow(184.45, -3.1)
			},
			message: "smartTakeLoss: slow decline sale held, profit -3.10% of the used amount under -2.00%",
			gate:    slow,
		},
		"the flat market over the cap": {
			capPct: 2,
			build: func() Row {
				return FlatMarketSaleHeldRow(184.45, -3.1)
			},
			message: "smartTakeLoss: flat market sale held, profit -3.10% of the used amount under -2.00%",
			gate:    low,
		},
		"the flat market over the cap, the loss and the cap to two decimals": {
			capPct: 0.75,
			build: func() Row {
				return FlatMarketSaleHeldRow(184.45, -12.3456)
			},
			message: "smartTakeLoss: flat market sale held, profit -12.35% of the used amount under -0.75%",
			gate:    low,
		},
		"the slow decline past the depth bound, the quiet decline": {
			left: 3,
			build: func() Row {
				return SaleHeldAtDepthRow(184.45, 6, 8, FeatureQuietDecline)
			},
			message:  "smartTakeLoss: slow decline sale held, depth 6 of 8 over 5",
			gate:     slow,
			features: quiet,
		},
		"the slow decline past the depth bound, both declines, at the bound another number says": {
			left: 1,
			build: func() Row {
				return SaleHeldAtDepthRow(184.45, 8, 8, FeatureQuietDecline, FeatureSwingDecline)
			},
			message:  "smartTakeLoss: slow decline sale held, depth 8 of 8 over 7",
			gate:     slow,
			features: both,
		},
		"the slow decline with no known ceiling": {
			left: 3,
			build: func() Row {
				return SaleHeldAtDepthRow(184.45, 6, 0, FeatureQuietDecline)
			},
			message:  "smartTakeLoss: slow decline sale held, depth 6 of no known ceiling",
			gate:     slow,
			features: quiet,
		},
		"the slow decline under the trigger at zero": {
			capPct: 0,
			build: func() Row {
				return SaleHeldRow(184.45, -0.3, FeatureQuietDecline)
			},
			message:  "smartTakeLoss: slow decline sale held, profit -0.30% of the used amount under 0.00%",
			gate:     slow,
			features: quiet,
		},
		"the flat market under a trigger at a profit": {
			capPct: -0.5,
			build: func() Row {
				return FlatMarketSaleHeldRow(184.45, 0.2)
			},
			message: "smartTakeLoss: flat market sale held, profit 0.20% of the used amount under 0.50%",
			gate:    low,
		},
		"the flat market past the depth bound": {
			left: 3,
			build: func() Row {
				return FlatMarketSaleHeldAtDepthRow(184.45, 7, 8)
			},
			message: "smartTakeLoss: flat market sale held, depth 7 of 8 over 5",
			gate:    low,
		},
		"the flat market with no known ceiling": {
			left: 3,
			build: func() Row {
				return FlatMarketSaleHeldAtDepthRow(184.45, 7, 0)
			},
			message: "smartTakeLoss: flat market sale held, depth 7 of no known ceiling",
			gate:    low,
		},
	} {
		withAllowedLoss(t, tc.capPct)
		withSellLossIgnoreLastDepths(t, tc.left)
		row := tc.build()
		if row.Message != tc.message {
			t.Errorf("%s:\n got  %q\n want %q", name, row.Message, tc.message)
		}
		if row.Gate != tc.gate || row.Event != EventSaleHeld || row.Price != 184.45 || row.Reasons != nil || !reflect.DeepEqual(row.Features, tc.features) {
			t.Errorf("%s: filed %q/%q at %v with features %v and reasons %q, want %s/saleHeld at 184.45 with %v and none", name, row.Gate, row.Event, row.Price, row.Features, row.Reasons, tc.gate, tc.features)
		}
		if strings.HasPrefix(row.Message, "smartTakeLoss: sell at ") || strings.Contains(row.Message, "Hold ") || strings.Contains(strings.ToLower(row.Message), "band") {
			t.Errorf("%s: a held sale is no sold row, no hold frame and names no band, got %q", name, row.Message)
		}
	}
}

// A held row owns the features it was given, whichever bound holds the sale: the list
// the caller passed is never aliased by the row, and a change made to it afterwards
// never reaches the row.
func TestAHeldSaleRowNeverAliasesTheFeaturesItWasGiven(t *testing.T) {
	for name, build := range map[string]func(features ...string) Row{
		"over the cap": func(features ...string) Row {
			return SaleHeldRow(184.45, -3.1, features...)
		},
		"past the depth bound": func(features ...string) Row {
			return SaleHeldAtDepthRow(184.45, 6, 8, features...)
		},
	} {
		given := []string{FeatureQuietDecline}
		row := build(given...)
		given[0] = "changed"
		if row.Features[0] != FeatureQuietDecline {
			t.Errorf("%s: the row aliases the features it was given, got %q", name, row.Features)
		}
		given[0] = FeatureQuietDecline
		row.Features[0] = "changed"
		if given[0] != FeatureQuietDecline {
			t.Errorf("%s: the caller's features were written through, got %q", name, given)
		}
	}
}

// A row owns its reasons: the lists the caller passed are never aliased by the
// row nor changed by it, and a row's own lists do not share a backing array with
// each other.
func TestARowNeverAliasesTheReasonsItWasGiven(t *testing.T) {
	given := []string{"a", "b"}
	row := PendingRow(150.48, quietFinding(given))
	row.Reasons[0] = "changed"
	if given[0] != "a" {
		t.Fatalf("the caller's reasons were written through, got %q", given)
	}
	row = PendingRow(150.48, quietFinding(given))
	given[1] = "changed"
	if row.Reasons[1] != "b" {
		t.Fatalf("the row aliases the caller's reasons, got %q", row.Reasons)
	}
}

// The event beside a row is the row's: the kind, the price, the features and
// the reasons in the document, in the keys the table has always had plus
// features after the reasons; a row with no features and no reasons writes the
// document it always did.
func TestTheEventBesideARowCarriesItsFeaturesAndReasons(t *testing.T) {
	trade := aggragates.Trades{ID: 7}
	at := testutil.At("17:45:00")
	for name, tc := range map[string]struct {
		row  Row
		want string
	}{
		"a trigger with the latch": {
			PendingRow(150.48, indecisionFinding(nil), swingFinding([]string{"w"})),
			`{"event":"pending","price":150.48,"reasons":["w"],"features":["earlyTakeProfit","swingDecline"]}`,
		},
		"a latch": {
			LatchedRow(150.48, indecisionFinding([]string{"b"})),
			`{"event":"latched","price":150.48,"reasons":["b"],"features":["earlyTakeProfit"]}`,
		},
		"a latch the swing decline names": {
			LatchedRow(150.48, swingFinding([]string{"w"})),
			`{"event":"latched","price":150.48,"reasons":["w"],"features":["earlyTakeProfit"]}`,
		},
		"a latch the linear decline names, its name in no event": {
			LatchedRow(150.48, linearFinding([]string{"l"})),
			`{"event":"latched","price":150.48,"reasons":["l"],"features":["earlyTakeProfit"]}`,
		},
		"a cancellation": {
			CancelledRow(150.48, quietFinding(nil), swingFinding([]string{"m"})),
			`{"event":"cancelled","price":150.48,"reasons":["m"],"features":["quietDecline","swingDecline"]}`,
		},
		"a reset": {
			ResetRow(150.48, swingFinding(nil)),
			`{"event":"reset","price":150.48,"features":["swingDecline"]}`,
		},
		"a row with no feature and no reason": {
			PendingRow(150.48),
			`{"event":"pending","price":150.48}`,
		},
		"a sale the loss cap holds, naming its trigger": {
			SaleHeldRow(150.48, -3.1, FeatureQuietDecline, FeatureSwingDecline),
			`{"event":"saleHeld","price":150.48,"features":["quietDecline","swingDecline"],"trigger":` + strconv.FormatFloat(FullTrendTrigger(), 'f', -1, 64) + `}`,
		},
		"a sale the depth bound holds, naming no trigger": {
			SaleHeldAtDepthRow(150.48, 7, 8, FeatureQuietDecline, FeatureSwingDecline),
			`{"event":"saleHeld","price":150.48,"features":["quietDecline","swingDecline"]}`,
		},
	} {
		logRow, event := Rows(trade, tc.row, at)
		if string(event.Data) != tc.want || event.Gate != GateSlowDecline || event.Param != aggragates.StrategyParamSmartTakeLoss || !event.CreatedAt.Equal(at) {
			t.Errorf("%s: event %s under %q/%q, want %s", name, event.Data, event.Param, event.Gate, tc.want)
		}
		if logRow.Message != tc.row.Message || logRow.Price != 150.48 || logRow.Type != aggragates.LOG_INFO {
			t.Errorf("%s: the row beside it, got %+v", name, logRow)
		}
		var data EventData
		if err := json.Unmarshal(event.Data, &data); err != nil || !reflect.DeepEqual(data.Features, tc.row.Features) || data.Event != tc.row.Event {
			t.Errorf("%s: the document decodes to %+v (%v)", name, data, err)
		}
	}
}
