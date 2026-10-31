package smartsell

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/quietdecline"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The events written before the gates were merged stay on trades in the
// database and fold as they did: a feature-less event of the slow decline gate
// drives the pending state of the quiet decline, an event of the slow pattern
// gate the one of the swing decline, a latched event of the indecision gate
// latches. The two pending states are independent, the ladder is pending while
// either is, and the first event that names features — written by the gate as it
// is — supersedes them. The documents are written by hand, as those releases
// wrote them: no features, no reasons.

// quietEvent, patternEvent and legacyLatch are the steps of one such event each,
// the kind and price given.
func quietEvent(kind string, price float64) step {
	return records(eventOf(GateSlowDecline, legacyDocument(kind, price)))
}

func patternEvent(kind string, price float64) step {
	return records(eventOf(gateSlowPattern, legacyDocument(kind, price)))
}

func legacyLatch(price float64) step {
	return records(eventOf(gateIndecision, legacyDocument(EventLatched, price)))
}

// foldsTo is the table of the legacy fold: the steps written to a ladder six
// deep and what the ladder reads as after them.
type foldsTo struct {
	steps    []step
	from     float64
	features []string
	latched  bool
}

// The legacy fold, case by case. A ladder is pending while either state is,
// from the price of the most recent pending event among the states still
// pending, on the features of those states in the order the rows name them; the
// slow pattern gate has no reset kind of its own, so a reset event on it is no
// event; a latched event of the indecision gate latches and nothing takes it
// away; and the two states are independent — quiet pending with the pattern
// cancelled stays pending.
func TestRebuildStateFoldsTheLegacyEvents(t *testing.T) {
	quiet, swing, both := []string{FeatureQuietDecline}, []string{FeatureSwingDecline}, []string{FeatureQuietDecline, FeatureSwingDecline}
	for name, tc := range map[string]foldsTo{
		"nothing":                   {nil, 0, nil, false},
		"the quiet decline pending": {[]step{quietEvent(EventPending, 184.45)}, 184.45, quiet, false},
		"the quiet decline pending, again at a fill":    {[]step{quietEvent(EventPending, 184.45), quietEvent(EventPending, 179.78)}, 179.78, quiet, false},
		"the quiet decline pending, cancelled":          {[]step{quietEvent(EventPending, 184.45), quietEvent(EventCancelled, 179.78)}, 0, nil, false},
		"the quiet decline pending, reset":              {[]step{quietEvent(EventPending, 184.45), quietEvent(EventReset, 184.45)}, 0, nil, false},
		"the quiet decline cancelled, then pending":     {[]step{quietEvent(EventCancelled, 184.45), quietEvent(EventPending, 179.78)}, 179.78, quiet, false},
		"the quiet decline pending, its sale":           {[]step{quietEvent(EventPending, 184.45), quietEvent(EventSold, 186)}, 184.45, quiet, false},
		"the pattern pending":                           {[]step{patternEvent(EventPending, 184.45)}, 184.45, swing, false},
		"the pattern pending, cancelled":                {[]step{patternEvent(EventPending, 184.45), patternEvent(EventCancelled, 179.78)}, 0, nil, false},
		"the pattern pending, a reset it never wrote":   {[]step{patternEvent(EventPending, 184.45), patternEvent(EventReset, 184.45)}, 184.45, swing, false},
		"the pattern pending, its sale":                 {[]step{patternEvent(EventPending, 184.45), patternEvent(EventSold, 186)}, 184.45, swing, false},
		"both pending, the pattern's the more recent":   {[]step{quietEvent(EventPending, 184.45), patternEvent(EventPending, 179.78)}, 179.78, both, false},
		"both pending, the quiet decline's the recent":  {[]step{patternEvent(EventPending, 184.45), quietEvent(EventPending, 179.78)}, 179.78, both, false},
		"both pending at one price":                     {[]step{quietEvent(EventPending, 184.45), patternEvent(EventPending, 184.45)}, 184.45, both, false},
		"both pending, the pattern cancelled":           {[]step{quietEvent(EventPending, 184.45), patternEvent(EventPending, 179.78), patternEvent(EventCancelled, 175.83)}, 184.45, quiet, false},
		"both pending, the quiet decline cancelled":     {[]step{quietEvent(EventPending, 184.45), patternEvent(EventPending, 179.78), quietEvent(EventCancelled, 175.83)}, 179.78, swing, false},
		"both pending, the quiet decline reset":         {[]step{quietEvent(EventPending, 184.45), patternEvent(EventPending, 179.78), quietEvent(EventReset, 175.83)}, 179.78, swing, false},
		"both pending, both cancelled":                  {[]step{quietEvent(EventPending, 184.45), patternEvent(EventPending, 179.78), quietEvent(EventCancelled, 175.83), patternEvent(EventCancelled, 175.83)}, 0, nil, false},
		"the quiet decline pending twice, the pattern":  {[]step{quietEvent(EventPending, 184.45), patternEvent(EventPending, 179.78), quietEvent(EventPending, 175.83)}, 175.83, both, false},
		"the latch alone":                               {[]step{legacyLatch(184.45)}, 0, nil, true},
		"the latch, the quiet decline pending":          {[]step{legacyLatch(184.45), quietEvent(EventPending, 184.45)}, 184.45, quiet, true},
		"the quiet decline pending, cancelled, latched": {[]step{quietEvent(EventPending, 184.45), quietEvent(EventCancelled, 179.78), legacyLatch(179.78)}, 0, nil, true},
		"an event without a price is skipped":           {[]step{quietEvent(EventPending, 184.45), records(eventOf(GateSlowDecline, `{"event":"cancelled"}`)), records(eventOf(gateSlowPattern, `{"event":"pending"}`))}, 184.45, quiet, false},
		"events of other gates between them":            {[]step{quietEvent(EventPending, 184.45), elsewhere, patternEvent(EventPending, 179.78), elsewhere}, 179.78, both, false},
		"the quiet decline pending, an activated kind":  {[]step{quietEvent(EventPending, 184.45), quietEvent(EventActivated, 179.78)}, 184.45, quiet, false},
		"the pattern pending, an activated kind":        {[]step{patternEvent(EventPending, 184.45), patternEvent(EventActivated, 179.78)}, 184.45, swing, false},
		"an activated kind alone":                       {[]step{quietEvent(EventActivated, 184.45)}, 0, nil, false},
		"both pending, a sale held kind":                {[]step{quietEvent(EventPending, 184.45), patternEvent(EventPending, 179.78), quietEvent(EventSaleHeld, 175.83)}, 179.78, both, false},
	} {
		st := rebuildState(withSteps(sixDeep(), tc.steps...))
		if st.slowDeclinePending != (tc.from != 0) || st.slowDeclinePendingFrom != tc.from || st.earlyTakeProfit != tc.latched || st.slowDeclineActivated || st.saleHeld ||
			!(len(st.pendingFeatures) == 0 && len(tc.features) == 0 || reflect.DeepEqual(st.pendingFeatures, tc.features)) {
			t.Errorf("%s: pending %v from %v on %v, latched %v, trailingBuy activated %v, sale held %v; want from %v on %v, latched %v, neither activated nor held", name, st.slowDeclinePending, st.slowDeclinePendingFrom, st.pendingFeatures, st.earlyTakeProfit, st.slowDeclineActivated, st.saleHeld, tc.from, tc.features, tc.latched)
		}
	}
}

