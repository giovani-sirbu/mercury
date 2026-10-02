package smarttakeloss

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The rebuildState specification: the fold reads the trade's smartTakeLoss
// events, in slice order, and nothing else of the trade's record — never the
// text of a log row, never a stamp. The events the gates before the merge wrote
// fold as they did (rebuildStateLegacy_test.go).

// retiredRows are the rows older releases wrote for the retired
// trend-reversal rule and that stay on trades in the database: its activation
// row and the row its wait after a fill refused an add with, each carrying a
// fill's price. The texts are copied as those releases wrote them. They are
// text with no event beside them, and read as nothing.
func retiredRows(price float64) []aggragates.TradesLogs {
	return []aggragates.TradesLogs{
		{Message: "Hold buy: smartTakeLoss: Potential trend reversal", Price: price, Type: aggragates.LOG_INFO},
		{Message: "Hold stopLoss: smartTakeLoss: last fill too fresh to sell, no add (0s)", Price: price, Type: aggragates.LOG_INFO},
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

// Without events a watched ladder is not pending, and its newest fill is the
// last in slice order even when its stamp is older than the one before it
// (live-testing stamps by hand).
func TestRebuildStateWithoutEvents(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(5, "17:38:00")...)
	trade.History[4].CreatedAt = testutil.At("07:00:00")

	st := rebuildState(trade)
	if !st.slowDeclineWatched || !st.quietDeclineWatched || st.slowDeclinePending || st.slowDeclinePendingFrom != 0 || len(st.pendingFeatures) != 0 || st.capitalProtectionWatched {
		t.Fatalf("no event must mean nothing pending, got %+v", st)
	}
	if !st.indecisionWatched || st.indecision {
		t.Fatalf("no event must mean nothing latched on a watched ladder, got %+v", st)
	}
	if len(st.fills) != 5 || st.lastFill().Price != 179.78 {
		t.Fatalf("5 fills and the last fill is 179.78, got %+v", st)
	}
}

// A pending event makes a watched ladder pending from the fill its price
// names, on the decline features it names, whatever else the trade carries
// around it: the pair of another gate, text rows. It never makes pending a
// ladder the gate does not watch — one short of QuietDeclineArmDepth included —
// and the first-fill hold's held event is no pending event.
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
	inverse := withRows(testutil.LadderTrade(true, fills(QuietDeclineArmDepth, "17:38:00")...), marker)
	noFill := withRows(testutil.LadderTrade(false), marker)
	shallow := withRows(testutil.LadderTrade(false, fills(QuietDeclineArmDepth-1, "17:38:00")...), marker)
	for _, unwatched := range []aggragates.Trades{inverse, noFill, shallow} {
		if st := rebuildState(unwatched); st.slowDeclineWatched || st.slowDeclinePending {
			t.Fatalf("a ladder the gate does not watch is never pending, got %+v", st)
		}
	}

	// QuietDeclineArmDepth filled entries are enough to be watched, and to be
	// pending on the event, exactly as quietDeclineWatched reads the trade.
	shallowest := withRows(testutil.LadderTrade(false, fills(QuietDeclineArmDepth, "17:38:00")...), marker)
	if st := rebuildState(shallowest); !st.quietDeclineWatched || !st.slowDeclinePending || !quietDeclineWatched(shallowest) || !slowDeclineWatched(shallowest) {
		t.Fatalf("a long ladder is watched from QuietDeclineArmDepth, got %+v", st)
	}
	if quietDeclineWatched(shallow) {
		t.Fatal("quietDeclineWatched must agree with rebuildState on the shallower ladder")
	}
}

