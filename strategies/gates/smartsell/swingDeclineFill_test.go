package smartsell

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// stairDecline is the staircase carried on down, so a ladder's later fills read
// the same decline: it holds at the sixth fill's bar (36) from the first depth.
var stairDecline = append(slices.Clone(stairTurns), 79, 73.5, 75.5, 70, 72, 66.5)

// stairRally is the staircase with a rally in place of its last leg, carried
// on down again in a second staircase from the 104 on bar 40. It breaks at the
// fifth fill's bar (30) and at the sixth's bar 36, and reads a decline again at
// bar 67, where a ladder's sixth fill reads it afresh: its first down leg, the
// 104 to the 97, is three bars and does not count.
var stairRally = append(slices.Clone(stairBounce), 104, 97, 99, 92.5, 94.5, 88, 90, 84, 86, 80, 82)

// A fill the series has not closed the bar of is not judged, and an unjudged
// fill acts on nothing: tick after tick, whatever the ladder proposes and in whatever
// state it rests, at a print under its break even too, the ladder is left alone — its
// trigger of the shared action does not stand — until the tick whose series has closed
// the fill's bar, which confirms the exit and asks the action on that same tick: the
// ladder in buy has its trailingBuy activated at a print under its break even, with the
// activation row after the confirmed one, the one in trailingBuy is sold with the
// confirmed row beside the sale. The ladder is judged from then on and the action goes
// on every tick, with no second row. The quiet decline's own reading, taken and missed
// on every tick the window is read, never cancels the fill ahead of the bar.
func TestSwingDeclineAnUnjudgedFillActsOnNothingUntilItsBarCloses(t *testing.T) {
	withBothDeclines(t)
	ticked, open := stairFifthFill(t), stairBlock(stairTurns, 29)
	for _, now := range []time.Time{stairOpen(30).Add(20 * time.Minute), stairOpen(30).Add(40 * time.Minute)} {
		for _, position := range everyProposal() {
			var got Result
			ticked, got = engineTick(ticked, position, armingPrice(ticked), now, withBlock(open))
			assertUntouched(t, got, position)
		}
	}
	if st := rebuildState(ticked); !st.slowDeclinePending || st.slowDeclinePendingFrom != stairPrices[3] || !slowDeclineFillUnjudged(ticked, st) || slowDeclineTriggered(ticked, st) {
		t.Fatalf("the ladder waits pending from its fourth fill with the fifth unjudged, acting on nothing, got %+v", rebuildState(ticked))
	}
	assertSoldNever(t, ticked, withBlock(open))

	closed := stairFifthBlock()
	confirmed := ConfirmedRow(stairPrices[4], swingFinding(stairReading(t, ticked, closed, true).Reasons))
	assertActedOn(t, ticked, withBlock(closed), confirmed)
	judged := withRow(ticked, confirmed, stairOpen(31))
	if st := rebuildState(judged); st.slowDeclinePendingFrom != stairPrices[4] || !slowDeclineTriggered(judged, st) {
		t.Fatalf("the confirmation moves the ladder to its fifth fill, judged, got %+v", st)
	}
	assertActedOn(t, judged, withBlock(closed))
}

