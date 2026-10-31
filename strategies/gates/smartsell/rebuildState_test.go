package smartsell

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown"
	"github.com/giovani-sirbu/mercury/strategies/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/earlytakeprofit"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/quietdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// rebuildState reads the trade's smartSell events in slice order and nothing else: never a log
// row's text, never a stamp; legacy events fold in rebuildStateLegacy_test.go.

// retiredRows are the rows older releases wrote for the retired trend-reversal rule, text with no event, read as nothing.
func retiredRows(price float64) []aggragates.TradesLogs {
	return []aggragates.TradesLogs{
		{Message: "Hold buy: smartSell: Potential trend reversal", Price: price, Type: aggragates.LOG_INFO},
		{Message: "Hold trailingBuy: smartSell: last fill too fresh to sell, no add (0s)", Price: price, Type: aggragates.LOG_INFO},
	}
}

// step is one thing that happens to a ladder's record: a fixture is a ladder
// and the steps written to it, in order.
type step func(aggragates.Trades) aggragates.Trades

// writes is the step of an engine writing the row: the log row and its event.
func writes(row Row) step {
	return func(trade aggragates.Trades) aggragates.Trades {
		return withRow(trade, row, time.Time{})
	}
}

// records is the step of an event alone joining the record.
func records(events ...aggragates.TradesStrategyEvents) step {
	return func(trade aggragates.Trades) aggragates.Trades {
		return withEvents(trade, events...)
	}
}

// elsewhere is the step of a gate the smart take loss does not fold speaking:
// the cooldown depth priority's pair, unstamped so that no hold starts.
func elsewhere(trade aggragates.Trades) aggragates.Trades {
	return heldBy(trade, time.Time{})
}

// withSteps is the ladder with the steps applied to it, in order.
func withSteps(trade aggragates.Trades, steps ...step) aggragates.Trades {
	for _, apply := range steps {
		trade = apply(trade)
	}
	return trade
}

// sixDeep is the w3s ladder six deep, watched by every feature the fold reads.
func sixDeep() aggragates.Trades {
	return testutil.LadderTrade(false, fills(6, "23:09:00")...)
}

// Without events a watched ladder is not pending, and its newest fill is the last in slice order whatever the stamps.
func TestRebuildStateWithoutEvents(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(5, "17:38:00")...)
	trade.History[4].CreatedAt = testutil.At("07:00:00")

	st := rebuildState(trade)
	if !st.slowDeclineWatched || !st.quietDeclineWatched || st.slowDeclinePending || st.slowDeclinePendingFrom != 0 || len(st.pendingFeatures) != 0 || st.capitalProtectionWatched {
		t.Fatalf("no event must mean nothing pending, got %+v", st)
	}
	if !st.earlyTakeProfitWatched || st.earlyTakeProfit {
		t.Fatalf("no event must mean nothing latched on a watched ladder, got %+v", st)
	}
	if len(st.fills) != 5 || st.lastFill().Price != 179.78 {
		t.Fatalf("5 fills and the last fill is 179.78, got %+v", st)
	}
}

// A pending event makes a watched ladder pending from the fill its price names on the features it
// names, never an unwatched ladder; the first-fill hold's held event is no pending event.
func TestRebuildStateFoldsThePendingEvent(t *testing.T) {
	trade := watchedTrade()
	if st := rebuildState(trade); !st.slowDeclineWatched || st.slowDeclinePending {
		t.Fatalf("a watched ladder without the event is not pending, got %+v", st)
	}

	framed := withSteps(watchedTrade(), elsewhere, writes(PendingRow(slowDeclineLastFill, quietFinding(nil))))
	if st := rebuildState(framed); !st.slowDeclinePending || st.slowDeclinePendingFrom != slowDeclineLastFill || !reflect.DeepEqual(st.pendingFeatures, []string{FeatureQuietDecline}) {
		t.Fatalf("the pending event must make the ladder pending from its fill on the quiet decline, got %+v", st)
	}

	entryHold := EntryHold(trade, aggragates.SideLong, slowDeclineVerdict(true))
	held := withEvents(trade, aggragates.NewStrategyEvent(trade.ID, entryHold.Param, entryHold.Gate, entryHold.Data, time.Time{}))
	if st := rebuildState(held); st.slowDeclinePending {
		t.Fatalf("the first-fill hold's event is not a pending event, got %+v", st)
	}

	marker := PendingRow(slowDeclineLastFill, quietFinding(nil))
	inverse := withRows(testutil.LadderTrade(true, fills(quietdecline.ArmDepth, "17:38:00")...), marker)
	noFill := withRows(testutil.LadderTrade(false), marker)
	shallow := withRows(testutil.LadderTrade(false, fills(quietdecline.ArmDepth-1, "17:38:00")...), marker)
	for _, unwatched := range []aggragates.Trades{inverse, noFill, shallow} {
		if st := rebuildState(unwatched); st.slowDeclineWatched || st.slowDeclinePending {
			t.Fatalf("a ladder the gate does not watch is never pending, got %+v", st)
		}
	}

	// quietdecline.ArmDepth filled entries are enough to be watched and pending, as quietDeclineWatched reads.
	shallowest := withRows(testutil.LadderTrade(false, fills(quietdecline.ArmDepth, "17:38:00")...), marker)
	if st := rebuildState(shallowest); !st.quietDeclineWatched || !st.slowDeclinePending || !quietDeclineWatched(shallowest) || !slowDeclineWatched(shallowest) {
		t.Fatalf("a long ladder is watched from quietdecline.ArmDepth, got %+v", st)
	}
	if quietDeclineWatched(shallow) {
		t.Fatal("quietDeclineWatched must agree with rebuildState on the shallower ladder")
	}
}