// The activated and the sale held kinds belong to the shared action and were never
// written before the merge: a legacy gate's event of either kind names no features, is a
// kind the legacy folds do not have, and changes nothing — neither the pending state it
// follows nor the memory of the action. The same kinds written as the gate writes them now
// (they name features) move that memory alone, on a ladder pending by the legacy events as on
// one pending by the new ones, and leave the legacy states in force until a pending, a
// cancelled or a reset event that names features supersedes them, which clears the memory
// too.
func TestTheSharedActionsKindsAreNoLegacyKindAndTheNewOnesMoveItsMemoryAlone(t *testing.T) {
	withQuietDecline(t, true)
	withSwingDecline(t, true)
	both := []string{FeatureQuietDecline, FeatureSwingDecline}
	pending := []step{quietEvent(EventPending, 184.45), patternEvent(EventPending, 179.78)}
	activated := writes(ActivatedRow(179.78, -0.25, both...))
	held := writes(SaleHeldRow(179.78, -1.5, both...))
	for name, tc := range map[string]struct {
		steps           []step
		pending         bool
		activated, held bool
	}{
		"legacy pending alone":                                {pending, true, false, false},
		"an activated kind of the legacy gates":               {append(slices.Clone(pending), quietEvent(EventActivated, 175.83), patternEvent(EventActivated, 175.83)), true, false, false},
		"a sale held kind of the legacy gates":                {append(slices.Clone(pending), quietEvent(EventSaleHeld, 175.83), patternEvent(EventSaleHeld, 175.83)), true, false, false},
		"the activation as written now":                       {append(slices.Clone(pending), activated), true, true, false},
		"the held sale as written now":                        {append(slices.Clone(pending), held), true, false, true},
		"the held sale, then the activation":                  {append(slices.Clone(pending), held, activated), true, true, true},
		"the activation, then a cancelled that names them":    {append(slices.Clone(pending), activated, writes(CancelledRow(175.83, quietFinding(nil), swingFinding(nil)))), false, false, false},
		"the activation, then a reset that names them":        {append(slices.Clone(pending), held, activated, writes(ResetRow(179.78, quietFinding(nil), swingFinding(nil)))), false, false, false},
		"the activation, then a pending that names a decline": {append(slices.Clone(pending), held, activated, writes(PendingRow(175.83, swingFinding(nil)))), true, false, false},
	} {
		st := rebuildState(withSteps(sixDeep(), tc.steps...))
		if st.slowDeclinePending != tc.pending || st.slowDeclineActivated != tc.activated || st.saleHeld != tc.held {
			t.Errorf("%s: pending %v, trailingBuy activated %v, sale held %v; want %v, %v, %v", name, st.slowDeclinePending, st.slowDeclineActivated, st.saleHeld, tc.pending, tc.activated, tc.held)
		}
	}
}