// A pending ladder whose newest fill's bar is older than the first bar the
// series serves — the window cannot be read at all — is cancelled, not waited
// on: the row names why nothing was read under the swing decline alone, the one
// feature the ladder is pending on (a new fill is judged on that one, whatever
// else watches the ladder), the latch the swing decline took stays, and the
// shared action does nothing on that tick, the ladder no longer pending: it
// activates no trailingBuy and sells nothing, the proposal an arming included.
func TestSwingDeclineCancelsAFillOlderThanTheServedSeries(t *testing.T) {
	withBothDeclines(t)
	later := stairBlock(stairDecline, 42)
	later.SeriesOpens, later.SeriesCloses, later.SeriesHighs, later.SeriesLows = later.SeriesOpens[31:], later.SeriesCloses[31:], later.SeriesHighs[31:], later.SeriesLows[31:]
	if later.SeriesOpens[0] <= stairOpen(30).UnixMilli() {
		t.Fatal("fixture drifted: the series must start after the fifth fill's bar")
	}
	fifth := stairFifthFill(t)
	ticked, got := engineTick(fifth, "trailingBuy", armingPrice(fifth), stairOpen(43), withBlock(later))
	unreadable := []string{"depth 5, no window between the fills is readable in the 1h bars served"}
	assertRow(t, got, CancelledRow(stairPrices[4], swingFinding(unreadable)))
	if !slices.Equal(got.Rows[0].Reasons, unreadable) {
		t.Errorf("the cancel names why nothing was read, got %q", got.Rows[0].Reasons)
	}
	assertNoSale(t, got, "trailingBuy")
	if st := rebuildState(ticked); st.slowDeclinePending || !st.swingDeclineWatched || !st.earlyTakeProfit {
		t.Fatalf("watched, no longer pending and still latched, got %+v", st)
	}
}

// A cancelled ladder is made pending again by the trigger alone, on a later
// fill (the cancel naming the swing decline alone, the feature it was pending
// on): the series that has just closed the sixth fill's bar writes the pending
// row, and so does the series three bars past it — the swing decline reads every
// tick, to the last closed bar — and the latch the swing decline took at the
// fourth fill is not written a second time. The ticks that write the pending row
// run at a print over the ladder's break even (gainPrint) with a buy order resting, where the ladder in buy
// is activated nothing and the row stands alone.
func TestSwingDeclineACancelledLadderGoesPendingAgainOnAFreshRead(t *testing.T) {
	fifth := stairFifthFill(t)
	rally := stairBlock(stairRally, 30)
	breaks := stairReading(t, fifth, rally, false).Breaks
	cancelled, got := engineTick(fifth, "", underTheBand, stairOpen(31), withBlock(rally))
	assertRow(t, got, CancelledRow(stairPrices[4], swingFinding(breaks)))
	if st := rebuildState(cancelled); st.slowDeclinePending || !st.earlyTakeProfit {
		t.Fatalf("fixture drifted: cancelled and latched, got %+v", st)
	}

	// The sixth fill lands in the first bar the second staircase reads a decline as of: its first
	// down leg (104 to 97) is three bars and does not count, so a second one counts only later.
	var sixth aggragates.Trades
	sixthBar := -1
	for bar := 36; bar < len(stairRally)*6 && sixthBar < 0; bar++ {
		filled := withFill(cancelled, stairPrices[5], stairOpen(bar).Add(17*time.Minute))
		if _, holds := readsAsOf(filled, stairBlock(stairRally, bar)); holds {
			sixth, sixthBar = filled, bar
		}
	}
	if sixthBar != 67 { // the profile is pinned (TestMain), so no bar, or another, means the rule or the staircase moved
		t.Fatalf("fixture drifted: the second staircase must first read a decline as of bar 67 for the sixth fill, got bar %d", sixthBar)
	}
	fresh := stairBlock(stairRally, sixthBar)
	pending, got := tickAtAGain(sixth, "", stairOpen(sixthBar+1), withBlock(fresh))
	assertRow(t, got, PendingRow(stairPrices[5], swingFinding(stairReadingAsOfNow(t, sixth, fresh, true).Reasons)))
	if st := rebuildState(pending); !st.slowDeclinePending || st.slowDeclinePendingFrom != stairPrices[5] {
		t.Errorf("pending from the sixth fill, got %+v", st)
	}
	later := stairBlock(stairRally, sixthBar+3)
	assertRow(t, applyAtAGain(sixth, "", withBlock(later)), PendingRow(stairPrices[5], swingFinding(stairReadingAsOfNow(t, sixth, later, true).Reasons)))
}