// The gate's events fold in slice order and the last one wins: pending makes pending, cancelled or
// reset makes not pending; other gates, the sale and priceless events change nothing.
func TestRebuildStateFoldsTheSlowDeclineEventsInOrder(t *testing.T) {
	withQuietDecline(t, true)
	pending := func(price float64) step { return writes(PendingRow(price, quietFinding(slowDeclineReasons))) }
	cancelled := func(price float64) step { return writes(CancelledRow(price, quietFinding(quietDeclineBreakReasons))) }
	reset := func(price float64) step { return writes(ResetRow(price, quietFinding(nil))) }
	sold := func(reason string) step { return writes(ExitRow(sixDeep(), reason, slowDeclineBand)) }
	cancelledWithoutAPrice := records(eventOf(GateSlowDecline, `{"event":"cancelled","features":["quietDecline"]}`))
	pendingWithoutAPrice := records(eventOf(GateSlowDecline, `{"event":"pending","features":["quietDecline"]}`))

	for name, tc := range map[string]struct {
		steps   []step
		pending bool
		from    float64
	}{
		"a pending event":                               {[]step{pending(pendingFromFifth)}, true, pendingFromFifth},
		"pending, cancelled":                            {[]step{pending(pendingFromFifth), cancelled(sixthFill)}, false, 0},
		"pending, cancelled, pending":                   {[]step{pending(pendingFromFifth), cancelled(sixthFill), pending(sixthFill)}, true, sixthFill},
		"pending, pending":                              {[]step{pending(pendingFromFifth), pending(sixthFill)}, true, sixthFill},
		"cancelled, pending":                            {[]step{cancelled(sixthFill), pending(pendingFromFifth)}, true, pendingFromFifth},
		"a cancelled alone":                             {[]step{cancelled(sixthFill)}, false, 0},
		"pending, reset":                                {[]step{pending(pendingFromFifth), reset(pendingFromFifth)}, false, 0},
		"pending, reset, pending":                       {[]step{pending(pendingFromFifth), reset(pendingFromFifth), pending(sixthFill)}, true, sixthFill},
		"a reset alone":                                 {[]step{reset(sixthFill)}, false, 0},
		"pending, sold":                                 {[]step{pending(pendingFromFifth), sold(reasonSlowDecline)}, true, pendingFromFifth},
		"pending, capital protection's sold":            {[]step{pending(pendingFromFifth), sold(reasonCapitalProtection)}, true, pendingFromFifth},
		"a sold alone":                                  {[]step{sold(reasonSlowDecline)}, false, 0},
		"pending, cancelled, sold":                      {[]step{pending(pendingFromFifth), cancelled(sixthFill), sold(reasonSlowDecline)}, false, 0},
		"pending, elsewhere, cancelled, elsewhere":      {[]step{pending(pendingFromFifth), elsewhere, cancelled(sixthFill), elsewhere}, false, 0},
		"pending, a cancelled without a price":          {[]step{pending(pendingFromFifth), cancelledWithoutAPrice}, true, pendingFromFifth},
		"pending, cancelled, a pending without a price": {[]step{pending(pendingFromFifth), cancelled(sixthFill), pendingWithoutAPrice}, false, 0},
		"pending, cancelled, pending, cancelled, pending": {
			[]step{pending(pendingFromFifth), cancelled(sixthFill), pending(sixthFill), cancelled(sixthFill), pending(sixthFill)}, true, sixthFill,
		},
	} {
		st := rebuildState(withSteps(sixDeep(), tc.steps...))
		if st.slowDeclinePending != tc.pending || st.slowDeclinePendingFrom != tc.from {
			t.Errorf("%s: pending %v from %v, want %v from %v", name, st.slowDeclinePending, st.slowDeclinePendingFrom, tc.pending, tc.from)
		}

		shallow := testutil.LadderTrade(false, fills(quietdecline.ArmDepth-1, "17:38:00")...)
		if st := rebuildState(withSteps(shallow, tc.steps...)); st.slowDeclinePending || st.slowDeclinePendingFrom != 0 {
			t.Errorf("%s: a ladder the gate does not watch is never pending, got %+v", name, st)
		}
	}
}

// A pending event stands for the features it names; a feature that does not watch the ladder is
// ignored, and the features are the last pending event's, never the union.
func TestRebuildStateReadsThePendingFeaturesOfTheLastPendingEvent(t *testing.T) {
	pending := func(findings ...Finding) step { return writes(PendingRow(pendingFromFifth, findings...)) }
	for name, tc := range map[string]struct {
		steps []step
		on    []string
	}{
		"the quiet decline":                    {[]step{pending(quietFinding(nil))}, []string{FeatureQuietDecline}},
		"the swing decline":                    {[]step{pending(swingFinding(nil))}, []string{FeatureSwingDecline}},
		"both, in the order the rows name":     {[]step{pending(swingFinding(nil), quietFinding(nil))}, []string{FeatureQuietDecline, FeatureSwingDecline}},
		"the latch beside the swing decline":   {[]step{pending(indecisionFinding(nil), swingFinding(nil))}, []string{FeatureSwingDecline}},
		"the latch alone makes nothing":        {[]step{pending(indecisionFinding(nil))}, nil},
		"both, then the quiet decline alone":   {[]step{pending(quietFinding(nil), swingFinding(nil)), pending(quietFinding(nil))}, []string{FeatureQuietDecline}},
		"the quiet decline, then the swing":    {[]step{pending(quietFinding(nil)), pending(swingFinding(nil))}, []string{FeatureSwingDecline}},
		"an unknown feature beside the swing":  {[]step{writes(Row{Message: "x", Price: pendingFromFifth, Gate: GateSlowDecline, Event: EventPending, Features: []string{"unknown", FeatureSwingDecline}})}, []string{FeatureSwingDecline}},
		"an unknown feature alone":             {[]step{writes(Row{Message: "x", Price: pendingFromFifth, Gate: GateSlowDecline, Event: EventPending, Features: []string{"unknown"}})}, nil},
		"cancelled after both":                 {[]step{pending(quietFinding(nil), swingFinding(nil)), writes(CancelledRow(sixthFill, quietFinding(nil)))}, nil},
		"reset after the swing decline":        {[]step{pending(swingFinding(nil)), writes(ResetRow(pendingFromFifth, swingFinding(nil)))}, nil},
		"pending again after the reset, quiet": {[]step{pending(swingFinding(nil)), writes(ResetRow(pendingFromFifth, swingFinding(nil))), pending(quietFinding(nil))}, []string{FeatureQuietDecline}},
	} {
		st := rebuildState(withSteps(sixDeep(), tc.steps...))
		if st.slowDeclinePending != (len(tc.on) > 0) || !(len(st.pendingFeatures) == 0 && len(tc.on) == 0 || reflect.DeepEqual(st.pendingFeatures, tc.on)) {
			t.Errorf("%s: pending on %v (pending %v), want %v", name, st.pendingFeatures, st.slowDeclinePending, tc.on)
		}
	}

	withSwingDecline(t, false)
	onlySwing := withSteps(sixDeep(), writes(PendingRow(pendingFromFifth, swingFinding(nil))))
	if st := rebuildState(onlySwing); st.slowDeclinePending || len(st.pendingFeatures) != 0 {
		t.Errorf("switched off, the swing decline's events are ignored, got %+v", st)
	}
	both := withSteps(sixDeep(), writes(PendingRow(pendingFromFifth, quietFinding(nil), swingFinding(nil))))
	if st := rebuildState(both); !st.slowDeclinePending || !reflect.DeepEqual(st.pendingFeatures, []string{FeatureQuietDecline}) {
		t.Errorf("switched off, a ladder pending on both stays pending on the quiet decline alone, got %+v", st)
	}
	withSwingDecline(t, true)
	withQuietDecline(t, false)
	if st := rebuildState(both); !st.slowDeclinePending || !reflect.DeepEqual(st.pendingFeatures, []string{FeatureSwingDecline}) {
		t.Errorf("the quiet decline switched off, the same ladder stays pending on the swing decline alone, got %+v", st)
	}
}

// The fold reads the slice order and never the stamps: a later event with an earlier stamp still wins.
func TestRebuildStateFoldsInSliceOrderNotStampOrder(t *testing.T) {
	early, late := testutil.At("09:00:00"), testutil.At("21:00:00")
	pending := PendingRow(pendingFromFifth, quietFinding(nil))
	cancelled := CancelledRow(sixthFill, quietFinding(nil))

	cancelsInSliceOrder := withRow(withRow(sixDeep(), pending, late), cancelled, early)
	if st := rebuildState(cancelsInSliceOrder); st.slowDeclinePending {
		t.Fatalf("the cancelled event is last in the slice, whatever its stamp, got %+v", st)
	}
	pendsInSliceOrder := withRow(withRow(sixDeep(), cancelled, late), pending, early)
	if st := rebuildState(pendsInSliceOrder); !st.slowDeclinePending || st.slowDeclinePendingFrom != pendingFromFifth {
		t.Fatalf("the pending event is last in the slice, whatever its stamp, got %+v", st)
	}
	sameStamp := withRow(withRow(sixDeep(), pending, late), cancelled, late)
	if st := rebuildState(sameStamp); st.slowDeclinePending {
		t.Fatalf("events of one stamp fold in slice order, got %+v", st)
	}
}