// The gate's events fold in slice order and the last one wins: a pending event
// makes a watched ladder pending from the fill its price names, a cancelled or
// a reset event makes it not pending. Events of other gates between them
// change nothing, the sale changes nothing, an event without a price is
// skipped, and a ladder the gate does not watch is pending on none of them.
func TestRebuildStateFoldsTheSlowDeclineEventsInOrder(t *testing.T) {
	pending := func(price float64) step { return writes(PendingRow(price, quietFinding(slowDeclineReasons))) }
	cancelled := func(price float64) step { return writes(CancelledRow(price, quietFinding(slowDeclineBreakReasons))) }
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
		"pending, sold":                                 {[]step{pending(pendingFromFifth), sold(reasonSellBand)}, true, pendingFromFifth},
		"pending, capital protection's sold":            {[]step{pending(pendingFromFifth), sold(reasonCapitalProtection)}, true, pendingFromFifth},
		"a sold alone":                                  {[]step{sold(reasonSellBand)}, false, 0},
		"pending, cancelled, sold":                      {[]step{pending(pendingFromFifth), cancelled(sixthFill), sold(reasonSellBand)}, false, 0},
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

		shallow := testutil.LadderTrade(false, fills(QuietDeclineArmDepth-1, "17:38:00")...)
		if st := rebuildState(withSteps(shallow, tc.steps...)); st.slowDeclinePending || st.slowDeclinePendingFrom != 0 {
			t.Errorf("%s: a ladder the gate does not watch is never pending, got %+v", name, st)
		}
	}
}

// A pending event stands for the decline features it names, and a feature that
// does not watch the ladder — switched off, or a ladder it never reads — is
// ignored with its events: the ladder is pending on the features that do, and a
// pending event naming only features that do not is a ladder not pending. The
// features it is pending on are those of the last pending event, never the
// union over the events before.
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

// The fold reads the order of the events in the trade and never their stamps:
// a cancelled event stamped earlier than the pending event before it, but
// after it in the slice, cancels; a pending event stamped earlier than the
// cancelled event before it, but after it in the slice, makes the ladder
// pending. Events of one stamp fold in slice order too.
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

// The sale changes nothing: a pending ladder whose exit sold — either rule —
// reads pending from the same fill, a ladder not pending reads not pending,
// and no other field of the state moves.
func TestRebuildStateSoldChangesNothing(t *testing.T) {
	for _, reason := range []string{reasonSellBand, reasonCapitalProtection} {
		for name, trade := range map[string]aggragates.Trades{"pending": pendingAtFive(), "not pending": watchedTrade(), "last depth": lastDepthLadder()} {
			sold := withRow(trade, ExitRow(trade, reason, slowDeclineBand), testutil.At("23:30:00"))
			if got, want := rebuildState(sold), rebuildState(trade); !reflect.DeepEqual(got, want) {
				t.Errorf("%s, sold by %q: the state moved to %+v, want %+v", name, reason, got, want)
			}
		}
	}
}