// The fill that takes a pending ladder to its last depth is never judged, even
// when the series it is served would break the swing decline: the exit stays
// pending from where it was, no row is written, and the shared action goes on every
// tick: the trailingBuy of the ladder in buy activated, the ladder in trailingBuy sold.
func TestSwingDeclineTheLastDepthFillIsNotJudgedWhateverTheSeriesReads(t *testing.T) {
	withBothDeclines(t)
	eighth := withFill(withRows(stairLadder(0, 9, 15, 24, 30, 36, 42), PendingRow(stairPrices[6], swingFinding(nil))), stairPrices[7], stairOpen(48).Add(time.Minute))
	breaking := stairBlock([]float64{100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200, 210, 220, 230, 240, 250, 260}, 48)
	if _, holds := swingdecline.Trigger(swingFills(rebuildState(eighth).fills), series.SeriesOf(breaking), series.TrendOf(breaking)); holds {
		t.Fatal("fixture drifted: the series must break the swing decline at the eighth fill")
	}
	assertActedOn(t, eighth, withBlock(breaking))
}

// ExitReached is Apply's sale on the empty proposal and an activation is no sale. So
// for the swing decline it reads false on a ladder resting in buy at every print,
// the trailingBuy it activates being no exit, and on a ladder resting in trailingBuy it
// follows the sale: true on the tick a ladder goes pending and on the tick a new
// fill's judgement confirms the exit, where Apply sells, and false on the tick that
// cancels the exit and on one whose new fill is not judged yet, where it sells
// nothing. The proposal of the tick changes none of it.
func TestSwingDeclineExitReachedFollowsTheSaleAndNeverAnActivation(t *testing.T) {
	withBothDeclines(t)
	fourth, fifth := stairFourth(), stairFifthFill(t)
	for name, tc := range map[string]struct {
		trade aggragates.Trades
		block aggragates.SmartSellIndicators
		sells bool
	}{
		"going pending":          {fourth, stairFourthBlock(), true},
		"a confirming judgement": {fifth, stairFifthBlock(), true},
		"a cancelling judgement": {fifth, stairBlock(stairRally, 30), false},
		"a fill not judged yet":  {fifth, stairBlock(stairTurns, 29), false},
	} {
		resting := restingIn(tc.trade, "trailingBuy")
		for _, tick := range everyTick(tc.trade) {
			if ExitReached(tc.trade, tick.price, tc.block) {
				t.Errorf("%s, resting in buy, at %v: an activation is no exit, got true", name, tick.price)
			}
			sold := Apply(resting, tick.position, tick.price, withBlock(tc.block)).Reason == reasonSlowDecline
			if sold != tc.sells || ExitReached(resting, tick.price, tc.block) != sold {
				t.Errorf("%s, resting in trailingBuy, on %q at %v: sold %v, ExitReached %v, want both %v", name, tick.position, tick.price, sold, ExitReached(resting, tick.price, tc.block), tc.sells)
			}
		}
	}
}