// The sale changes nothing: a pending ladder whose exit sold reads pending from the same fill.
func TestRebuildStateSoldChangesNothing(t *testing.T) {
	withQuietDecline(t, true)
	for _, reason := range []string{reasonSlowDecline, reasonCapitalProtection} {
		for name, trade := range map[string]aggragates.Trades{"pending": pendingAtFive(), "not pending": watchedTrade(), "last depth": lastDepthLadder()} {
			sold := withRow(trade, ExitRow(trade, reason, slowDeclineBand), testutil.At("23:30:00"))
			if got, want := rebuildState(sold), rebuildState(trade); !reflect.DeepEqual(got, want) {
				t.Errorf("%s, sold by %q: the state moved to %+v, want %+v", name, reason, got, want)
			}
		}
	}
}

// The shared action's memory in the slow decline gate's events: a sale held event sets saleHeld, an
// activated one slowDeclineActivated, each alone and leaving the ladder pending; a new pending,
// cancelled, reset or sold event clears both. Documents are read as Postgres renders them; one with
// no feature, no price or another gate is nothing, and the flat market activation moves neither flag.
func TestRebuildStateFoldsTheSlowDeclinesSaleHeldAndActivatedEvents(t *testing.T) {
	withQuietDecline(t, true)
	pending := writes(quietPending(pendingFromFifth))
	document := func(gate, kind, rest string) step {
		return records(eventOf(gate, `{"event": "`+kind+`", `+rest+`}`))
	}
	const named = `"price": 179.78, "features": ["quietDecline"]`
	held, activated := document(GateSlowDecline, EventSaleHeld, named), document(GateSlowDecline, EventActivated, named)
	confirmed := writes(ConfirmedRow(sixthFill, quietFinding(nil)))
	cancelled := writes(CancelledRow(sixthFill, quietFinding(nil)))
	reset := writes(ResetRow(pendingFromFifth, quietFinding(nil)))
	sold := writes(ExitRow(withSteps(sixDeep(), pending), reasonSlowDecline, slowDeclineBand))

	for name, tc := range map[string]struct {
		steps                       []step
		pending                     bool
		from                        float64
		saleHeld, slowDeclineActive bool
	}{
		"pending":                                   {[]step{pending}, true, pendingFromFifth, false, false},
		"pending, sale held":                        {[]step{pending, held}, true, pendingFromFifth, true, false},
		"pending, activated":                        {[]step{pending, activated}, true, pendingFromFifth, false, true},
		"pending, sale held, activated":             {[]step{pending, held, activated}, true, pendingFromFifth, true, true},
		"pending, activated, sale held":             {[]step{pending, activated, held}, true, pendingFromFifth, true, true},
		"pending, held, activated, confirmed":       {[]step{pending, held, activated, confirmed}, true, sixthFill, false, false},
		"pending, held, activated, pending again":   {[]step{pending, held, activated, pending}, true, pendingFromFifth, false, false},
		"pending, held, activated, cancelled":       {[]step{pending, held, activated, cancelled}, false, 0, false, false},
		"pending, held, activated, reset":           {[]step{pending, held, activated, reset}, false, 0, false, false},
		"pending, held, activated, sold":            {[]step{pending, held, activated, sold}, true, pendingFromFifth, false, false},
		"pending, held, activated, sold, held":      {[]step{pending, held, activated, sold, held}, true, pendingFromFifth, true, false},
		"pending, the flat market gate's activated": {[]step{pending, writes(FlatMarketActivatedRow(pendingFromFifth, -0.25))}, true, pendingFromFifth, false, false},
		"pending, a held sale with no feature":      {[]step{pending, document(GateSlowDecline, EventSaleHeld, `"price": 179.78`)}, true, pendingFromFifth, false, false},
		"pending, an activation with no feature":    {[]step{pending, document(GateSlowDecline, EventActivated, `"price": 179.78`)}, true, pendingFromFifth, false, false},
		"pending, a held sale with no price":        {[]step{pending, document(GateSlowDecline, EventSaleHeld, `"features": ["quietDecline"]`)}, true, pendingFromFifth, false, false},
		"pending, an activation at a zero price":    {[]step{pending, document(GateSlowDecline, EventActivated, `"price": 0, "features": ["quietDecline"]`)}, true, pendingFromFifth, false, false},
		"pending, an activation under another gate": {[]step{pending, document(GateCapitalProtection, EventActivated, named)}, true, pendingFromFifth, false, false},
	} {
		st := rebuildState(withSteps(sixDeep(), tc.steps...))
		if st.slowDeclinePending != tc.pending || st.slowDeclinePendingFrom != tc.from || st.saleHeld != tc.saleHeld || st.slowDeclineActivated != tc.slowDeclineActive {
			t.Errorf("%s: pending %v from %v, sale held %v, trailingBuy activated %v; want %v from %v, %v, %v",
				name, st.slowDeclinePending, st.slowDeclinePendingFrom, st.saleHeld, st.slowDeclineActivated, tc.pending, tc.from, tc.saleHeld, tc.slowDeclineActive)
		}
	}
}

