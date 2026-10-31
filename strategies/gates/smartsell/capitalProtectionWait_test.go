package smartsell

import (
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
)

// The wait after the trigger (capitalprotection.WaitAfterTrigger) through ApplyAt, the clock the
// engines hand the overlay: the tick the trend reads against the ladder on opens the episode with
// the triggered row alone, which names the wait, and until the wait has passed from that row the
// shared action is not asked — no held row, no activation, no sale of a self-armed trailingBuy — the
// proposal passing through, the ladder's own closes with it, and the predicate (CapitalProtectionActs)
// reading nothing to act on. The first tick at the wait is judged as before. The stamp is the triggered
// event's, read back off the events, so any later tick knows the wait.
func TestCapitalProtectionWaitsAfterItsTriggerBeforeTheSharedActionIsAsked(t *testing.T) {
	capitalProtectionAlone(t)
	withCapitalProtectionWaitAfterTrigger(t, 4*time.Hour)
	start := capitalProtectionLadder(false)
	ai, fill := withBlock(solBlock()), start.PositionPrice
	atTheDrawdown, oneStepLower := drawdownPrices(t, start)
	opened := testutil.At("13:00:00")
	triggered := capitalProtectionTriggered(start)
	held := CapitalProtectionSaleHeldRow(fill, -handLossPct(start, oneStepLower))
	activation := CapitalProtectionActivatedRow(fill, -handLossPct(start, atTheDrawdown))
	if !slices.Contains(triggered.Reasons, "the sale waits 4h0m0s from the trigger") {
		t.Fatalf("the triggered row must name the wait, got %q", triggered.Reasons)
	}

	assertWaits(t, ApplyAt(start, "", oneStepLower, ai, opened), "", triggered)
	trade := withRow(start, triggered, opened)
	if st := rebuildState(trade); !st.capitalProtectionActive || !st.capitalProtectionTriggeredAt.Equal(opened) {
		t.Fatalf("the triggered event stamps the episode, got %+v", st)
	}
	for _, now := range []time.Time{opened, opened.Add(time.Minute), opened.Add(4*time.Hour - time.Second)} {
		for _, position := range addSideProposals {
			assertUntouched(t, ApplyAt(trade, position, atTheDrawdown, ai, now), position)
			assertUntouched(t, ApplyAt(trade, position, oneStepLower, ai, now), position)
		}
		assertUntouched(t, ApplyAt(restingIn(trade, "trailingBuy"), "", atTheDrawdown, ai, now), "")
		assertUntouched(t, ApplyAt(trade, "takeProfit", atTheDrawdown, ai, now), "takeProfit")
		if CapitalProtectionActs(trade, atTheDrawdown, ai.SmartSell, now) {
			t.Fatalf("at %v, within the wait, the predicate must read nothing to act on", now)
		}
	}

	past := opened.Add(4 * time.Hour)
	assertWaits(t, ApplyAt(trade, "", oneStepLower, ai, past), "", held)
	assertActivated(t, ApplyAt(trade, "", atTheDrawdown, ai, past), activation)
	assertSold(t, ApplyAt(restingIn(trade, "trailingBuy"), "", atTheDrawdown, ai, past), reasonCapitalProtection)
	if !CapitalProtectionActs(trade, atTheDrawdown, ai.SmartSell, past) {
		t.Fatal("at the wait the predicate must read the activation")
	}
	assertActivated(t, Apply(trade, "", atTheDrawdown, ai), activation)

	withCapitalProtectionWaitAfterTrigger(t, 0)
	assertActivated(t, ApplyAt(trade, "", atTheDrawdown, ai, opened), activation)
	if slices.Contains(capitalProtectionTriggered(start).Reasons, "the sale waits") {
		t.Fatal("with no wait the triggered row names none")
	}
}

// A fill past the ceiling resets the episode and the next trend reading re-triggers it, which
// restarts the wait from the new triggered row; an episode a held or activated event opened, with
// no triggered event to stamp it, waits nothing, and so does one that was already activated.
func TestCapitalProtectionWaitRestartsOnARetriggerAndSkipsAnUnstampedEpisode(t *testing.T) {
	capitalProtectionAlone(t)
	withCapitalProtectionWaitAfterTrigger(t, 4*time.Hour)
	start := capitalProtectionLadder(false)
	ai := withBlock(solBlock())
	opened := testutil.At("13:00:00")
	trade := withRow(start, capitalProtectionTriggered(start), opened)

	deeper := fundsRefused(withFill(trade, 97, opened.Add(2*time.Hour)))
	retriggered := opened.Add(5 * time.Hour)
	within, _ := firstPriceWithinTheDrawdown(t, deeper)
	got := ApplyAt(deeper, "", within, ai, retriggered)
	assertWaits(t, got, "", CapitalProtectionResetRow(97), capitalProtectionTriggered(deeper))
	deeper = withRow(withRow(deeper, got.Rows[0], retriggered), got.Rows[1], retriggered)
	assertUntouched(t, ApplyAt(deeper, "", within, ai, retriggered.Add(3*time.Hour)), "")
	assertActivated(t, ApplyAt(deeper, "", within, ai, retriggered.Add(4*time.Hour)), CapitalProtectionActivatedRow(97, -handLossPct(deeper, within)))

	atTheDrawdown, oneStepLower := drawdownPrices(t, start)
	unstamped := withRow(start, CapitalProtectionSaleHeldRow(start.PositionPrice, -handLossPct(start, oneStepLower)), opened)
	if st := rebuildState(unstamped); !st.capitalProtectionActive || !st.capitalProtectionTriggeredAt.IsZero() {
		t.Fatalf("a held event opens the episode with no stamp, got %+v", st)
	}
	assertActivated(t, ApplyAt(unstamped, "", atTheDrawdown, ai, opened.Add(time.Minute)), CapitalProtectionActivatedRow(start.PositionPrice, -handLossPct(start, atTheDrawdown)))

	committed := committedBy(withRow(start, capitalProtectionTriggered(start), opened), CapitalProtectionActivatedRow(start.PositionPrice, -0.5))
	assertTrailHeld(t, ApplyAt(anchoredToHold(committed, atTheDrawdown), "", atTheDrawdown, ai, opened.Add(time.Minute)))
	assertSold(t, ApplyAt(anchoredToSell(committed, atTheDrawdown), "", atTheDrawdown, ai, opened.Add(time.Minute)), reasonCapitalProtection)
	if st := rebuildState(withRow(committed, ExitRow(committed, reasonCapitalProtection, atTheDrawdown), opened.Add(time.Hour))); st.capitalProtectionActive || !st.capitalProtectionTriggeredAt.IsZero() {
		t.Fatalf("the sold event closes the episode and its stamp, got %+v", st)
	}
}