// The first event that names features and makes a ladder pending or not pending
// supersedes the legacy states, whichever of them were pending: a new pending
// event stands for the features it names alone, a new cancelled or reset event
// ends the pending of both, and the legacy events written after it — a release
// rolled back, say — are ignored. A new latched event, a new sale and an event
// that makes nothing of the ladder supersede nothing.
func TestTheFirstNewEventSupersedesTheLegacyStates(t *testing.T) {
	withQuietDecline(t, true)
	withSwingDecline(t, true)
	withEarlyTakeProfit(t, true)
	quiet, swing := []string{FeatureQuietDecline}, []string{FeatureSwingDecline}
	both := []step{quietEvent(EventPending, 184.45), patternEvent(EventPending, 179.78)}
	for name, tc := range map[string]foldsTo{
		"a new pending on the swing decline":               {append(both, writes(PendingRow(175.83, swingFinding(nil)))), 175.83, swing, false},
		"a new pending on the quiet decline":               {append(both, writes(PendingRow(175.83, quietFinding(nil)))), 175.83, quiet, false},
		"a new pending that latched":                       {append(both, writes(PendingRow(175.83, indecisionFinding(nil), swingFinding(nil)))), 175.83, swing, true},
		"a new cancelled":                                  {append(both, writes(CancelledRow(175.83, quietFinding(nil), swingFinding(nil)))), 0, nil, false},
		"a new reset":                                      {append(both, writes(ResetRow(179.78, quietFinding(nil), swingFinding(nil)))), 0, nil, false},
		"a new cancelled, a legacy pending after it":       {append(both, writes(CancelledRow(175.83, quietFinding(nil))), quietEvent(EventPending, 171.97)), 0, nil, false},
		"a new pending, a legacy cancel after it":          {append(both, writes(PendingRow(175.83, swingFinding(nil))), quietEvent(EventCancelled, 171.97), patternEvent(EventCancelled, 171.97)), 175.83, swing, false},
		"a new pending, the legacy quiet pending after it": {append(both, writes(PendingRow(175.83, swingFinding(nil))), quietEvent(EventPending, 171.97)), 175.83, swing, false},
		"a new latched row supersedes nothing":             {append(both, writes(LatchedRow(179.78, indecisionFinding(indecisionReasons)))), 179.78, []string{FeatureQuietDecline, FeatureSwingDecline}, true},
		"a new sale supersedes nothing":                    {append(both, writes(ExitRow(sixDeep(), reasonSlowDecline, slowDeclineBand))), 179.78, []string{FeatureQuietDecline, FeatureSwingDecline}, false},
		"an event with an unknown kind supersedes nothing": {append(both, records(eventOf(GateSlowDecline, `{"event":"unknown","price":175.83,"features":["quietDecline"]}`))), 179.78, []string{FeatureQuietDecline, FeatureSwingDecline}, false},
	} {
		st := rebuildState(withSteps(sixDeep(), tc.steps...))
		if st.slowDeclinePending != (tc.from != 0) || st.slowDeclinePendingFrom != tc.from || st.earlyTakeProfit != tc.latched ||
			!(len(st.pendingFeatures) == 0 && len(tc.features) == 0 || reflect.DeepEqual(st.pendingFeatures, tc.features)) {
			t.Errorf("%s: pending %v from %v on %v, latched %v; want from %v on %v, latched %v", name, st.slowDeclinePending, st.slowDeclinePendingFrom, st.pendingFeatures, st.earlyTakeProfit, tc.from, tc.features, tc.latched)
		}
	}
}