// The slow decline gate's gap stamp: a gapHeld event stamps the gap at the event's own clock and a
// gapClosed one zeroes it, a gap after a close stamps a new episode; a held row leaves the stamp, and
// an activation, a new pending, a cancelled, a reset or a sold event zeroes it. A document with no
// feature or no price, or one filed under another gate, is nothing.
func TestRebuildStateFoldsTheSlowDeclinesGapStamp(t *testing.T) {
	withQuietDecline(t, true)
	at := func(second int) time.Time { return confirmStart.Add(time.Duration(second) * time.Second) }
	row := func(r Row, second int) step {
		return func(trade aggragates.Trades) aggragates.Trades { return withRow(trade, r, at(second)) }
	}
	document := func(gate, rest string, second int) step {
		event := eventOf(gate, `{`+rest+`}`)
		event.CreatedAt = at(second)
		return records(event)
	}
	const fill = pendingFromFifth
	pending := row(quietPending(fill), 0)
	held := func(second int) step { return row(SaleHeldRow(fill, -5, FeatureQuietDecline), second) }
	activated := func(second int) step { return row(ActivatedRow(fill, -5, FeatureQuietDecline), second) }
	gap := func(second int) step {
		return row(SaleHeldByGapRow(GateSlowDecline, fill, 10, 0.5, 5, FeatureQuietDecline), second)
	}
	closed := func(second int) step {
		return row(GapClosedRow(GateSlowDecline, fill, 0.5, FeatureQuietDecline), second)
	}
	opened := []step{pending, held(1), activated(2), gap(3)}
	none := time.Time{}

	for name, tc := range map[string]struct {
		steps   []step
		want    time.Time
		pending bool
	}{
		"pending, held, activated, gap":             {opened, at(3), true},
		"a held row after the gap leaves the stamp": {append(slices.Clone(opened), held(4)), at(3), true},
		"gap, closed":                                          {append(slices.Clone(opened), closed(4)), none, true},
		"gap, closed, gap again":                               {append(slices.Clone(opened), closed(4), gap(6)), at(6), true},
		"gap, closed, gap again, closed":                       {append(slices.Clone(opened), closed(4), gap(6), closed(7)), none, true},
		"a close with no gap open":                             {[]step{pending, held(1), activated(2), closed(3)}, none, true},
		"gap, activated again":                                 {append(slices.Clone(opened), activated(4)), none, true},
		"gap, pending again":                                   {append(slices.Clone(opened), pending), none, true},
		"gap, confirmed":                                       {append(slices.Clone(opened), row(ConfirmedRow(sixthFill, quietFinding(nil)), 4)), none, true},
		"gap, cancelled":                                       {append(slices.Clone(opened), row(CancelledRow(sixthFill, quietFinding(nil)), 4)), none, false},
		"gap, reset":                                           {append(slices.Clone(opened), row(ResetRow(fill, quietFinding(nil)), 4)), none, false},
		"gap, sold":                                            {append(slices.Clone(opened), row(ExitRow(withSteps(sixDeep(), pending), reasonSlowDecline, slowDeclineBand), 4)), none, true},
		"another gate's gap leaves the stamp":                  {append(slices.Clone(opened), row(SaleHeldByGapRow(GateCapitalProtection, fill, 10, 0.5, 5), 4)), at(3), true},
		"another gate's close leaves the stamp":                {append(slices.Clone(opened), row(GapClosedRow(GateCapitalProtection, fill, 0.5), 4)), at(3), true},
		"a gap with no feature is nothing":                     {[]step{pending, held(1), activated(2), document(GateSlowDecline, `"event":"gapHeld","price":179.78`, 3)}, none, true},
		"a gap with no price is nothing":                       {[]step{pending, held(1), activated(2), document(GateSlowDecline, `"event":"gapHeld","features":["quietDecline"]`, 3)}, none, true},
		"a gap at a zero price is nothing":                     {[]step{pending, held(1), activated(2), document(GateSlowDecline, `"event":"gapHeld","price":0,"features":["quietDecline"]`, 3)}, none, true},
		"a close with no feature leaves the stamp":             {append(slices.Clone(opened), document(GateSlowDecline, `"event":"gapClosed","price":179.78`, 4)), at(3), true},
		"a close with no price leaves the stamp":               {append(slices.Clone(opened), document(GateSlowDecline, `"event":"gapClosed","features":["quietDecline"]`, 4)), at(3), true},
		"a gap under the flat market gate leaves it untouched": {[]step{pending, held(1), activated(2), row(SaleHeldByGapRow(GateFlatMarket, fill, 10, 0.5, 5), 3)}, none, true},
	} {
		st := rebuildState(withSteps(sixDeep(), tc.steps...))
		if !st.slowDeclineGapHeldAt.Equal(tc.want) || st.slowDeclinePending != tc.pending {
			t.Errorf("%s: gap stamp %v, pending %v; want %v, %v", name, st.slowDeclineGapHeldAt, st.slowDeclinePending, tc.want, tc.pending)
		}
	}
}

// Every event of another gate or another param changes nothing, each unstamped so none holds a depth priority.
func TestRebuildStateIgnoresTheOtherGatesEvents(t *testing.T) {
	other := []aggragates.TradesStrategyEvents{
		cooldown.NewFirstFillEvent(45211, cooldown.FirstFillEvent{Event: cooldown.FirstFillActivated, Price: 190, Reference: 190}, time.Time{}),
		cooldown.NewFirstFillEvent(45211, cooldown.FirstFillEvent{Event: cooldown.FirstFillEntered, Price: 191, Reference: 190}, time.Time{}),
		cooldown.NewDepthSpacingEvent(45211, cooldown.DepthSpacingEvent{Event: gates.EventHeld, Depth: 4, Step: 3, Hold: time.Hour}, time.Time{}),
		cooldown.NewDepthPriorityEvent(45211, cooldown.DepthPriorityEvent{Event: gates.EventHeld, PrioritySymbol: "ETH/USDT", PriorityDepth: 5, PriorityMaxDepth: 8, Depth: 4, MaxDepth: 8}, time.Time{}),
		aggragates.NewStrategyEvent(45211, aggragates.StrategyParamSmartSell, GateEntryHold, EventData{Event: gates.EventHeld}, time.Time{}),
		aggragates.NewStrategyEvent(45211, aggragates.StrategyParamSmartSell, GateCapitalProtection, EventData{Event: EventSold, Price: lastDepthPrint}, time.Time{}),
		aggragates.NewStrategyEvent(45211, aggragates.StrategyParamDynamicParams, dynamicparams.GateOpened, dynamicparams.OpenedEvent{Event: dynamicparams.EventOpened, Points: 0.4, Depths: 1}, time.Time{}),
		aggragates.NewStrategyEvent(45211, aggragates.StrategyParamCooldown, GateSlowDecline, EventData{Event: EventCancelled, Price: sixthFill, Features: []string{FeatureQuietDecline}}, time.Time{}),
		aggragates.NewStrategyEvent(45211, aggragates.StrategyParamDynamicParams, GateSlowDecline, EventData{Event: EventReset, Price: sixthFill, Features: []string{FeatureQuietDecline}}, time.Time{}),
		aggragates.NewStrategyEvent(45211, aggragates.StrategyParamCooldown, gateIndecision, EventData{Event: EventLatched, Price: sixthFill}, time.Time{}),
		aggragates.NewStrategyEvent(45211, aggragates.StrategyParamCooldown, GateSlowDecline, EventData{Event: EventLatched, Price: sixthFill, Features: []string{FeatureEarlyTakeProfit}}, time.Time{}),
	}
	for name, trade := range map[string]aggragates.Trades{"pending": pendingAtFive(), "not pending": watchedTrade(), "latched": latchedBy(indecisionLadder())} {
		with := withEvents(trade, other...)
		if got, want := rebuildState(with), rebuildState(trade); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: another gate's events moved the state to %+v, want %+v", name, got, want)
		}
	}
}

// While quietdecline.Enabled is off the feature watches no ladder, so its pending events make nothing pending.
func TestRebuildStateIgnoresTheQuietDeclineEventsWhileSwitchedOff(t *testing.T) {
	withQuietDecline(t, false)
	if st := rebuildState(pendingTrade()); st.quietDeclineWatched || st.slowDeclinePending || st.slowDeclinePendingFrom != 0 {
		t.Fatalf("switched off, the pending event must be ignored, got %+v", st)
	}
	withSwingDecline(t, false)
	if st := rebuildState(pendingTrade()); st.slowDeclineWatched || st.slowDeclinePending {
		t.Fatalf("with both declines switched off the gate watches no ladder, got %+v", st)
	}
}

