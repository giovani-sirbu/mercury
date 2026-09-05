package cooldown

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

var trade25858 = testutil.Trade25858()

func fold(placements ...time.Time) depthSpacingState {
	return depthSpacingEligibleFrom(depthFills(testutil.DepthTrade(placements...)))
}

// The rule the second depth used to escape. Seeding the fold at the first
// fill meant the gate had nothing to measure until two entries existed, so
// depth 2 always went through — on trade 32309 it landed 7m25s after the
// first fill, and the rest of the cascade was built on it.
func TestDepthSpacingHoldsTheSecondDepthFromTheFirstFill(t *testing.T) {
	state := fold(trade25858[0])

	if state.step != 1 {
		t.Errorf("step = %d, want 1 — one fill already carries a base hold", state.step)
	}
	if state.hold != DepthSpacingBaseHold {
		t.Errorf("hold = %s, want the base %s", state.hold, DepthSpacingBaseHold)
	}
	want := testutil.At("13:41:08").Add(DepthSpacingBaseHold) // 13:56:08
	if !state.eligibleFrom.Equal(want) {
		t.Fatalf("eligibleFrom = %s, want %s", state.eligibleFrom, want)
	}
	if !testutil.At("13:48:33").Before(state.eligibleFrom) {
		t.Fatal("the 13:48:33 depth of trade 32309 must fall inside the hold")
	}
}

// Filling at or right after the expiry means the drop is still running, so
// the next hold doubles. The gap is measured from the previous hold's EXPIRY,
// not the previous fill — measured from the fill it would be >= the hold by
// construction and the rule could never escalate.
func TestDepthSpacingDoublesWhenADepthFillsTheInstantTheHoldLifts(t *testing.T) {
	first := testutil.At("09:00:00")
	expiry := first.Add(DepthSpacingBaseHold)
	second := depthSpacingHoldFor(2)

	state := fold(first, expiry)
	if state.step != 2 || state.hold != second {
		t.Fatalf("step %d hold %s, want step 2 hold %s", state.step, state.hold, second)
	}
	if want := expiry.Add(second); !state.eligibleFrom.Equal(want) {
		t.Fatalf("eligibleFrom = %s, want %s", state.eligibleFrom, want)
	}
}

// A ladder that waited out a full window is no longer in the cascade, so the
// escalation goes back to base. Carrying the count forever would hand a
// four-hour hold to a trade whose only fast pair happened weeks earlier.
func TestDepthSpacingResetsTheEscalationAfterARealPause(t *testing.T) {
	first := testutil.At("09:00:00")
	fast := first.Add(DepthSpacingBaseHold)                       // escalates to step 2
	slow := fast.Add(depthSpacingHoldFor(2) + DepthSpacingWindow) // a full window past that hold's expiry

	if got := fold(first, fast).step; got != 2 {
		t.Fatalf("step after the fast depth = %d, want 2", got)
	}
	state := fold(first, fast, slow)
	if state.step != 1 || state.hold != DepthSpacingBaseHold {
		t.Fatalf("after a real pause: step %d hold %s, want step 1 hold %s", state.step, state.hold, DepthSpacingBaseHold)
	}
	if want := slow.Add(DepthSpacingBaseHold); !state.eligibleFrom.Equal(want) {
		t.Fatalf("eligibleFrom = %s, want %s", state.eligibleFrom, want)
	}
}

// The recorded cascade end to end. Every one of its seven depths landed
// inside the hold it had earned, so nothing resets and the escalation runs to
// the ceiling: the eighth depth is parked into the evening instead of being
// on the book by 16:40.
func TestDepthSpacingHoldsTheRecordedTrade25858Cascade(t *testing.T) {
	state := fold(trade25858...)

	if want := testutil.At("16:39:22").Add(4 * time.Hour); !state.eligibleFrom.Equal(want) { // 20:39:22
		t.Fatalf("eligibleFrom = %s, want %s", state.eligibleFrom, want)
	}
	if state.step != 7 {
		t.Fatalf("step = %d, want 7 — every depth of the recorded ladder is inside its hold", state.step)
	}
}

// Two base holds apart is genuinely spaced: each depth lands a full window
// past the previous expiry, so the escalation never leaves step 1 and no
// depth is ever parked.
func TestDepthSpacingNeverHoldsALadderTwoBaseHoldsApart(t *testing.T) {
	start := testutil.At("09:00:00")
	var placements []time.Time
	for i := 0; i < 7; i++ {
		placements = append(placements, start.Add(time.Duration(i)*2*DepthSpacingBaseHold))
	}

	state := fold(placements...)
	if state.step != 1 || state.hold != DepthSpacingBaseHold {
		t.Fatalf("a well-spaced ladder must stay at base: step %d hold %s", state.step, state.hold)
	}
	last := placements[len(placements)-1]
	if want := last.Add(DepthSpacingBaseHold); !state.eligibleFrom.Equal(want) {
		t.Fatalf("eligibleFrom = %s, want %s", state.eligibleFrom, want)
	}
	// The next depth at the same cadence is already past the expiry.
	if next := last.Add(2 * DepthSpacingBaseHold); next.Before(state.eligibleFrom) {
		t.Error("the next depth at this cadence must not be parked")
	}
}

