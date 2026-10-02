package smarttakeloss

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The reasons the grammar tests feed the builders are data, as sophos and the
// swing decline serve them: the grammar prints what it is given, so the window
// below is a literal and none of these tests leans on a constant of the rule.
var (
	windowReasons = []string{
		"depth 1 to 5, 1h bars 2021-10-08 11:00 to 2021-10-10 21:00 UTC",
		"weight 0.40 over 0.33",
		"6 legs down of 3 needed",
		"4 lower highs of 2 needed",
		"largest leg 35% of the fall under 60%",
		"down 7.1% past 4.0%",
		"smc trend direction down 2/3 of 2 needed",
	}
	windowBreaks   = []string{"depth 1 to 5, 1h bars 2021-10-08 11:00 to 2021-10-10 21:00 UTC", "weight -0.29 under 0.33", "up 19.1% short of 4.0% down"}
	exitReasons    = []string{"leg down 7.0% from its high close", "leg 60 bars long on 1h"}
	breakReasons   = []string{"NATR(14) 1.04x its median sampled every 24 bars, over 1.00x", "2 of 4 readings hold with a smooth ladder, 3 needed"}
	windowText     = "depth 1 to 5, 1h bars 2021-10-08 11:00 to 2021-10-10 21:00 UTC, weight 0.40 over 0.33, 6 legs down of 3 needed, 4 lower highs of 2 needed, largest leg 35% of the fall under 60%, down 7.1% past 4.0%, smc trend direction down 2/3 of 2 needed"
	windowBreakTxt = "depth 1 to 5, 1h bars 2021-10-08 11:00 to 2021-10-10 21:00 UTC, weight -0.29 under 0.33, up 19.1% short of 4.0% down"
	exitText       = "leg down 7.0% from its high close, leg 60 bars long on 1h"
	breakText      = "NATR(14) 1.04x its median sampled every 24 bars, over 1.00x, 2 of 4 readings hold with a smooth ladder, 3 needed"
)