// An event carrying the indecision latches a watched ladder (latched, pending, cancelled and reset
// kinds alike) and nothing takes the latch away; it latches no unwatched ladder and is ignored while
// earlytakeprofit.Enabled is off. The pending fold reads the same events on its own.
func TestRebuildStateLatchesOnTheEventsThatNameTheEarlyTakeProfit(t *testing.T) {
	latch := writes(LatchedRow(sixthFill, indecisionFinding(indecisionReasons)))
	latchWithoutAPrice := records(eventOf(GateSlowDecline, `{"event":"latched","features":["indecision"]}`), eventOf(gateIndecision, `{"event":"latched"}`))
	latchWithoutFeatures := records(eventOf(GateSlowDecline, legacyDocument(EventLatched, sixthFill)))
	marker := writes(PendingRow(pendingFromFifth, quietFinding(slowDeclineReasons)))
	cancel := writes(CancelledRow(sixthFill, quietFinding(quietDeclineBreakReasons)))
	latchingPending := writes(PendingRow(pendingFromFifth, indecisionFinding(nil), quietFinding(nil)))
	latchingCancel := writes(CancelledRow(sixthFill, indecisionFinding(indecisionReasons), quietFinding(indecisionReasons)))
	latchingReset := writes(ResetRow(sixthFill, indecisionFinding(indecisionReasons), quietFinding(nil)))
	for name, tc := range map[string]struct {
		steps            []step
		latched, pending bool
	}{
		"no event":                                  {nil, false, false},
		"a latched event":                           {[]step{latch}, true, false},
		"a latched event without a price":           {[]step{latchWithoutAPrice}, true, false},
		"a latched kind without features":           {[]step{latchWithoutFeatures}, false, false},
		"the latch, then a marker and its cancel":   {[]step{latch, marker, cancel}, true, false},
		"a marker, then the latch":                  {[]step{marker, latch}, true, true},
		"the latch among other gates' events":       {[]step{elsewhere, latch, elsewhere}, true, false},
		"a cancel, then the latch, then a marker":   {[]step{cancel, latch, marker}, true, true},
		"a pending row that latched":                {[]step{latchingPending}, true, true},
		"a cancelled row that latched":              {[]step{marker, latchingCancel}, true, false},
		"a reset row that latched":                  {[]step{marker, latchingReset}, true, false},
		"a latched pending row, then the cancel":    {[]step{latchingPending, cancel}, true, false},
		"a latched pending row, then another":       {[]step{latchingPending, marker}, true, true},
		"the legacy gate's latched event":           {[]step{records(eventOf(gateIndecision, legacyDocument(EventLatched, sixthFill)))}, true, false},
		"the legacy gate's latched, a quiet cancel": {[]step{records(eventOf(gateIndecision, legacyDocument(EventLatched, sixthFill))), cancel}, true, false},
	} {
		trade := withSteps(sixDeep(), tc.steps...)
		st := rebuildState(trade)
		if !st.earlyTakeProfitWatched || st.earlyTakeProfit != tc.latched || st.slowDeclinePending != tc.pending {
			t.Errorf("%s: latched %v pending %v, want %v and %v (%+v)", name, st.earlyTakeProfit, st.slowDeclinePending, tc.latched, tc.pending, st)
		}
	}

	latchedAtTheFill := LatchedRow(slowDeclineLastFill, indecisionFinding(nil))
	inverse := testutil.LadderTrade(true, fills(watchedFills, "17:38:00")...)
	futures := watchedTrade()
	futures.Strategy.TradeType = aggragates.Futures
	unwatched := map[string]aggragates.Trades{"an inverse ladder": inverse, "a futures ladder": futures}
	// At an arm depth of zero the latch watches an unfilled ladder too (TestAnUnfilledLadderIsLatchedOnce).
	if earlytakeprofit.ArmDepth > 0 {
		unwatched["one short of earlytakeprofit.ArmDepth"] = testutil.LadderTrade(false, fills(earlytakeprofit.ArmDepth-1, "17:38:00")...)
		unwatched["a ladder with no fill"] = testutil.LadderTrade(false)
	}
	for name, trade := range unwatched {
		trade = withRows(trade, latchedAtTheFill, PendingRow(slowDeclineLastFill, indecisionFinding(nil), quietFinding(nil)))
		if st := rebuildState(trade); st.earlyTakeProfitWatched || st.earlyTakeProfit || earlyTakeProfitWatched(trade) {
			t.Errorf("%s: a ladder the rule does not watch is never latched, got %+v", name, st)
		}
	}
	atDepth := withRows(testutil.LadderTrade(false, fills(earlytakeprofit.ArmDepth, "17:38:00")...), latchedAtTheFill)
	if st := rebuildState(atDepth); !st.earlyTakeProfitWatched || !st.earlyTakeProfit || !earlyTakeProfitWatched(atDepth) {
		t.Fatalf("a long spot ladder is watched and latched from earlytakeprofit.ArmDepth, exactly as earlyTakeProfitWatched reads it, got %+v", st)
	}

	withEarlyTakeProfit(t, false)
	if st := rebuildState(atDepth); st.earlyTakeProfitWatched || st.earlyTakeProfit {
		t.Fatalf("switched off, the latched event must be ignored, got %+v", st)
	}
}

// The features fold each on its own watch: a ladder only the early take profit watches is latched
// all the same, and a futures ladder is neither pending nor latched.
func TestRebuildStateFoldsEachFeatureApart(t *testing.T) {
	withEarlyTakeProfit(t, true)
	events := []Row{
		PendingRow(slowDeclineLastFill, quietFinding(nil)),
		LatchedRow(slowDeclineLastFill, indecisionFinding(nil)),
	}
	futures := watchedTrade()
	futures.Strategy.TradeType = aggragates.Futures
	if st := rebuildState(withRows(futures, events...)); st.slowDeclineWatched || st.slowDeclinePending || st.earlyTakeProfitWatched || st.earlyTakeProfit {
		t.Fatalf("a futures ladder is watched by no feature, so it is neither pending nor latched, got %+v", st)
	}

	withQuietDecline(t, false)
	withSwingDecline(t, false)
	if st := rebuildState(withRows(watchedTrade(), events...)); st.slowDeclineWatched || st.slowDeclinePending || !st.earlyTakeProfit {
		t.Fatalf("with the declines switched off the ladder is latched and not pending, got %+v", st)
	}
}