// Every event of another gate or another param changes nothing: the
// cooldown's three gates, the entry hold, capital protection's sale and the
// dynamic params' opened event, and the slow decline gate's own kinds filed
// under another param. Each is unstamped, so none holds a depth priority.
func TestRebuildStateIgnoresTheOtherGatesEvents(t *testing.T) {
	other := []aggragates.TradesStrategyEvents{
		cooldown.NewFirstFillEvent(45211, cooldown.FirstFillEvent{Event: cooldown.FirstFillActivated, Price: 190, Reference: 190}, time.Time{}),
		cooldown.NewFirstFillEvent(45211, cooldown.FirstFillEvent{Event: cooldown.FirstFillEntered, Price: 191, Reference: 190}, time.Time{}),
		cooldown.NewDepthSpacingEvent(45211, cooldown.DepthSpacingEvent{Event: gates.EventHeld, Depth: 4, Step: 3, Hold: time.Hour}, time.Time{}),
		cooldown.NewDepthPriorityEvent(45211, cooldown.DepthPriorityEvent{Event: gates.EventHeld, PrioritySymbol: "ETH/USDT", PriorityDepth: 5, PriorityMaxDepth: 8, Depth: 4, MaxDepth: 8}, time.Time{}),
		aggragates.NewStrategyEvent(45211, aggragates.StrategyParamSmartTakeLoss, GateEntryHold, EventData{Event: gates.EventHeld}, time.Time{}),
		aggragates.NewStrategyEvent(45211, aggragates.StrategyParamSmartTakeLoss, GateCapitalProtection, EventData{Event: EventSold, Price: capitalProtectionBand}, time.Time{}),
		aggragates.NewStrategyEvent(45211, aggragates.StrategyParamDynamicParams, dynamicparams.GateOpened, dynamicparams.OpenedEvent{Event: dynamicparams.EventOpened, Points: 0.4, Depths: 1}, time.Time{}),
		aggragates.NewStrategyEvent(45211, aggragates.StrategyParamCooldown, GateSlowDecline, EventData{Event: EventCancelled, Price: sixthFill, Features: []string{FeatureQuietDecline}}, time.Time{}),
		aggragates.NewStrategyEvent(45211, aggragates.StrategyParamDynamicParams, GateSlowDecline, EventData{Event: EventReset, Price: sixthFill, Features: []string{FeatureQuietDecline}}, time.Time{}),
		aggragates.NewStrategyEvent(45211, aggragates.StrategyParamCooldown, gateIndecision, EventData{Event: EventLatched, Price: sixthFill}, time.Time{}),
		aggragates.NewStrategyEvent(45211, aggragates.StrategyParamCooldown, GateSlowDecline, EventData{Event: EventLatched, Price: sixthFill, Features: []string{FeatureIndecision}}, time.Time{}),
	}
	for name, trade := range map[string]aggragates.Trades{"pending": pendingAtFive(), "not pending": watchedTrade(), "latched": latchedBy(indecisionLadder())} {
		with := withEvents(trade, other...)
		if got, want := rebuildState(with), rebuildState(trade); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: another gate's events moved the state to %+v, want %+v", name, got, want)
		}
	}
}

// While QuietDecline is off the feature watches no ladder, so the events that
// name it are ignored: a pending event makes nothing pending, whatever else
// still watches the ladder.
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

// An event that carries the indecision among its features latches a ladder the
// indecision direction watches — on a latched kind and on the pending,
// cancelled and reset kinds of a row that latched the ladder beside — and
// nothing takes the latch away: no cancelled event, no later pending event, no
// event of another kind. It latches no ladder the rule does not watch — one
// short of IndecisionArmDepth, inverse, futures, with no fill — and while
// Indecision is off every such event is ignored. The pending fold reads the same
// events as it does without the latch: each feature reads its own.
func TestRebuildStateLatchesOnTheEventsThatNameTheIndecision(t *testing.T) {
	latch := writes(LatchedRow(sixthFill, indecisionFinding(indecisionReasons)))
	latchWithoutAPrice := records(eventOf(GateSlowDecline, `{"event":"latched","features":["indecision"]}`), eventOf(gateIndecision, `{"event":"latched"}`))
	latchWithoutFeatures := records(eventOf(GateSlowDecline, legacyDocument(EventLatched, sixthFill)))
	marker := writes(PendingRow(pendingFromFifth, quietFinding(slowDeclineReasons)))
	cancel := writes(CancelledRow(sixthFill, quietFinding(slowDeclineBreakReasons)))
	latchingPending := writes(PendingRow(pendingFromFifth, indecisionFinding(nil), quietFinding(nil)))
	latchingCancel := writes(CancelledRow(sixthFill, indecisionFinding(indecisionReasons), quietFinding(indecisionReasons)))
	latchingReset := writes(ResetRow(sixthFill, indecisionFinding(indecisionReasons), quietFinding(nil)))
	for name, tc := range map[string]struct {
		steps            []step
		latched, pending bool
	}{
		"no event":                                  {nil, false, false},
		"a latched event":                           {[]step{latch}, true, false},
		"a latched event without a price":           {[]step{latchWithoutAPrice}, false, false},
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
		if !st.indecisionWatched || st.indecision != tc.latched || st.slowDeclinePending != tc.pending {
			t.Errorf("%s: latched %v pending %v, want %v and %v (%+v)", name, st.indecision, st.slowDeclinePending, tc.latched, tc.pending, st)
		}
	}

	latchedAtTheFill := LatchedRow(slowDeclineLastFill, indecisionFinding(nil))
	shallow := testutil.LadderTrade(false, fills(IndecisionArmDepth-1, "17:38:00")...)
	inverse := testutil.LadderTrade(true, fills(watchedFills, "17:38:00")...)
	futures := watchedTrade()
	futures.Strategy.TradeType = aggragates.Futures
	noFill := testutil.LadderTrade(false)
	for name, trade := range map[string]aggragates.Trades{"one short of IndecisionArmDepth": shallow, "an inverse ladder": inverse, "a futures ladder": futures, "a ladder with no fill": noFill} {
		trade = withRows(trade, latchedAtTheFill, PendingRow(slowDeclineLastFill, indecisionFinding(nil), quietFinding(nil)))
		if st := rebuildState(trade); st.indecisionWatched || st.indecision || indecisionWatched(trade) {
			t.Errorf("%s: a ladder the rule does not watch is never latched, got %+v", name, st)
		}
	}
	atDepth := withRows(testutil.LadderTrade(false, fills(IndecisionArmDepth, "17:38:00")...), latchedAtTheFill)
	if st := rebuildState(atDepth); !st.indecisionWatched || !st.indecision || !indecisionWatched(atDepth) {
		t.Fatalf("a long spot ladder is watched and latched from IndecisionArmDepth, exactly as indecisionWatched reads it, got %+v", st)
	}

	withIndecision(t, false)
	if st := rebuildState(atDepth); st.indecisionWatched || st.indecision {
		t.Fatalf("switched off, the latched event must be ignored, got %+v", st)
	}
}