// The seven rows of the grammar, byte for byte: the swing decline triggering
// beside the indecision it latches, the indecision, the quiet decline and the
// swing decline all triggering on one row, the quiet decline alone, the
// indecision alone, a confirmation, a cancellation of both declines, and a
// reset the indecision latched on. Each is filed under the gate as the kind and
// the features it is about; none carries the hold frame.
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
			"smartTakeLoss: indecision direction and swing decline triggered (swing decline: " + windowText + ")",
			EventPending, []string{FeatureIndecision, FeatureSwingDecline}, windowReasons,
		},
		"the indecision, the quiet decline and the swing decline": {
			PendingRow(150.48, indecisionFinding(breakReasons), quietFinding(exitReasons), swingFinding(windowReasons)),
			"smartTakeLoss: indecision direction, quiet decline and swing decline triggered (indecision direction: " + breakText + "; quiet decline: " + exitText + "; swing decline: " + windowText + ")",
			EventPending, []string{FeatureIndecision, FeatureQuietDecline, FeatureSwingDecline}, concat(breakReasons, exitReasons, windowReasons),
		},
		"the quiet decline": {
			PendingRow(150.48, quietFinding(exitReasons)),
			"smartTakeLoss: quiet decline triggered (quiet decline: " + exitText + ")",
			EventPending, []string{FeatureQuietDecline}, exitReasons,
		},
		"the indecision": {
			LatchedRow(150.48, indecisionFinding(breakReasons)),
			"smartTakeLoss: indecision direction triggered (indecision direction: " + breakText + ")",
			EventLatched, []string{FeatureIndecision}, breakReasons,
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
			"smartTakeLoss: indecision direction triggered, slow decline reset, depth priority holds this ladder (indecision direction: " + breakText + ")",
			EventReset, []string{FeatureIndecision, FeatureQuietDecline}, breakReasons,
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
// swing decline names alone, groups that name the same reasons printed once
// under every label that names them, the order of the features whatever order
// the findings come in, and what a finding with no reasons or no feature at all
// prints.
func TestTheGrammarOfTheGatesRows(t *testing.T) {
	same := []string{"the same reasons", "named twice"}
	for name, tc := range map[string]struct {
		row      Row
		message  string
		features []string
	}{
		"the latch the swing decline names alone": {
			LatchedRow(150.48, swingFinding(windowReasons)),
			"smartTakeLoss: indecision direction triggered (swing decline: " + windowText + ")", []string{FeatureIndecision},
		},
		"the latch alone, no reasons": {
			LatchedRow(150.48, indecisionFinding(nil)),
			"smartTakeLoss: indecision direction triggered", []string{FeatureIndecision},
		},
		"two declines trigger": {
			PendingRow(150.48, quietFinding(exitReasons), swingFinding(windowReasons)),
			"smartTakeLoss: quiet decline and swing decline triggered (quiet decline: " + exitText + "; swing decline: " + windowText + ")", []string{FeatureQuietDecline, FeatureSwingDecline},
		},
		"the findings come in the wrong order": {
			PendingRow(150.48, swingFinding(windowReasons), quietFinding(exitReasons), indecisionFinding(breakReasons)),
			"smartTakeLoss: indecision direction, quiet decline and swing decline triggered (indecision direction: " + breakText + "; quiet decline: " + exitText + "; swing decline: " + windowText + ")",
			[]string{FeatureIndecision, FeatureQuietDecline, FeatureSwingDecline},
		},
		"the indecision and the quiet decline name the same reasons": {
			PendingRow(150.48, indecisionFinding(same), quietFinding(same)),
			"smartTakeLoss: indecision direction and quiet decline triggered (indecision direction and quiet decline: the same reasons, named twice)",
			[]string{FeatureIndecision, FeatureQuietDecline},
		},
		"all three name the same reasons": {
			PendingRow(150.48, swingFinding(same), indecisionFinding(same), quietFinding(same)),
			"smartTakeLoss: indecision direction, quiet decline and swing decline triggered (indecision direction, quiet decline and swing decline: the same reasons, named twice)",
			[]string{FeatureIndecision, FeatureQuietDecline, FeatureSwingDecline},
		},
		"two share, one differs, in the order of the first": {
			PendingRow(150.48, indecisionFinding(same), quietFinding(exitReasons), swingFinding(same)),
			"smartTakeLoss: indecision direction, quiet decline and swing decline triggered (indecision direction and swing decline: the same reasons, named twice; quiet decline: " + exitText + ")",
			[]string{FeatureIndecision, FeatureQuietDecline, FeatureSwingDecline},
		},
		"a cancellation the latch landed on, sharing reasons": {
			CancelledRow(150.48, indecisionFinding(breakReasons), quietFinding(breakReasons)),
			"smartTakeLoss: indecision direction triggered, slow decline cancelled at the new fill (indecision direction and quiet decline: " + breakText + ")",
			[]string{FeatureIndecision, FeatureQuietDecline},
		},
		"a cancellation with nothing named": {
			CancelledRow(150.48, quietFinding(nil)),
			"smartTakeLoss: slow decline cancelled at the new fill", []string{FeatureQuietDecline},
		},
		"a confirmation of two declines the latch landed on": {
			ConfirmedRow(150.48, swingFinding(windowReasons), indecisionFinding(nil), quietFinding(exitReasons)),
			"smartTakeLoss: indecision direction triggered, quiet decline and swing decline confirmed at the new fill (quiet decline: " + exitText + "; swing decline: " + windowText + ")",
			[]string{FeatureIndecision, FeatureQuietDecline, FeatureSwingDecline},
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
			"smartTakeLoss: indecision direction and quiet decline triggered (quiet decline: " + exitText + ")", []string{FeatureIndecision, FeatureQuietDecline},
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
		{[]string{FeatureIndecision, FeatureSwingDecline}, "indecision direction and swing decline"},
		{[]string{FeatureIndecision, FeatureQuietDecline, FeatureSwingDecline}, "indecision direction, quiet decline and swing decline"},
		{[]string{"unknown"}, "unknown"},
	} {
		if got := labelList(tc.features); got != tc.want {
			t.Errorf("%v: %q, want %q", tc.features, got, tc.want)
		}
	}
}

// The row of a held sale, byte for byte: the loss and the cap with two decimals,
// the words of the hold and none of a sold row's, no hold frame, filed under the
// gate as the sale held kind at the fill, naming the features it is given and no
// reasons. The cap is the package's, so the pins run under caps they name.
func TestTheHeldSaleRowReadsAsThePinnedText(t *testing.T) {
	for name, tc := range map[string]struct {
		cap      float64
		row      func() Row
		message  string
		features []string
	}{
		"the quiet decline, the shipped words": {
			2, func() Row { return SaleHeldRow(184.45, 3.1, FeatureQuietDecline) },
			"smartTakeLoss: slow decline sale held at the bollinger band, loss 3.10% of the used amount over 2.00%",
			[]string{FeatureQuietDecline},
		},
		"both declines, the loss and the cap to two decimals": {
			2.5, func() Row { return SaleHeldRow(184.45, 12.3456, FeatureQuietDecline, FeatureSwingDecline) },
			"smartTakeLoss: slow decline sale held at the bollinger band, loss 12.35% of the used amount over 2.50%",
			[]string{FeatureQuietDecline, FeatureSwingDecline},
		},
		"a small cap and a loss just over it": {
			0.75, func() Row { return SaleHeldRow(184.45, 0.8, FeatureSwingDecline) },
			"smartTakeLoss: slow decline sale held at the bollinger band, loss 0.80% of the used amount over 0.75%",
			[]string{FeatureSwingDecline},
		},
		"no feature at all": {
			2, func() Row { return SaleHeldRow(184.45, 3.1) },
			"smartTakeLoss: slow decline sale held at the bollinger band, loss 3.10% of the used amount over 2.00%",
			nil,
		},
	} {
		withLossCap(t, tc.cap)
		row := tc.row()
		if row.Message != tc.message {
			t.Errorf("%s:\n got  %q\n want %q", name, row.Message, tc.message)
		}
		if row.Gate != GateSlowDecline || row.Event != EventSaleHeld || row.Price != 184.45 || row.Reasons != nil || !reflect.DeepEqual(row.Features, tc.features) {
			t.Errorf("%s: filed %q/%q at %v with features %v and reasons %q, want slowDecline/saleHeld at 184.45 with %v and none", name, row.Gate, row.Event, row.Price, row.Features, row.Reasons, tc.features)
		}
		if strings.HasPrefix(row.Message, "smartTakeLoss: sell at ") || strings.Contains(row.Message, "Hold ") {
			t.Errorf("%s: a held sale is no sold row and no hold frame, got %q", name, row.Message)
		}
	}

	given := []string{FeatureQuietDecline}
	row := SaleHeldRow(184.45, 3.1, given...)
	given[0] = "changed"
	if row.Features[0] != FeatureQuietDecline {
		t.Fatalf("the row aliases the features it was given, got %q", row.Features)
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
			`{"event":"pending","price":150.48,"reasons":["w"],"features":["indecision","swingDecline"]}`,
		},
		"a latch": {
			LatchedRow(150.48, indecisionFinding([]string{"b"})),
			`{"event":"latched","price":150.48,"reasons":["b"],"features":["indecision"]}`,
		},
		"a latch the swing decline names": {
			LatchedRow(150.48, swingFinding([]string{"w"})),
			`{"event":"latched","price":150.48,"reasons":["w"],"features":["indecision"]}`,
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
		"a sale the loss cap holds": {
			SaleHeldRow(150.48, 3.1, FeatureQuietDecline, FeatureSwingDecline),
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