// An unusable event is skipped and the fold goes on: a document that does not decode, an empty or
// missing one, an unknown kind, a price at or under zero, and a kind filed under the wrong gate.
func TestRebuildStateSkipsWhatItCannotRead(t *testing.T) {
	unreadable := map[string]string{
		"not JSON":                      `not json`,
		"a truncated document":          `{"event":`,
		"an array":                      `[]`,
		"a string":                      `"cancelled"`,
		"an event that is no string":    `{"event":5,"price":175.83}`,
		"a price that is no number":     `{"event":"cancelled","price":"175.83"}`,
		"features that are no list":     `{"event":"cancelled","price":175.83,"features":"quietDecline"}`,
		"an empty object":               `{}`,
		"null":                          `null`,
		"an unknown kind":               `{"event":"unknown","price":175.83}`,
		"the hold's kind":               `{"event":"held","price":175.83}`,
		"a kind with no price":          `{"event":"cancelled"}`,
		"a kind with a zero price":      `{"event":"cancelled","price":0}`,
		"a kind with a negative price":  `{"event":"reset","price":-175.83}`,
		"a pending event with no price": `{"event":"pending"}`,
	}
	for name, document := range unreadable {
		event := eventOf(GateSlowDecline, document)
		if st := rebuildState(withEvents(pendingAtFive(), event)); !st.slowDeclinePending || st.slowDeclinePendingFrom != pendingFromFifth {
			t.Errorf("%s: after the pending event it must leave the ladder pending from its fill, got %+v", name, st)
		}
		before := withRow(withEvents(sixDeep(), event), PendingRow(pendingFromFifth, quietFinding(nil)), time.Time{})
		if st := rebuildState(before); !st.slowDeclinePending || st.slowDeclinePendingFrom != pendingFromFifth {
			t.Errorf("%s: before the pending event it must not stop the fold, got %+v", name, st)
		}
		if st := rebuildState(withEvents(sixDeep(), event)); st.slowDeclinePending || st.slowDeclinePendingFrom != 0 {
			t.Errorf("%s: alone it must make nothing pending, got %+v", name, st)
		}
		if st := rebuildState(withEvents(indecisionLadder(), eventOf(gateIndecision, document))); st.earlyTakeProfit {
			t.Errorf("%s: on the legacy indecision gate it must latch nothing, got %+v", name, st)
		}
	}

	nilDocument := aggragates.TradesStrategyEvents{Param: aggragates.StrategyParamSmartSell, Gate: GateSlowDecline}
	if st := rebuildState(withEvents(pendingAtFive(), nilDocument)); !st.slowDeclinePending {
		t.Errorf("an event with no document must be skipped, got %+v", st)
	}

	latchedWithoutFeatures := eventOf(GateSlowDecline, `{"event":"latched","price":175.83}`)
	if st := rebuildState(withEvents(pendingAtFive(), latchedWithoutFeatures)); !st.slowDeclinePending || st.earlyTakeProfit {
		t.Errorf("the latch's kind without features on the slow decline's gate is no event, got %+v", st)
	}
	for _, kind := range []string{EventPending, EventCancelled, EventReset, EventSold, EventSaleHeld, EventActivated} {
		foreign := eventOf(gateIndecision, `{"event":"`+kind+`","price":179.78,"features":["quietDecline"]}`)
		if st := rebuildState(withEvents(indecisionLadder(), foreign)); st.earlyTakeProfit || st.slowDeclinePending || st.saleHeld || st.slowDeclineActivated {
			t.Errorf("%q on the legacy indecision gate is no event, got %+v", kind, st)
		}
	}
}

// The documents a Postgres jsonb column hands back (reordered keys, spaces, another number form) fold exactly as the writer's.
func TestRebuildStateFoldsThePostgresRenderingOfADocument(t *testing.T) {
	written := withRow(sixDeep(), PendingRow(pendingFromFifth, quietFinding([]string{"leg down 7.0% from its high close", "leg 60 bars long on 1h"})), time.Time{})
	written = withRow(written, LatchedRow(sixthFill, indecisionFinding(nil)), time.Time{})
	if st := rebuildState(written); !st.slowDeclinePending || st.slowDeclinePendingFrom != pendingFromFifth || !st.earlyTakeProfit {
		t.Fatalf("fixture drifted: the written events must fold, got %+v", st)
	}

	renderings := map[string][2]string{
		"jsonb's key order and spacing": {
			`{"event": "pending", "price": 179.78, "reasons": ["leg down 7.0% from its high close", "leg 60 bars long on 1h"], "features": ["quietDecline"]}`,
			`{"event": "latched", "price": 175.83, "features": ["indecision"]}`,
		},
		"keys in reverse": {
			`{"features": ["quietDecline"], "reasons": ["leg down 7.0% from its high close"], "price": 179.78, "event": "pending"}`,
			`{"features": ["indecision"], "price": 175.83, "event": "latched"}`,
		},
		"the number in other forms": {
			`{"event":"pending","price":179.780,"features":["quietDecline"]}`,
			`{"event":"latched","price":1.7583E+2,"features":["indecision"]}`,
		},
		"whitespace everywhere": {
			"{\n  \"event\" : \"pending\" ,\n  \"price\" : 179.78 ,\n  \"features\" : [ \"quietDecline\" ]\n}",
			"\t{ \"event\":\"latched\",\"price\":175.83,\"features\":[\"indecision\"] }\n",
		},
	}
	for name, documents := range renderings {
		rendered := withEvents(sixDeep(), eventOf(GateSlowDecline, documents[0]), eventOf(GateSlowDecline, documents[1]))
		if got, want := rebuildState(rendered), rebuildState(written); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: folded to %+v, the written documents fold to %+v", name, got, want)
		}
	}

	var decoded EventData
	if err := json.Unmarshal(written.StrategyEvents[0].Data, &decoded); err != nil || decoded.Price != pendingFromFifth || !reflect.DeepEqual(decoded.Features, []string{FeatureQuietDecline}) {
		t.Fatalf("the writer's own document must decode to the fill and its feature, got %+v, %v", decoded, err)
	}
}

// The fold reads events and the events only: a pending event folds whatever
// the trade's rows say, or when it carries none.
func TestRebuildStateReadsTheEventsWhateverTheRowsSay(t *testing.T) {
	for name, logs := range map[string][]aggragates.TradesLogs{
		"no rows":           nil,
		"another row":       {{Message: "Hold trailingBuy: cooldown: depth held", Price: slowDeclineLastFill}},
		"the cancel's text": {{Message: CancelledRow(slowDeclineLastFill, quietFinding(nil)).Message, Price: slowDeclineLastFill}},
	} {
		trade := pendingTrade()
		trade.Logs = logs
		if st := rebuildState(trade); !st.slowDeclinePending || st.slowDeclinePendingFrom != slowDeclineLastFill {
			t.Errorf("%s: the pending event must fold, got %+v", name, st)
		}
	}
	latched := latchedBy(indecisionLadder())
	latched.Logs = nil
	if !rebuildState(latched).earlyTakeProfit {
		t.Error("a latched event folds with no row beside it")
	}
}

// Text is not state: a trade carrying every row text with no event reads exactly as the same trade bare.
func TestRebuildStateTextIsNotState(t *testing.T) {
	for name, trade := range map[string]aggragates.Trades{"a watched ladder": watchedTrade(), "a ladder at its last depth": lastDepthLadder(), "a six-deep ladder": sixDeep()} {
		newest := trade.PositionPrice
		stamp := rebuildState(trade).lastFill().At.Add(time.Hour)
		carrying := trade
		carrying.Logs = append(retiredRows(newest),
			aggragates.TradesLogs{Message: PendingRow(newest, quietFinding(slowDeclineReasons)).Message, Price: newest, Type: aggragates.LOG_INFO},
			aggragates.TradesLogs{Message: PendingRow(newest, indecisionFinding(nil), swingFinding(slowDeclineReasons)).Message, Price: newest, Type: aggragates.LOG_INFO},
			aggragates.TradesLogs{Message: CancelledRow(newest, quietFinding(quietDeclineBreakReasons)).Message, Price: newest, Type: aggragates.LOG_INFO},
			aggragates.TradesLogs{Message: ResetRow(newest, quietFinding(nil)).Message, Price: newest, Type: aggragates.LOG_INFO},
			aggragates.TradesLogs{Message: LatchedRow(newest, indecisionFinding(indecisionReasons)).Message, Price: newest, Type: aggragates.LOG_INFO},
			aggragates.TradesLogs{Message: "Hold entry: " + SlowDeclineEntryHoldReason, Price: newest, Type: aggragates.LOG_INFO},
			aggragates.TradesLogs{Message: "Hold trailingBuy: " + cooldown.DepthPriorityHoldMarker + ", ETH/USDT at depth 5 of 8 keeps the wallet for its remaining depths, this ladder waits at depth 4 of 8", Price: newest, Type: aggragates.LOG_INFO, CreatedAt: stamp},
		)
		if got, want := rebuildState(carrying), rebuildState(trade); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: text moved the state to %+v, want %+v", name, got, want)
		}
		if st := rebuildState(withRows(carrying, PendingRow(newest, quietFinding(nil)))); !st.slowDeclinePending || st.slowDeclinePendingFrom != newest {
			t.Errorf("%s: a pending event after the rows still folds, got %+v", name, st)
		}
	}
}