// The judgement of a pending ladder's new fill, on every series the swing decline
// can be served: confirmed when its window and trend read again, cancelled when a
// rally breaks it or the series no longer holds its window, and waited on — no
// row, nothing done — when the fill's bar is not closed yet or no series is served
// at all. A ladder pending on the swing decline alone is judged on that reading
// only (pendsOn), so with no series nothing reads its fill and it waits rather
// than cancels on nothing, the quiet decline's window read beside it or not. The
// judgement is read at a print over the ladder's break even (gainPrint), on every
// proposal, where the tick writes the gate's row alone. The shared action then acts
// on a confirmation and on no other tick, in the state of the ladder: the trailingBuy
// activated for one in buy, the sale for one in trailingBuy. The latch the swing decline
// took at the fourth fill is never written a second time.
func TestSwingDeclineJudgesTheNewFillOfAPendingLadder(t *testing.T) {
	withBothDeclines(t)
	fifth := stairFifthFill(t)
	if st := rebuildState(fifth); !st.slowDeclinePending || !slowDeclineFillUnjudged(fifth, st) || slowDeclineTriggered(fifth, st) {
		t.Fatalf("fixture drifted: pending from the fourth fill, the fifth unjudged and acted on by nobody, got %+v", st)
	}
	late := stairBlock(append(slices.Clone(stairTurns), 84, 83, 82, 81), 42)
	// A series that starts after the third fill's bar holds no bar an earlier fill
	// sits in, so no window between the fills is readable whatever depths apart
	// the rule asks them.
	older := stairFifthBlock()
	older.SeriesOpens, older.SeriesCloses, older.SeriesHighs, older.SeriesLows = older.SeriesOpens[16:], older.SeriesCloses[16:], older.SeriesHighs[16:], older.SeriesLows[16:]
	for name, tc := range map[string]struct {
		block aggragates.SmartSellIndicators
		event string
	}{
		"confirmed":                           {stairFifthBlock(), EventPending},
		"confirmed twelve bars late":          {late, EventPending},
		"broken by a rally":                   {stairBlock(stairBounce, 30), EventCancelled},
		"a window the series no longer holds": {older, EventCancelled},
		"the fill's bar not closed yet":       {stairBlock(stairTurns, 29), ""},
		"no series served":                    {windowReadAlone().SmartSell, ""},
		"no window read, no series served":    {nothingServed().SmartSell, ""},
	} {
		var judged []Row
		for _, position := range everyProposal() {
			got := applyAtAGain(fifth, position, withBlock(tc.block))
			if (len(got.Rows) == 0) != (tc.event == "") || len(got.Rows) > 1 {
				t.Errorf("%s on %q: want a %q row at the fifth fill, got %+v", name, position, tc.event, got.Rows)
				continue
			}
			if len(got.Rows) == 1 && (got.Rows[0].Event != tc.event || got.Rows[0].Gate != GateSlowDecline || got.Rows[0].Price != stairPrices[4]) {
				t.Errorf("%s on %q: want a %q row at the fifth fill, got %+v", name, position, tc.event, got.Rows)
			}
			if len(got.Rows) == 1 && slices.Contains(got.Rows[0].Features, FeatureEarlyTakeProfit) {
				t.Errorf("%s on %q: the latch is taken and never written twice, got %+v", name, position, got.Rows[0])
			}
			if got.Position != position || got.Reason != "" || got.Activated {
				t.Errorf("%s on %q: over the break even the ladder in buy is neither activated nor sold, got %+v", name, position, got)
			}
			judged = got.Rows
		}
		t.Run(name, func(t *testing.T) {
			if tc.event == EventPending {
				assertActedOn(t, fifth, withBlock(tc.block), judged...)
				return
			}
			assertSoldNever(t, fifth, withBlock(tc.block), judged...)
		})
	}

	_, got := engineTick(fifth, "trailingBuy", armingPrice(fifth), stairOpen(60), withBlock(older))
	if want := []string{"depth 5, no window between the fills is readable in the 1h bars served"}; len(got.Rows) != 1 || !slices.Equal(got.Rows[0].Reasons, want) {
		t.Fatalf("the cancel names why nothing was read, got %+v", got.Rows)
	}
	broken := stairBlock(stairBounce, 30)
	cancelled, _ := engineTick(fifth, "trailingBuy", armingPrice(fifth), stairOpen(60), withBlock(broken))
	if st := rebuildState(cancelled); !st.swingDeclineWatched || st.slowDeclinePending || !st.earlyTakeProfit {
		t.Fatalf("a cancelled ladder is watched, not pending, and still latched, got %+v", st)
	}
	assertSoldNever(t, cancelled, withBlock(broken))
}

// The fill that takes a pending swing ladder to its last depth is never judged:
// it stays pending from where it was and the shared action goes at once, on the first
// tick and on every one after it, whatever is served: the trailingBuy of the ladder in
// buy activated, the ladder in trailingBuy sold.
func TestSwingDeclineNeverJudgesTheFillThatReachesTheLastDepth(t *testing.T) {
	withBothDeclines(t)
	seventh := withRows(stairLadder(0, 9, 15, 24, 30, 36, 42), PendingRow(stairPrices[6], swingFinding(nil)))
	eighth := withFill(seventh, stairPrices[7], stairOpen(48).Add(time.Minute))
	if st := rebuildState(eighth); !st.slowDeclinePending || slowDeclineFillUnjudged(eighth, st) {
		t.Fatalf("fixture drifted: the last-depth fill must read as judged, got %+v", st)
	}
	assertActedOn(t, eighth, windowReadAlone())
	assertActedOn(t, eighth, nothingServed())
}