// The features fold each on its own watch: a ladder the indecision direction
// watches and the gate does not — the declines switched off — is latched on its
// event all the same, and a futures ladder the quiet decline watches and the
// indecision direction does not goes pending on its event all the same.
func TestRebuildStateFoldsEachFeatureApart(t *testing.T) {
	events := []Row{
		PendingRow(slowDeclineLastFill, quietFinding(nil)),
		LatchedRow(slowDeclineLastFill, indecisionFinding(nil)),
	}
	futures := watchedTrade()
	futures.Strategy.TradeType = aggragates.Futures
	if st := rebuildState(withRows(futures, events...)); !st.slowDeclinePending || st.indecisionWatched || st.indecision {
		t.Fatalf("a futures ladder goes pending and is not latched, got %+v", st)
	}

	withQuietDecline(t, false)
	withSwingDecline(t, false)
	if st := rebuildState(withRows(watchedTrade(), events...)); st.slowDeclineWatched || st.slowDeclinePending || !st.indecision {
		t.Fatalf("with the declines switched off the ladder is latched and not pending, got %+v", st)
	}
}

// An event the fold cannot use is skipped, and the fold goes on past it: a
// document that does not decode, an empty document, no document at all, a
// kind the gate does not have, and a price at or under zero. Each one leaves a
// ladder pending from the fill it was pending from, before it or after it,
// and makes a ladder that was not pending no more pending; filed under the
// legacy indecision gate none of them latches. A kind of the other gate — latched
// on the slow decline's without features, the slow decline's own on the
// indecision's — is such a kind too.
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
		"a latched event with no price": `{"event":"latched"}`,
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
		if st := rebuildState(withEvents(indecisionLadder(), eventOf(gateIndecision, document))); st.indecision {
			t.Errorf("%s: on the legacy indecision gate it must latch nothing, got %+v", name, st)
		}
	}

	nilDocument := aggragates.TradesStrategyEvents{Param: aggragates.StrategyParamSmartTakeLoss, Gate: GateSlowDecline}
	if st := rebuildState(withEvents(pendingAtFive(), nilDocument)); !st.slowDeclinePending {
		t.Errorf("an event with no document must be skipped, got %+v", st)
	}

	latchedWithoutFeatures := eventOf(GateSlowDecline, `{"event":"latched","price":175.83}`)
	if st := rebuildState(withEvents(pendingAtFive(), latchedWithoutFeatures)); !st.slowDeclinePending || st.indecision {
		t.Errorf("the latch's kind without features on the slow decline's gate is no event, got %+v", st)
	}
	for _, kind := range []string{EventPending, EventCancelled, EventReset, EventSold} {
		foreign := eventOf(gateIndecision, `{"event":"`+kind+`","price":179.78}`)
		if st := rebuildState(withEvents(indecisionLadder(), foreign)); st.indecision || st.slowDeclinePending {
			t.Errorf("%q on the legacy indecision gate is no event, got %+v", kind, st)
		}
	}
}