// The flat market episode folds off its own events in slice order: triggered opens it at its price,
// sale held and activated keep it open setting their own flag (opening it when the triggered row is
// lost), reset, sold and the legacy ended event close it and forget everything; the price stays the
// opening event's, and holds, other gates' events and unreadable documents leave it as it was.
func TestRebuildStateFoldsTheFlatMarketEpisode(t *testing.T) {
	withFlatMarket(t, true)
	trade := flatMarketLadder(false)
	fill := trade.PositionPrice
	const deeper = 96.0
	triggered := writes(FlatMarketTriggeredRow(fill, []string{"a reason"}))
	triggeredDeeper := writes(FlatMarketTriggeredRow(deeper, nil))
	held := writes(FlatMarketSaleHeldRow(fill, -3))
	heldDeeper := writes(FlatMarketSaleHeldRow(deeper, -3))
	activated := writes(FlatMarketActivatedRow(fill, -0.25))
	legacyEnded := records(eventOf(GateFlatMarket, legacyDocument(EventEnded, fill)))
	reset := writes(FlatMarketResetRow(deeper))
	sold := writes(ExitRow(trade, reasonFlatMarket, fill))
	hold := records(eventOf(GateFlatMarket, `{"event":"held","reasons":["a reason"]}`))
	otherParam := records(aggragates.NewStrategyEvent(trade.ID, aggragates.StrategyParamCooldown, GateFlatMarket, EventData{Event: EventTriggered, Price: fill}, time.Time{}))

	for name, tc := range map[string]struct {
		steps                       []step
		active, saleHeld, activated bool
		from                        float64
	}{
		"no event":               {nil, false, false, false, 0},
		"triggered":              {[]step{triggered}, true, false, false, fill},
		"triggered, sale held":   {[]step{triggered, held}, true, true, false, fill},
		"a sale held on its own": {[]step{held}, true, true, false, fill},
		"a sale held on its own, at a deeper price":       {[]step{heldDeeper}, true, true, false, deeper},
		"triggered, activated":                            {[]step{triggered, activated}, true, false, true, fill},
		"triggered, sale held, activated":                 {[]step{triggered, held, activated}, true, true, true, fill},
		"triggered, activated, sale held":                 {[]step{triggered, activated, held}, true, true, true, fill},
		"an activation on its own":                        {[]step{activated}, true, false, true, fill},
		"triggered, then held at a deeper price":          {[]step{triggered, heldDeeper}, true, true, false, fill},
		"triggered, held, a stored legacy ended event":    {[]step{triggered, held, legacyEnded}, false, false, false, 0},
		"triggered, sale held, activated, reset":          {[]step{triggered, held, activated, reset}, false, false, false, 0},
		"triggered, sale held, sold":                      {[]step{triggered, held, sold}, false, false, false, 0},
		"triggered, activated, sold":                      {[]step{triggered, activated, sold}, false, false, false, 0},
		"reset and sold with no episode open":             {[]step{reset, sold}, false, false, false, 0},
		"triggered, sale held, triggered again":           {[]step{triggered, held, triggered}, true, false, false, fill},
		"triggered, activated, triggered again":           {[]step{triggered, activated, triggered}, true, false, false, fill},
		"a second episode":                                {[]step{triggered, held, sold, triggered}, true, false, false, fill},
		"a second episode, its sale held":                 {[]step{triggered, held, sold, triggered, held}, true, true, false, fill},
		"a second episode at a new depth":                 {[]step{triggered, held, activated, reset, triggeredDeeper}, true, false, false, deeper},
		"a second episode at a new depth, its activation": {[]step{triggered, activated, reset, triggeredDeeper, activated}, true, false, true, deeper},
		"a hold between the rows":                         {[]step{triggered, hold, held, hold}, true, true, false, fill},
		"a hold alone":                                    {[]step{hold}, false, false, false, 0},
		"the slow decline's sale held":                    {[]step{writes(SaleHeldRow(fill, -3, FeatureQuietDecline))}, false, false, false, 0},
		"the slow decline's activated event":              {[]step{triggered, held, writes(ActivatedRow(fill, -0.25, FeatureQuietDecline))}, true, true, false, fill},
		"the slow decline's reset event":                  {[]step{triggered, held, writes(ResetRow(deeper, quietFinding(nil)))}, true, true, false, fill},
		"the slow decline's sold event":                   {[]step{triggered, held, writes(ExitRow(trade, reasonSlowDecline, fill))}, true, true, false, fill},
		"capital protection's sold event":                 {[]step{triggered, held, writes(ExitRow(trade, reasonCapitalProtection, fill))}, true, true, false, fill},
		"a kind under another gate":                       {[]step{records(eventOf(GateEntryHold, legacyDocument(EventTriggered, fill)))}, false, false, false, 0},
		"the gate's events under another param":           {[]step{otherParam}, false, false, false, 0},
	} {
		st := rebuildState(withSteps(trade, tc.steps...))
		if !st.flatMarketWatched || st.flatMarketActive != tc.active || st.flatMarketSaleHeld != tc.saleHeld || st.flatMarketActivated != tc.activated || st.flatMarketFrom != tc.from {
			t.Errorf("%s: watched %v, episode open %v, sale held %v, trailingBuy activated %v, opened at %v; want open %v, sale held %v, activated %v, opened at %v",
				name, st.flatMarketWatched, st.flatMarketActive, st.flatMarketSaleHeld, st.flatMarketActivated, st.flatMarketFrom, tc.active, tc.saleHeld, tc.activated, tc.from)
		}
	}

	for name, document := range map[string]string{
		"not JSON":                           `not json`,
		"an empty object":                    `{}`,
		"a triggered event without a price":  `{"event":"triggered"}`,
		"a triggered event at a zero price":  `{"event":"triggered","price":0}`,
		"a triggered event under zero":       `{"event":"triggered","price":-184.45}`,
		"an ended event without a price":     `{"event":"ended"}`,
		"a reset event without a price":      `{"event":"reset"}`,
		"an activated event without a price": `{"event":"activated"}`,
		"an activated event at a zero price": `{"event":"activated","price":0}`,
		"a sold event without a price":       `{"event":"sold"}`,
		"an unknown kind":                    legacyDocument("unknown", fill),
		"a kind of the slow decline gate":    legacyDocument(EventPending, fill),
		"the hold's kind with a price":       legacyDocument(gates.EventHeld, fill),
	} {
		unreadable := records(eventOf(GateFlatMarket, document))
		if st := rebuildState(withSteps(trade, unreadable)); st.flatMarketActive || st.flatMarketSaleHeld || st.flatMarketActivated || st.flatMarketFrom != 0 {
			t.Errorf("%s: alone it must open nothing, got %+v", name, st)
		}
		if st := rebuildState(withSteps(trade, triggered, held, activated, unreadable)); !st.flatMarketActive || !st.flatMarketSaleHeld || !st.flatMarketActivated || st.flatMarketFrom != fill {
			t.Errorf("%s: it must leave an open episode as it was, got %+v", name, st)
		}
	}
}