// The shared action is the same one for a ladder the swing decline made pending, and it
// is read off the ladder: within both its bounds it acts, the trailingBuy of the ladder in
// buy activated and the ladder in trailingBuy sold, and past either it WAITS — the depth
// row when the ladder has too few depths left, the loss row when its loss is over the
// cap — with the swing decline named on the row as the feature the ladder is pending
// on, at the newest fill, once per episode, nothing activated or sold and the proposal
// going on as the ladder made it. Whatever the ladder proposes. The bounds are set from
// the ladder (its ceiling and the loss its arming print takes), so the case holds under
// any cap and depth bound the tree is tuned to.
func TestSwingDeclineTheActionWithinBothBoundsGoesAndPastEitherWaitsOnTheSwingsRow(t *testing.T) {
	withBothDeclines(t)
	_, holds := swingdecline.Trigger(swingFills(rebuildState(stairFourth()).fills), series.SeriesOf(stairFourthBlock()), series.TrendOf(stairFourthBlock()))
	needsAScenario(t, rebuildState(stairFourth()).swingDeclineWatched && holds, "a staircase that reads a decline at its fourth fill's bar")
	pending := stairPending(t)
	filled, ceiling, armed := ladder.CountFilledEntries(pending), ladder.ConfiguredDepths(pending), armingPrice(pending)
	loss := handLossPct(pending, armed)
	if ceiling <= filled || loss <= 0 {
		t.Fatalf("fixture drifted: pending short of its ceiling, its arming print under break even, got %d of %d at a loss of %v", filled, ceiling, loss)
	}

	withSaleIgnoreLastDepths(t, ceiling-filled)
	withAllowedLoss(t, loss+1)
	activation := ActivatedRow(stairPrices[3], -loss, FeatureSwingDecline)
	for _, position := range everyProposal() {
		assertActivated(t, Apply(pending, position, armed, nothingServed()), activation)
		assertSold(t, Apply(restingIn(pending, "trailingBuy"), position, armed, nothingServed()), reasonSlowDecline)
	}

	withSaleIgnoreLastDepths(t, ceiling-filled+1)
	heldAtDepth := SaleHeldAtDepthRow(stairPrices[3], filled, ceiling, FeatureSwingDecline)
	for _, state := range []string{"buy", "trailingBuy"} {
		for _, position := range everyProposal() {
			assertWaits(t, Apply(restingIn(pending, state), position, armed, nothingServed()), position, heldAtDepth)
		}
	}
	written, got := engineTick(pending, "trailingBuy", armed, stairOpen(26), nothingServed())
	assertRow(t, got, heldAtDepth)
	if row := got.Rows[0]; !slices.Equal(row.Features, []string{FeatureSwingDecline}) || row.Gate != GateSlowDecline || row.Event != EventSaleHeld || row.Price != stairPrices[3] {
		t.Errorf("the held row is the slow decline gate's, at the newest fill, naming the swing decline the ladder is pending on, got %+v", row)
	}
	if st := rebuildState(written); !st.saleHeld || st.saleHeldRowDue() || !st.slowDeclinePending {
		t.Fatalf("the held row folds back as the sale held in the episode, the ladder still pending, got %+v", st)
	}
	assertSoldNever(t, written, nothingServed())

	withSaleIgnoreLastDepths(t, ceiling-filled)
	withAllowedLoss(t, loss/2)
	heldByLoss := SaleHeldRow(stairPrices[3], -loss, FeatureSwingDecline)
	for _, state := range []string{"buy", "trailingBuy"} {
		for _, position := range everyProposal() {
			assertWaits(t, Apply(restingIn(pending, state), position, armed, nothingServed()), position, heldByLoss)
		}
	}
}