// Each legacy state folds on its own feature's watch, switch and ladder, as it
// always did: the quiet decline's events on a ladder the quiet decline watches
// while it is on, the slow pattern's on a long spot ladder from
// swingdecline.ArmDepth while the swing decline is on, the
// indecision gate's on a ladder the early take profit watches while it is on.
// An event the watch does not admit is ignored, the other state's untouched.
func TestTheLegacyStatesFoldOnTheirOwnFeaturesWatch(t *testing.T) {
	steps := []step{quietEvent(EventPending, 184.45), patternEvent(EventPending, 179.78), legacyLatch(175.83)}
	trade := withSteps(sixDeep(), steps...)
	if st := rebuildState(trade); !reflect.DeepEqual(st.pendingFeatures, []string{FeatureQuietDecline, FeatureSwingDecline}) || !st.earlyTakeProfit || st.slowDeclinePendingFrom != 179.78 {
		t.Fatalf("fixture drifted: both pending and latched, got %+v", st)
	}

	withQuietDecline(t, false)
	if st := rebuildState(trade); !reflect.DeepEqual(st.pendingFeatures, []string{FeatureSwingDecline}) || st.slowDeclinePendingFrom != 179.78 || !st.earlyTakeProfit {
		t.Errorf("the quiet decline switched off, only the pattern stands pending, got %+v", st)
	}
	withQuietDecline(t, true)
	withSwingDecline(t, false)
	if st := rebuildState(trade); !reflect.DeepEqual(st.pendingFeatures, []string{FeatureQuietDecline}) || st.slowDeclinePendingFrom != 184.45 || !st.earlyTakeProfit {
		t.Errorf("the swing decline switched off, only the quiet decline stands pending from its own fill, got %+v", st)
	}
	withSwingDecline(t, true)
	withEarlyTakeProfit(t, false)
	if st := rebuildState(trade); st.earlyTakeProfit || len(st.pendingFeatures) != 2 {
		t.Errorf("the early take profit switched off, the latch is ignored and the pending reads on, got %+v", st)
	}
	withEarlyTakeProfit(t, true)

	shallow := testutil.LadderTrade(false, fills(max(swingdecline.ArmDepth-1, quietdecline.ArmDepth), "23:09:00")...)
	shallowFolded := rebuildState(withSteps(shallow, patternEvent(EventPending, 184.45)))
	if shallowFolded.swingDeclineWatched || shallowFolded.slowDeclinePending && reflect.DeepEqual(shallowFolded.pendingFeatures, []string{FeatureSwingDecline}) {
		t.Errorf("a ladder short of swingdecline.ArmDepth reads none of the pattern's events, got %+v", shallowFolded)
	}
	futures := sixDeep()
	futures.Strategy.TradeType = aggragates.Futures
	st := rebuildState(withSteps(futures, quietEvent(EventPending, 184.45), patternEvent(EventPending, 179.78), legacyLatch(175.83)))
	if st.slowDeclineWatched || st.slowDeclinePending || len(st.pendingFeatures) != 0 || st.earlyTakeProfit {
		t.Errorf("a futures ladder is watched by no feature, so it reads none of the legacy events, got %+v", st)
	}
	inverse := testutil.LadderTrade(true, fills(6, "23:09:00")...)
	if st := rebuildState(withSteps(inverse, quietEvent(EventPending, 184.45), patternEvent(EventPending, 179.78), legacyLatch(175.83))); st.slowDeclinePending || st.earlyTakeProfit {
		t.Errorf("an inverse ladder is watched by none, got %+v", st)
	}
}