// The documents a Postgres jsonb column hands back are the writers' documents
// in another rendering: keys in the order jsonb keeps them or in any other,
// spaces after the separators, the number in another form. Each folds exactly
// as the document the writer marshalled.
func TestRebuildStateFoldsThePostgresRenderingOfADocument(t *testing.T) {
	written := withRow(sixDeep(), PendingRow(pendingFromFifth, quietFinding([]string{"leg down 7.0% from its high close", "leg 60 bars long on 1h"})), time.Time{})
	written = withRow(written, LatchedRow(sixthFill, indecisionFinding(nil)), time.Time{})
	if st := rebuildState(written); !st.slowDeclinePending || st.slowDeclinePendingFrom != pendingFromFifth || !st.indecision {
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
		"another row":       {{Message: "Hold stopLoss: cooldown: depth held", Price: slowDeclineLastFill}},
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
	if !rebuildState(latched).indecision {
		t.Error("a latched event folds with no row beside it")
	}
}

// Text is not state. A trade carrying every row text the smart take loss
// writes, the retired rule's rows and a row of the cooldown's depth priority
// stamped after its newest fill — each with a price, framed, and no event at
// all — reads exactly as the same trade bare: nothing pending, nothing
// latched, nothing held. The events written after those rows still fold.
func TestRebuildStateTextIsNotState(t *testing.T) {
	for name, trade := range map[string]aggragates.Trades{"a watched ladder": watchedTrade(), "a ladder at its last depth": lastDepthLadder(), "a six-deep ladder": sixDeep()} {
		newest := trade.PositionPrice
		stamp := rebuildState(trade).lastFill().At.Add(time.Hour)
		carrying := trade
		carrying.Logs = append(retiredRows(newest),
			aggragates.TradesLogs{Message: PendingRow(newest, quietFinding(slowDeclineReasons)).Message, Price: newest, Type: aggragates.LOG_INFO},
			aggragates.TradesLogs{Message: PendingRow(newest, indecisionFinding(nil), swingFinding(slowDeclineReasons)).Message, Price: newest, Type: aggragates.LOG_INFO},
			aggragates.TradesLogs{Message: CancelledRow(newest, quietFinding(slowDeclineBreakReasons)).Message, Price: newest, Type: aggragates.LOG_INFO},
			aggragates.TradesLogs{Message: ResetRow(newest, quietFinding(nil)).Message, Price: newest, Type: aggragates.LOG_INFO},
			aggragates.TradesLogs{Message: LatchedRow(newest, indecisionFinding(indecisionReasons)).Message, Price: newest, Type: aggragates.LOG_INFO},
			aggragates.TradesLogs{Message: "Hold entry: " + SlowDeclineEntryHoldReason, Price: newest, Type: aggragates.LOG_INFO},
			aggragates.TradesLogs{Message: "Hold stopLoss: " + cooldown.DepthPriorityHoldMarker + ", ETH/USDT at depth 5 of 8 keeps the wallet for its remaining depths, this ladder waits at depth 4 of 8", Price: newest, Type: aggragates.LOG_INFO, CreatedAt: stamp},
		)
		if got, want := rebuildState(carrying), rebuildState(trade); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: text moved the state to %+v, want %+v", name, got, want)
		}
		if st := rebuildState(withRows(carrying, PendingRow(newest, quietFinding(nil)))); !st.slowDeclinePending || st.slowDeclinePendingFrom != newest {
			t.Errorf("%s: a pending event after the rows still folds, got %+v", name, st)
		}
	}
}