// Doubling from the base hold reaches four hours at the third consecutive fast
// depth (60m, 2h, 4h). The hold clamps there and stays clamped: past four
// hours the gate would make the trade sit out the bottom of the move.
func TestDepthSpacingClampsTheHoldAtFourHours(t *testing.T) {
	for _, c := range []struct {
		step int
		want time.Duration
	}{
		{1, time.Hour}, {2, 2 * time.Hour}, {3, 4 * time.Hour},
		{4, depthSpacingMaxHold}, {5, depthSpacingMaxHold},
		{6, depthSpacingMaxHold}, {9, depthSpacingMaxHold}, {40, depthSpacingMaxHold},
	} {
		if got := depthSpacingHoldFor(c.step); got != c.want {
			t.Errorf("depthSpacingHoldFor(%d) = %s, want %s", c.step, got, c.want)
		}
	}
	if depthSpacingHoldFor(0) != 0 {
		t.Error("no ladder at all earns no hold")
	}

	// The same clamp through the fold: every depth fills the instant the
	// previous hold lifts, so nothing ever resets the escalation.
	placements := []time.Time{testutil.At("09:00:00")}
	state := fold(placements...)
	for i := 0; i < 6; i++ {
		placements = append(placements, state.eligibleFrom)
		state = fold(placements...)
	}
	if state.step != 7 || state.hold != depthSpacingMaxHold {
		t.Fatalf("six consecutive fast depths = step %d hold %s, want step 7 hold %s", state.step, state.hold, depthSpacingMaxHold)
	}
	last := placements[len(placements)-1]
	if want := last.Add(depthSpacingMaxHold); !state.eligibleFrom.Equal(want) {
		t.Fatalf("eligibleFrom = %s, want %s", state.eligibleFrom, want)
	}
}

// A depth whose placement stamp was never persisted voids the whole read:
// live-testing memory trades arrive exactly like this.
func TestDepthFillsVoidTheReadOnAnUnstampedDepth(t *testing.T) {
	unstamped := testutil.DepthTrade(trade25858[0], trade25858[1])
	unstamped.History[1].CreatedAt = time.Time{}
	if fills := depthFills(unstamped); fills != nil {
		t.Fatalf("an unstamped depth must void the read, got %v", fills)
	}
}

// Partial fills update the same exchange order and are one depth, not two —
// the membership rule mirrors ladder.CountFilledEntries. Accounting rows
// (an impasse child's profit marked onto the parent at the sentinel price)
// are bookkeeping and never a depth either.
func TestDepthFillsCountDistinctOrdersAndSkipAccountingRows(t *testing.T) {
	trade := testutil.DepthTrade(trade25858[0], trade25858[1])
	trade.History = append(trade.History,
		// A partial top-up of depth 2, hours later: same order id.
		aggragates.TradesHistory{Type: "BUY", Quantity: 0.5, Price: 99, OrderId: 2, CreatedAt: testutil.At("18:00:00")},
		// A child's profit transfer at the sentinel price.
		aggragates.TradesHistory{Type: "BUY", Quantity: 3, Price: 1e-13, OrderId: 77, CreatedAt: testutil.At("18:30:00")},
		// The exit leg of a long is not an entry.
		aggragates.TradesHistory{Type: "SELL", Quantity: 2, Price: 120, OrderId: 78, CreatedAt: testutil.At("19:00:00")},
	)

	fills := depthFills(trade)
	if len(fills) != 2 {
		t.Fatalf("expected 2 depths, got %v", fills)
	}
	if !fills[0].At.Equal(trade25858[0]) || !fills[1].At.Equal(trade25858[1]) {
		t.Fatalf("depths = %v, want the two placements", fills)
	}
	// The price travels with the stamp: the release leg measures from the
	// newest fill, so the wrong row here would discount from the wrong price.
	if fills[1].Price != 99 {
		t.Errorf("newest fill price = %v, want the depth-2 price 99", fills[1].Price)
	}
}

// History that arrives out of order still folds correctly: the ladder is
// sorted before it is read.
func TestDepthFillsSortRehydratedHistory(t *testing.T) {
	trade := testutil.DepthTrade(trade25858[1], trade25858[0])

	fills := depthFills(trade)
	if len(fills) != 2 || !fills[0].At.Equal(trade25858[0]) || !fills[1].At.Equal(trade25858[1]) {
		t.Fatalf("fills = %v, want them oldest first", fills)
	}
}

// The escalation counter is not the trade's depth: a ladder with one real
// pause has five filled entries and a lower escalation step. The row must
// print the former.
func TestDepthSpacingStepLagsTheLadderDepthAfterARealPause(t *testing.T) {
	// One real pause — a full window past the first hold's expiry, which is
	// what the fold calls a pause — and then three fast depths behind it.
	start := testutil.At("09:00:00")
	pause := start.Add(DepthSpacingBaseHold + DepthSpacingWindow)
	placements := []time.Time{
		start, pause,
		pause.Add(5 * time.Minute), pause.Add(10 * time.Minute), pause.Add(15 * time.Minute),
	}
	trade := testutil.DepthTrade(placements...)

	state := depthSpacingEligibleFrom(depthFills(trade))
	if state.step >= ladder.CountFilledEntries(trade) {
		t.Fatalf("step %d must lag the ladder depth %d after a pause", state.step, ladder.CountFilledEntries(trade))
	}
	if got := ladder.CountFilledEntries(trade); got != 5 {
		t.Fatalf("CountFilledEntries = %d, want 5", got)
	}
}