// The legacy events drive everything a fold of the new ones does. A ladder
// pending on both legacy states resting in buy has its trailingBuy activated, the
// activation row naming both features, at a print under its break even and at none
// over it; once activated, as the engines leave it resting in trailingBuy, it trails from
// the activation print (a rise over the trail re-anchors, a print within the bounds
// holds) and is sold under the gate's one reason at a fall under the tolerance whatever
// it proposes, its sold row naming both features, and one that armed itself in
// trailingBuy is sold as well. Apply's
// judgement of its new fill is the gate's — judged from the price of the most recent
// pending event. The first held tick resets it with one row naming both features, and
// the take profit reads the newest fill for it, from break even up, as for a pending
// ladder of the gate, and the position price for a latched one.
func TestLegacyEventsDriveTheGateLikeTheNewOnes(t *testing.T) {
	withQuietDecline(t, true)
	withSwingDecline(t, true)
	withEarlyTakeProfit(t, true)
	legacy := withSteps(watchedTrade(), quietEvent(EventPending, slowDeclineLastFill), patternEvent(EventPending, slowDeclineLastFill), legacyLatch(slowDeclineLastFill))
	if st := rebuildState(legacy); !st.slowDeclinePending || st.slowDeclinePendingFrom != slowDeclineLastFill || !st.earlyTakeProfit || len(st.pendingFeatures) != 2 {
		t.Fatalf("fixture drifted: pending on both legacy states and latched, got %+v", st)
	}

	armed := armingPrice(legacy)
	both := []string{FeatureQuietDecline, FeatureSwingDecline}
	assertActivated(t, Apply(legacy, "trailingBuy", armed, slowDeclineBlock(false)), ActivatedRow(slowDeclineLastFill, -handLossPct(legacy, armed), both...))
	assertActivated(t, Apply(legacy, "", underTheBand, slowDeclineBlock(false)), ActivatedRow(slowDeclineLastFill, -handLossPct(legacy, underTheBand), both...))
	assertUntouched(t, applyAtAGain(legacy, "", slowDeclineBlock(false)), "")

	activated, _ := engineStep(legacy, "trailingBuy", armed, testutil.At("18:00:00"), slowDeclineBlock(false))
	if st := rebuildState(activated); !st.slowDeclinePending || !st.slowDeclineActivated || st.saleHeld || len(st.pendingFeatures) != 2 {
		t.Fatalf("the activation reads back on the ladder the legacy events left pending, got %+v", st)
	}
	if activated.PositionPrice != armed {
		t.Fatalf("the activation anchors the ladder at its print, got %v want %v", activated.PositionPrice, armed)
	}
	falls := priceAtMove(activated, armed, -(trailRow(activated).Tolerance + 1))
	assertTrailHeld(t, Apply(activated, "", armed, slowDeclineBlock(false)))
	assertReanchored(t, Apply(activated, "", slowDeclineBand, slowDeclineBlock(false)))
	sold := Apply(activated, "", falls, slowDeclineBlock(false))
	assertSold(t, sold, reasonSlowDecline)
	assertSold(t, Apply(restingIn(legacy, "trailingBuy"), "trailingBuy", armed, slowDeclineBlock(false)), reasonSlowDecline)
	exit := ExitRow(legacy, sold.Reason, falls)
	want := fmt.Sprintf("smartSell: take loss at slow decline %.2f (quiet decline and swing decline)", falls)
	if exit.Message != want || exit.Gate != GateSlowDecline || exit.Event != EventSold || !reflect.DeepEqual(exit.Features, []string{FeatureQuietDecline, FeatureSwingDecline}) {
		t.Errorf("the sold row names both features under the gate, got %+v, want %q", exit, want)
	}

	held := heldBy(legacy, testutil.At("18:05:00"))
	assertRow(t, Apply(held, "", slowDeclineBand+1, slowDeclineBlock(true)), ResetRow(slowDeclineLastFill, quietFinding(nil), swingFinding(nil)))

	price := betweenTheTakeProfits(t, pendingTrade())
	fromAverage := moveAgainst(price, ladder.AverageEntryPrice(legacy))
	latchedOnly := withSteps(watchedTrade(), legacyLatch(slowDeclineLastFill))
	for name, trade := range map[string]aggragates.Trades{"pending on both": legacy, "the quiet decline alone": withSteps(watchedTrade(), quietEvent(EventPending, slowDeclineLastFill)), "the pattern alone": withSteps(watchedTrade(), patternEvent(EventPending, slowDeclineLastFill))} {
		if !carriesTakeProfitEvent(trade) {
			t.Errorf("%s: the take profit's scan must find the legacy event the fold reads", name)
		}
		if got := TakeProfitPercentage(trade, price, fromAverage); got != moveAgainst(price, slowDeclineLastFill) {
			t.Errorf("%s: the take profit reads the newest fill, got %v", name, got)
		}
	}
	if !carriesTakeProfitEvent(latchedOnly) || TakeProfitPercentage(latchedOnly, price, 0) != moveAgainst(price, latchedOnly.PositionPrice) {
		t.Error("the legacy latch is found by the scan and the take profit reads the position price")
	}
	cancelled := withSteps(watchedTrade(), quietEvent(EventPending, slowDeclineLastFill), quietEvent(EventCancelled, slowDeclineLastFill))
	if !carriesTakeProfitEvent(cancelled) || TakeProfitPercentage(cancelled, price, fromAverage) != fromAverage {
		t.Error("a legacy cancel is found by the scan and ends the reading of the newest fill")
	}

	quietOnly := withSteps(watchedTrade(), quietEvent(EventPending, slowDeclineLastFill))
	patternOnly := withSteps(watchedTrade(), patternEvent(EventPending, slowDeclineLastFill))
	withQuietDecline(t, false)
	withSwingDecline(t, false)
	if carriesTakeProfitEvent(patternOnly) {
		t.Error("with the swing decline switched off the scan finds none of the slow pattern gate's events")
	}
	if !carriesTakeProfitEvent(quietOnly) || !carriesTakeProfitEvent(latchedOnly) {
		t.Error("the scan reads filings alone: the slow decline gate's filing still can hold a latch while the early take profit is on")
	}
	withEarlyTakeProfit(t, false)
	if carriesTakeProfitEvent(quietOnly) || carriesTakeProfitEvent(latchedOnly) {
		t.Error("with every feature switched off the scan finds no event")
	}
}