// The gate folds its events on the ladders it watches and on those alone: a spot
// ladder with a filled entry, long or inverse, whether the older features watch
// it too or nothing else does — the fold of those returns before it reads a row
// and the gate's pass is made first — and not a futures ladder, a ladder with no
// fill, or any ladder while flatmarket.Enabled is off. That holds for the whole
// episode: the price it opened at, the held sale and the activated trailingBuy. The rows
// carry a price of their own, so that a ladder with no fill is skipped for want of a
// fill and not of a price.
func TestRebuildStateFoldsTheFlatMarketEventsOfTheLaddersItWatches(t *testing.T) {
	const fill = 184.45
	futures := flatMarketLadder(false)
	futures.Strategy.TradeType = aggragates.Futures
	cases := map[string]struct {
		trade   aggragates.Trades
		watched bool
	}{
		"a long ladder":                       {flatMarketLadder(false), true},
		"an inverse ladder":                   {flatMarketLadder(true), true},
		"a long ladder with a single fill":    {testutil.LadderTrade(false, fills(1, "17:38:00")...), true},
		"an inverse ladder at its last depth": {testutil.LadderTrade(true, fills(lastDepthFills, "21:30:00")...), true},
		"a futures ladder":                    {futures, false},
		"a ladder with no fill":               {testutil.LadderTrade(false), false},
	}
	for _, olderOn := range []bool{false, true} {
		withQuietDecline(t, olderOn)
		withSwingDecline(t, olderOn)
		withEarlyTakeProfit(t, olderOn)
		withCapitalProtectionExit(t, olderOn)

		withFlatMarket(t, true)
		for name, tc := range cases {
			written := withSteps(tc.trade, writes(FlatMarketTriggeredRow(fill, nil)), writes(FlatMarketSaleHeldRow(fill, -3)), writes(FlatMarketActivatedRow(fill, -0.25)))
			st := rebuildState(written)
			if st.flatMarketWatched != tc.watched || st.flatMarketActive != tc.watched || st.flatMarketSaleHeld != tc.watched || st.flatMarketActivated != tc.watched || (st.flatMarketFrom == fill) != tc.watched {
				t.Errorf("older features on %v, %s: watched %v, episode open %v, sale held %v, trailingBuy activated %v, opened at %v, want all %v and the price %v", olderOn, name, st.flatMarketWatched, st.flatMarketActive, st.flatMarketSaleHeld, st.flatMarketActivated, st.flatMarketFrom, tc.watched, fill)
			}
		}

		withFlatMarket(t, false)
		for name, tc := range cases {
			written := withSteps(tc.trade, writes(FlatMarketTriggeredRow(fill, nil)), writes(FlatMarketSaleHeldRow(fill, -3)), writes(FlatMarketActivatedRow(fill, -0.25)))
			if st := rebuildState(written); st.flatMarketWatched || st.flatMarketActive || st.flatMarketSaleHeld || st.flatMarketActivated || st.flatMarketFrom != 0 {
				t.Errorf("older features on %v, %s: with flatmarket.Enabled off its events are ignored, got %+v", olderOn, name, st)
			}
		}
	}
}

// The two folds do not meet: the slow decline's pending, latch, held sale and
// activated trailingBuy read none of the flat market gate's events, and its episode
// reads none of theirs. A ladder pending on the quiet decline that carries an
// episode's rows reads the same as without them but for the episode — whether it is
// open, its sale held, its trailingBuy activated, or the gate's own sale or reset has
// closed it — and the slow decline's pending outlives that sale. The other
// way round, the slow decline's activation and held sale move its own flags alone and
// open no episode.
func TestRebuildStateKeepsTheFlatMarketEpisodeApartFromTheSlowDecline(t *testing.T) {
	withFlatMarket(t, true)
	withQuietDecline(t, true)
	base := pendingAtFive()
	fill := base.PositionPrice
	want := rebuildState(base)
	if !want.slowDeclinePending || !want.flatMarketWatched {
		t.Fatalf("fixture drifted: the ladder must be pending on the quiet decline and watched by the flat market gate, got %+v", want)
	}

	triggered, held, activated := writes(FlatMarketTriggeredRow(fill, nil)), writes(FlatMarketSaleHeldRow(fill, -3)), writes(FlatMarketActivatedRow(fill, -0.25))
	open := withSteps(base, triggered, held)
	committed := withSteps(open, activated)
	for name, tc := range map[string]struct {
		trade                       aggragates.Trades
		active, saleHeld, activated bool
	}{
		"an episode open": {open, true, true, false},
		"an episode whose trailingBuy is activated": {committed, true, true, true},
		"closed by the gate's sale":                 {withSteps(committed, writes(ExitRow(committed, reasonFlatMarket, fill))), false, false, false},
		"closed by the gate's reset":                {withSteps(committed, writes(FlatMarketResetRow(fill-1))), false, false, false},
	} {
		got := rebuildState(tc.trade)
		if got.flatMarketActive != tc.active || got.flatMarketSaleHeld != tc.saleHeld || got.flatMarketActivated != tc.activated || (got.flatMarketFrom == fill) != tc.active {
			t.Errorf("%s: episode open %v, its sale held %v, its trailingBuy activated %v, opened at %v; want %v, %v, %v and the fill's price %v", name, got.flatMarketActive, got.flatMarketSaleHeld, got.flatMarketActivated, got.flatMarketFrom, tc.active, tc.saleHeld, tc.activated, fill)
		}
		if got.saleHeld || got.slowDeclineActivated {
			t.Errorf("%s: the slow decline's sale held %v and activated trailingBuy %v, want neither", name, got.saleHeld, got.slowDeclineActivated)
		}
		if heldUnder := (heldTrigger{pct: FullTrendTrigger(), named: true}); (got.flatMarketHeldUnder == heldUnder) != tc.saleHeld {
			t.Errorf("%s: the episode's held row was held under %+v, want %+v while its sale is held", name, got.flatMarketHeldUnder, heldUnder)
		}
		got.flatMarketActive, got.flatMarketSaleHeld, got.flatMarketActivated, got.flatMarketFrom = false, false, false, 0
		got.flatMarketHeldUnder, got.flatMarketActivatedIn = heldTrigger{}, ""
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: the slow decline's state moved to %+v, want %+v", name, got, want)
		}
	}

	slow := rebuildState(withRows(base, SaleHeldRow(fill, -3, FeatureQuietDecline), ActivatedRow(fill, -0.25, FeatureQuietDecline)))
	if !slow.saleHeld || !slow.slowDeclineActivated || slow.flatMarketActive || slow.flatMarketSaleHeld || slow.flatMarketActivated || slow.flatMarketFrom != 0 {
		t.Errorf("the slow decline's held sale and activation move its own flags alone, got %+v", slow)
	}
	slow.saleHeld, slow.saleHeldUnder, slow.slowDeclineActivated, slow.slowDeclineActivatedIn = false, heldTrigger{}, false, ""
	if !reflect.DeepEqual(slow, want) {
		t.Errorf("the slow decline's flags moved something else: %+v, want %+v", slow, want)
	}
}
