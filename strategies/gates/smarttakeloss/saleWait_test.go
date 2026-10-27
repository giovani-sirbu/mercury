package smarttakeloss

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The wait after a sale gate's last trigger (WaitAfterTrigger) through ApplyAt, the clock the
// engines hand the overlay. The flat market's and the free fall prevention's trigger is the
// triggered row of an episode (opening it, or re-arming it after a reset at a new depth), the slow
// decline's the pending row (going pending, or a new fill confirming it); from that tick the
// gate's trigger is not handed to the shared action until the wait has passed: no held row, no
// activation, no sale of a self-armed stopLoss, the proposal passing through, while the episode
// and the ladder run as they would. The first tick at the wait is judged as before; a committed
// sale trails through it. Every test sets the wait for itself (withWaitAfterTrigger): TestMain
// leaves it off.

// saleWait is the wait the tests set, and saleWaitReason what the triggered rows then name.
const (
	saleWait       = time.Hour
	saleWaitReason = "the sale waits 1h0m0s from the trigger"
)

// episodeGate is what the two episode gates, the flat market and the free fall prevention, differ
// by on the round ladder: the market their reading holds on and the one it misses on, their rows,
// the reason of their sale and the state they leave.
type episodeGate struct {
	name      string
	alone     func(*testing.T)
	market    func(aggragates.Trades) aggragates.AIIndicators
	gone      func(aggragates.Trades) aggragates.AIIndicators
	triggered func(aggragates.Trades, aggragates.AIIndicators) Row
	held      func(fill, profitPct float64) Row
	activated func(fill, profitPct float64) Row
	reset     func(fill float64) Row
	reason    string
	active    func(state) bool
	stamp     func(state) time.Time
}

var episodeGates = []episodeGate{
	{
		name:   "flat market",
		alone:  flatMarketAlone,
		market: flatMarketMarket,
		gone:   barsGone,
		triggered: func(trade aggragates.Trades, ai aggragates.AIIndicators) Row {
			return triggeredBy(trade, ai)
		},
		held:      FlatMarketSaleHeldRow,
		activated: FlatMarketStopLossActivatedRow,
		reset:     FlatMarketResetRow,
		reason:    reasonFlatMarket,
		active:    func(st state) bool { return st.flatMarketActive },
		stamp:     func(st state) time.Time { return st.flatMarketTriggeredAt },
	},
	{
		name:   "free fall prevention",
		alone:  freeFallPreventionAlone,
		market: freeFallMarket,
		gone: func(aggragates.Trades) aggragates.AIIndicators {
			return freeFallMisses(aggragates.SideLong)["no row in free fall"]
		},
		triggered: func(trade aggragates.Trades, _ aggragates.AIIndicators) Row {
			return freeFallTriggeredBy(trade)
		},
		held:      FreeFallPreventionSaleHeldRow,
		activated: FreeFallPreventionStopLossActivatedRow,
		reset:     FreeFallPreventionResetRow,
		reason:    reasonFreeFallPrevention,
		active:    func(st state) bool { return st.freeFallPreventionActive },
		stamp:     func(st state) time.Time { return st.freeFallPreventionTriggeredAt },
	},
}

// An episode gate's trigger tick writes the triggered row alone, at the cap and over it, on any
// proposal and from a self-armed stopLoss; from the stored event the wait is known to every later
// tick, within which the proposal passes through untouched — no held row over the cap, no
// activation within it, no sale — the ladder's own closes with it and the episode open (Triggered).
// The tick at the wait is judged as before: the held row over the cap (once), the activation in
// buy and the sale of a self-armed stopLoss within it.
func TestAnEpisodeGatesSaleWaitsFromItsTriggerAndIsJudgedWhenTheWaitEnds(t *testing.T) {
	for _, gate := range episodeGates {
		t.Run(gate.name, func(t *testing.T) {
			gate.alone(t)
			withWaitAfterTrigger(t, saleWait)
			start := flatMarketLadder(false)
			ai, fill := gate.market(start), start.PositionPrice
			atTheCap, oneStepLower := flatMarketPrices(t, start)
			opened := testutil.At("13:00:00")
			triggered := gate.triggered(start, ai)
			held := gate.held(fill, -handLossPct(start, oneStepLower))
			activation := gate.activated(fill, -handLossPct(start, atTheCap))
			if !slices.Contains(triggered.Reasons, saleWaitReason) || !strings.HasSuffix(triggered.Message, ", "+saleWaitReason+")") {
				t.Fatalf("the triggered row must name the wait last, got %+v", triggered)
			}

			for _, price := range []float64{atTheCap, oneStepLower} {
				for _, position := range addSideProposals {
					assertWaits(t, ApplyAt(start, position, price, ai, opened), position, triggered)
					assertWaits(t, ApplyAt(restingIn(start, "stopLoss"), position, price, ai, opened), position, triggered)
				}
			}

			trade := withRow(start, triggered, opened)
			if st := rebuildState(trade); !gate.active(st) || !gate.stamp(st).Equal(opened) || st.flatMarketSaleHeld || st.freeFallPreventionSaleHeld || !Triggered(trade) {
				t.Fatalf("the triggered event opens the episode and stamps it, the gate reads triggered, nothing is held, got %+v", st)
			}
			for _, now := range []time.Time{opened, opened.Add(time.Minute), opened.Add(saleWait - time.Second)} {
				for _, price := range []float64{atTheCap, oneStepLower, oneStepLower - 1} {
					for _, position := range addSideProposals {
						assertUntouched(t, ApplyAt(trade, position, price, ai, now), position)
					}
					for _, state := range []string{"stopLoss", "forceTrailingStopLoss"} {
						assertUntouched(t, ApplyAt(restingIn(trade, state), "", price, ai, now), "")
					}
				}
				for _, position := range slices.Concat(closingProposals, takeProfitProposals) {
					assertUntouched(t, ApplyAt(trade, position, atTheCap, ai, now), position)
				}
			}

			past := opened.Add(saleWait)
			for _, position := range addSideProposals {
				assertWaits(t, ApplyAt(trade, position, oneStepLower, ai, past), position, held)
				assertActivated(t, ApplyAt(trade, position, atTheCap, ai, past), activation)
				assertSold(t, ApplyAt(restingIn(trade, "stopLoss"), position, atTheCap, ai, past), gate.reason)
			}
			assertActivated(t, ApplyAt(trade, "", atTheCap, ai, opened.Add(24*time.Hour)), activation)
			written := withRow(trade, held, past)
			assertUntouched(t, ApplyAt(written, "", oneStepLower, ai, past.Add(time.Minute)), "")
			assertActivated(t, ApplyAt(written, "", atTheCap, ai, past.Add(time.Minute)), activation)
		})
	}
}

// A fill at a new depth long after the first wait resets the episode and the reading re-arms it on
// the same tick: the new triggered row restarts the wait, so the sale that the first trigger would
// have let through by then is held back again until a wait after the new row. A reading that no
// longer holds at the new depth only closes the episode, with no stamp left to wait on.
func TestAReArmedEpisodeRestartsTheSaleWait(t *testing.T) {
	for _, gate := range episodeGates {
		t.Run(gate.name, func(t *testing.T) {
			gate.alone(t)
			withWaitAfterTrigger(t, saleWait)
			start := flatMarketLadder(false)
			ai := gate.market(start)
			opened := testutil.At("13:00:00")
			trade := withRow(start, gate.triggered(start, ai), opened)
			atNewDepth := deeperBy(trade, 97)
			atTheCap, oneStepLower := 99.0, 96.0
			within, over := handLossPct(atNewDepth, atTheCap), handLossPct(atNewDepth, oneStepLower)
			if !(0 < within && within < over) {
				t.Fatalf("fixture drifted: the two prints lose, the lower more, got %v and %v", within, over)
			}
			withAllowedLoss(t, (within+over)/2)
			rearmed := opened.Add(2 * saleWait)
			reset, again := gate.reset(97), gate.triggered(atNewDepth, ai)

			got := ApplyAt(atNewDepth, "", atTheCap, ai, rearmed)
			assertWaits(t, got, "", reset, again)
			deeper := withRow(withRow(atNewDepth, got.Rows[0], rearmed), got.Rows[1], rearmed)
			if st := rebuildState(deeper); !gate.active(st) || !gate.stamp(st).Equal(rearmed) {
				t.Fatalf("the new triggered event stamps the episode afresh, got %+v", st)
			}
			for _, now := range []time.Time{rearmed, rearmed.Add(time.Minute), rearmed.Add(saleWait - time.Second)} {
				for _, price := range []float64{atTheCap, oneStepLower} {
					assertUntouched(t, ApplyAt(deeper, "", price, ai, now), "")
					assertUntouched(t, ApplyAt(restingIn(deeper, "stopLoss"), "", price, ai, now), "")
				}
			}
			past := rearmed.Add(saleWait)
			assertWaits(t, ApplyAt(deeper, "", oneStepLower, ai, past), "", gate.held(97, -handLossPct(deeper, oneStepLower)))
			assertActivated(t, ApplyAt(deeper, "", atTheCap, ai, past), gate.activated(97, -handLossPct(deeper, atTheCap)))
			assertSold(t, ApplyAt(restingIn(deeper, "stopLoss"), "", atTheCap, ai, past), gate.reason)

			gone := gate.gone(atNewDepth)
			assertWaits(t, ApplyAt(atNewDepth, "", atTheCap, gone, rearmed), "", reset)
			closed := withRow(atNewDepth, reset, rearmed)
			if st := rebuildState(closed); gate.active(st) || !gate.stamp(st).IsZero() {
				t.Fatalf("the reset closes the episode and its stamp, got %+v", st)
			}
			assertUntouched(t, ApplyAt(closed, "", atTheCap, gone, rearmed.Add(saleWait)), "")
		})
	}
}

// A committed sale (the stopLoss activated on the gate's trigger) trails through the wait whatever
// the reading: held between its bounds, re-anchored over the trailing take profit, sold under the
// tolerance, with no row, while a self-armed stopLoss on the same stamp is not sold.
func TestACommittedSaleTrailsThroughTheSaleWaitOfAnEpisodeGate(t *testing.T) {
	for _, gate := range episodeGates {
		t.Run(gate.name, func(t *testing.T) {
			gate.alone(t)
			withWaitAfterTrigger(t, saleWait)
			start := flatMarketLadder(false)
			ai, fill := gate.market(start), start.PositionPrice
			atTheCap, _ := flatMarketPrices(t, start)
			opened := testutil.At("13:00:00")
			committed := committedBy(withRow(start, gate.triggered(start, ai), opened), gate.activated(fill, -handLossPct(start, atTheCap)))
			if st := rebuildState(committed); !gate.active(st) || !gate.stamp(st).Equal(opened) {
				t.Fatalf("fixture drifted: the committed episode keeps the stamp that only its activation lifts the wait from, got %+v", st)
			}

			within := opened.Add(time.Minute)
			assertUntouched(t, ApplyAt(restingIn(withRow(start, gate.triggered(start, ai), opened), "stopLoss"), "", atTheCap, ai, within), "")
			for _, reading := range []aggragates.AIIndicators{ai, gate.gone(start)} {
				assertTrailHeld(t, ApplyAt(anchoredToHold(committed, atTheCap), "", atTheCap, reading, within))
				assertReanchored(t, ApplyAt(anchoredToReanchor(committed, atTheCap), "", atTheCap, reading, within))
				assertSold(t, ApplyAt(anchoredToSell(committed, atTheCap), "", atTheCap, reading, within), gate.reason)
			}
		})
	}
}

// Apply hands the overlay no clock, and a wait of zero or below is none: the trigger tick activates
// a ladder in buy beside the triggered row and sells a self-armed stopLoss, and a stamped episode
// is acted on at once; ExitReached, which asks Apply, reads the sale of the stamped episode.
func TestApplyWithoutAClockAndAZeroWaitWaitNothingOnAnEpisodeGate(t *testing.T) {
	for _, gate := range episodeGates {
		t.Run(gate.name, func(t *testing.T) {
			gate.alone(t)
			withWaitAfterTrigger(t, saleWait)
			start := flatMarketLadder(false)
			ai, fill := gate.market(start), start.PositionPrice
			atTheCap, _ := flatMarketPrices(t, start)
			opened := testutil.At("13:00:00")
			triggered := gate.triggered(start, ai)
			activation := gate.activated(fill, -handLossPct(start, atTheCap))
			trade := withRow(start, triggered, opened)

			assertActivated(t, Apply(start, "", atTheCap, ai), triggered, activation)
			assertSoldWith(t, Apply(restingIn(start, "stopLoss"), "", atTheCap, ai), gate.reason, triggered)
			assertActivated(t, Apply(trade, "", atTheCap, ai), activation)
			assertActivated(t, ApplyAt(trade, "", atTheCap, ai, time.Time{}), activation)
			assertSold(t, Apply(restingIn(trade, "stopLoss"), "", atTheCap, ai), gate.reason)
			if !ExitReached(restingIn(trade, "stopLoss"), atTheCap, ai.SmartTakeLoss) {
				t.Fatal("ExitReached hands Apply no clock, so it reads the sale of a stamped episode")
			}

			for _, wait := range []time.Duration{0, -saleWait} {
				withWaitAfterTrigger(t, wait)
				plain := gate.triggered(start, ai)
				assertActivated(t, ApplyAt(start, "", atTheCap, ai, opened), plain, activation)
				assertSoldWith(t, ApplyAt(restingIn(start, "stopLoss"), "", atTheCap, ai, opened), gate.reason, plain)
				assertActivated(t, ApplyAt(trade, "", atTheCap, ai, opened.Add(time.Minute)), activation)
			}
		})
	}
}

// The stamp of an episode is the stored triggered event's own, read back by the fold: later held
// and activated events keep it, a reset or a sold event clears it, and an episode that a held or
// activated event opened, with no triggered event, has none and waits nothing.
func TestAnEpisodeGatesTriggerStampIsFoldedFromItsStoredEvents(t *testing.T) {
	for _, gate := range episodeGates {
		t.Run(gate.name, func(t *testing.T) {
			gate.alone(t)
			withWaitAfterTrigger(t, saleWait)
			start := flatMarketLadder(false)
			ai, fill := gate.market(start), start.PositionPrice
			atTheCap, oneStepLower := flatMarketPrices(t, start)
			first, second, third := testutil.At("13:00:00"), testutil.At("13:10:00"), testutil.At("13:20:00")

			triggered := withRow(start, gate.triggered(start, ai), first)
			if st := rebuildState(triggered); !gate.active(st) || !gate.stamp(st).Equal(first) {
				t.Fatalf("the triggered event stamps the episode with its own stamp, got %+v", st)
			}
			held := withRow(triggered, gate.held(fill, -handLossPct(start, oneStepLower)), second)
			if st := rebuildState(held); !gate.active(st) || !gate.stamp(st).Equal(first) {
				t.Fatalf("a held event keeps the stamp, got %+v", st)
			}
			activated := withRow(held, gate.activated(fill, -handLossPct(start, atTheCap)), third)
			if st := rebuildState(activated); !gate.active(st) || !gate.stamp(st).Equal(first) {
				t.Fatalf("an activated event keeps the stamp, got %+v", st)
			}
			sold := withRow(restingIn(activated, "stopLoss"), ExitRow(activated, gate.reason, atTheCap), third.Add(time.Minute))
			if st := rebuildState(sold); gate.active(st) || !gate.stamp(st).IsZero() {
				t.Fatalf("a sold event closes the episode and its stamp, got %+v", st)
			}
			reset := withRow(triggered, gate.reset(fill), second)
			if st := rebuildState(reset); gate.active(st) || !gate.stamp(st).IsZero() {
				t.Fatalf("a reset event closes the episode and its stamp, got %+v", st)
			}

			for name, opener := range map[string]Row{"held": gate.held(fill, -handLossPct(start, oneStepLower)), "activated": gate.activated(fill, -handLossPct(start, atTheCap))} {
				joined := withRow(start, opener, first)
				if st := rebuildState(joined); !gate.active(st) || !gate.stamp(st).IsZero() {
					t.Fatalf("a %s event opens the episode with no stamp, got %+v", name, st)
				}
				assertActivated(t, ApplyAt(restingIn(joined, "buy"), "", atTheCap, ai, first.Add(time.Minute)), gate.activated(fill, -handLossPct(start, atTheCap)))
			}
		})
	}
}

// The flat market's legacy gates (low interest, non interest area) wrote their triggered events
// under their own names; the fold stamps the episode with them as with the gate's own.
func TestTheFlatMarketsLegacyTriggeredEventsStampTheEpisodeToo(t *testing.T) {
	flatMarketAlone(t)
	withWaitAfterTrigger(t, saleWait)
	start := flatMarketLadder(false)
	ai := flatMarketMarket(start)
	atTheCap, _ := flatMarketPrices(t, start)
	opened := testutil.At("13:00:00")
	for _, gate := range []string{gateLowInterest, gateNonInterestArea} {
		event := eventOf(gate, legacyDocument(EventTriggered, start.PositionPrice))
		event.CreatedAt = opened
		trade := withEvents(start, event)
		if st := rebuildState(trade); !st.flatMarketActive || !st.flatMarketTriggeredAt.Equal(opened) {
			t.Fatalf("%s: the legacy triggered event opens the episode and stamps it, got %+v", gate, st)
		}
		assertUntouched(t, ApplyAt(trade, "", atTheCap, ai, opened.Add(time.Minute)), "")
		assertActivated(t, ApplyAt(trade, "", atTheCap, ai, opened.Add(saleWait)), FlatMarketStopLossActivatedRow(start.PositionPrice, -handLossPct(start, atTheCap)))
	}
}

// The flat market's and the free fall prevention's triggered rows end with the wait the sale keeps
// from them, listed among their reasons and worded in the message, while WaitAfterTrigger is above
// zero; at zero or below they name none, and the reasons the reading gave are never touched.
func TestTheTriggeredRowNamesTheSaleWaitOnlyWhileItIsAboveZero(t *testing.T) {
	builders := []struct {
		name  string
		build func(fill float64, reasons []string) Row
		head  string
	}{
		{"flat market", FlatMarketTriggeredRow, "smartTakeLoss: flat market triggered"},
		{"free fall prevention", FreeFallPreventionTriggeredRow, "smartTakeLoss: free fall prevention triggered"},
	}
	for _, builder := range builders {
		t.Run(builder.name, func(t *testing.T) {
			for wait, named := range map[time.Duration]string{saleWait: "1h0m0s", 90 * time.Minute: "1h30m0s", 30 * time.Second: "30s"} {
				withWaitAfterTrigger(t, wait)
				reasons := make([]string, 1, 4)
				reasons[0] = "a reading"
				row := builder.build(100, reasons)
				text := "the sale waits " + named + " from the trigger"
				if want := []string{"a reading", text}; !slices.Equal(row.Reasons, want) {
					t.Errorf("wait %v: reasons = %q, want %q", wait, row.Reasons, want)
				}
				if want := builder.head + " (a reading, " + text + ")"; row.Message != want {
					t.Errorf("wait %v: message = %q, want %q", wait, row.Message, want)
				}
				if reasons[:cap(reasons)][1] != "" || len(reasons) != 1 {
					t.Errorf("wait %v: the reading's reasons were written into, got %q", wait, reasons[:cap(reasons)])
				}
				if alone := builder.build(100, nil); !slices.Equal(alone.Reasons, []string{text}) || alone.Message != builder.head+" ("+text+")" {
					t.Errorf("wait %v: a reading with no reasons still names the wait, got %+v", wait, alone)
				}
			}
			for _, wait := range []time.Duration{0, -saleWait} {
				withWaitAfterTrigger(t, wait)
				row := builder.build(100, []string{"a reading"})
				if want := []string{"a reading"}; !slices.Equal(row.Reasons, want) || row.Message != builder.head+" (a reading)" {
					t.Errorf("wait %v: the row names no wait, got %+v", wait, row)
				}
				if bare := builder.build(100, nil); len(bare.Reasons) != 0 || bare.Message != builder.head {
					t.Errorf("wait %v: the bare row names no wait, got %+v", wait, bare)
				}
			}
		})
	}
}

// slowDeclineWaitAlone runs the test with the quiet decline the only gate on, the early take profit
// off so that no latch joins its rows, and the wait set.
func slowDeclineWaitAlone(t *testing.T) {
	t.Helper()
	withQuietDecline(t, true)
	withSwingDecline(t, false)
	withLinearDecline(t, false)
	withEarlyTakeProfit(t, false)
	withFlatMarket(t, false)
	withFreeFallPrevention(t, false)
	withCapitalProtectionExit(t, false)
	withWaitAfterTrigger(t, saleWait)
}

// The slow decline's trigger is its pending row: the tick that goes pending writes it alone, and
// until the wait has passed from it nothing is held, activated or sold at the cap or over it, in
// buy or from a self-armed stopLoss. The tick at the wait is judged as before: the held row over
// the cap (once), the activation within it, the sale of a self-armed stopLoss.
func TestTheSlowDeclineSaleWaitsFromItsPendingRowAndIsJudgedWhenTheWaitEnds(t *testing.T) {
	slowDeclineWaitAlone(t)
	trade, ai := watchedTrade(), slowDeclineBlock(true)
	within, over := handLossPct(trade, slowDeclineBand), handLossPct(trade, underTheBand)
	if !(0 < within && within < over) {
		t.Fatalf("fixture drifted: the two prints of the dead zone lose, the lower more, got %v and %v", within, over)
	}
	withAllowedLoss(t, (within+over)/2)
	opened := testutil.At("18:00:00")
	pending := quietPending(slowDeclineLastFill)
	held := SaleHeldRow(slowDeclineLastFill, -over, FeatureQuietDecline)
	activation := quietActivation(trade, slowDeclineBand)

	for _, price := range []float64{slowDeclineBand, underTheBand} {
		for _, position := range append(slices.Clone(ordinaryProposals), armingProposals...) {
			assertWaits(t, ApplyAt(trade, position, price, ai, opened), position, pending)
			assertWaits(t, ApplyAt(restingIn(trade, "stopLoss"), position, price, ai, opened), position, pending)
		}
	}

	pendingTrade := withRow(trade, pending, opened)
	if st := rebuildState(pendingTrade); !st.slowDeclinePending || !st.slowDeclineTriggeredAt.Equal(opened) || !Triggered(pendingTrade) {
		t.Fatalf("the pending event makes the ladder pending and stamps it, and the gate reads triggered, got %+v", st)
	}
	for _, now := range []time.Time{opened, opened.Add(time.Minute), opened.Add(saleWait - time.Second)} {
		for _, price := range []float64{slowDeclineBand, underTheBand, underTheBand - 1} {
			for _, position := range append(slices.Clone(ordinaryProposals), armingProposals...) {
				assertUntouched(t, ApplyAt(pendingTrade, position, price, ai, now), position)
			}
			for _, state := range []string{"stopLoss", "forceTrailingStopLoss"} {
				assertUntouched(t, ApplyAt(restingIn(pendingTrade, state), "", price, ai, now), "")
			}
		}
		for _, position := range slices.Concat(closingProposals, takeProfitProposals) {
			assertUntouched(t, ApplyAt(pendingTrade, position, slowDeclineBand, ai, now), position)
		}
	}

	past := opened.Add(saleWait)
	for _, position := range append(slices.Clone(ordinaryProposals), armingProposals...) {
		assertWaits(t, ApplyAt(pendingTrade, position, underTheBand, ai, past), position, held)
		assertActivated(t, ApplyAt(pendingTrade, position, slowDeclineBand, ai, past), activation)
		assertSold(t, ApplyAt(restingIn(pendingTrade, "stopLoss"), position, slowDeclineBand, ai, past), reasonSlowDecline)
	}
	assertActivated(t, ApplyAt(pendingTrade, "", slowDeclineBand, ai, opened.Add(24*time.Hour)), activation)
	written := withRow(pendingTrade, held, past)
	assertUntouched(t, ApplyAt(written, "", underTheBand, ai, past.Add(time.Minute)), "")
	assertActivated(t, ApplyAt(written, "", slowDeclineBand, ai, past.Add(time.Minute)), activation)
}

// A new fill that confirms the pending ladder writes a pending event again and restarts the wait,
// though the first pending row is long past it: the confirming tick writes the confirmed row alone,
// and the sale is held back until a wait after it. A later pending event is the stamp the fold
// reads; the held and activated events keep it.
func TestTheSlowDeclinesConfirmationAtANewFillRestartsTheSaleWait(t *testing.T) {
	slowDeclineWaitAlone(t)
	trade := pendingAtFive()
	if st := rebuildState(trade); !st.slowDeclinePending || !st.slowDeclineTriggeredAt.Equal(testutil.At("15:00:00")) {
		t.Fatalf("fixture drifted: pending from the fifth fill since 15:00, got %+v", st)
	}
	confirmed := sixthConfirmed()

	for _, position := range append(slices.Clone(ordinaryProposals), armingProposals...) {
		assertWaits(t, ApplyAt(trade, position, underJudgeBand, legOnAndQuiet(), judgeTick), position, confirmed)
		assertWaits(t, ApplyAt(restingIn(trade, "stopLoss"), position, underJudgeBand, legOnAndQuiet(), judgeTick), position, confirmed)
	}

	restarted := withRow(trade, confirmed, judgeTick)
	if st := rebuildState(restarted); !st.slowDeclinePending || st.slowDeclinePendingFrom != sixthFill || !st.slowDeclineTriggeredAt.Equal(judgeTick) {
		t.Fatalf("the confirmed event stamps the ladder afresh, got %+v", st)
	}
	for _, now := range []time.Time{judgeTick, judgeTick.Add(time.Minute), judgeTick.Add(saleWait - time.Second)} {
		for _, position := range append(slices.Clone(ordinaryProposals), armingProposals...) {
			assertUntouched(t, ApplyAt(restarted, position, underJudgeBand, legOnAndQuiet(), now), position)
			assertUntouched(t, ApplyAt(restingIn(restarted, "stopLoss"), position, underJudgeBand, legOnAndQuiet(), now), position)
		}
	}
	past := judgeTick.Add(saleWait)
	assertActivated(t, ApplyAt(restarted, "", underJudgeBand, legOnAndQuiet(), past), quietActivation(restarted, underJudgeBand))
	assertSold(t, ApplyAt(restingIn(restarted, "stopLoss"), "", underJudgeBand, legOnAndQuiet(), past), reasonSlowDecline)

	held := withRow(restarted, SaleHeldRow(sixthFill, -1, FeatureQuietDecline), judgeTick.Add(time.Minute))
	activated := withRow(held, StopLossActivatedRow(sixthFill, -1, FeatureQuietDecline), judgeTick.Add(2*time.Minute))
	for name, stamped := range map[string]aggragates.Trades{"held": held, "activated": activated} {
		if st := rebuildState(stamped); !st.slowDeclinePending || !st.slowDeclineTriggeredAt.Equal(judgeTick) {
			t.Errorf("a %s event keeps the stamp of the pending event, got %+v", name, st)
		}
	}
}

// A committed slow decline sale (the stopLoss activated on its trigger) trails through the wait
// whatever the reading, with no row, while a self-armed stopLoss under the same stamp is not sold.
func TestACommittedSaleTrailsThroughTheSlowDeclinesSaleWait(t *testing.T) {
	slowDeclineWaitAlone(t)
	trade, ai := watchedTrade(), slowDeclineBlock(true)
	opened := testutil.At("18:00:00")
	pendingTrade := withRow(trade, quietPending(slowDeclineLastFill), opened)
	committed := committedBy(pendingTrade, quietActivation(trade, slowDeclineBand))
	if st := rebuildState(committed); !st.slowDeclinePending || !st.slowDeclineActivated || !st.slowDeclineTriggeredAt.Equal(opened) {
		t.Fatalf("fixture drifted: pending, stamped and activated, got %+v", st)
	}

	within := opened.Add(time.Minute)
	assertUntouched(t, ApplyAt(restingIn(pendingTrade, "stopLoss"), "", slowDeclineBand, ai, within), "")
	for _, reading := range []aggragates.AIIndicators{ai, nothingServed()} {
		assertTrailHeld(t, ApplyAt(anchoredToHold(committed, slowDeclineBand), "", slowDeclineBand, reading, within))
		assertReanchored(t, ApplyAt(anchoredToReanchor(committed, slowDeclineBand), "", slowDeclineBand, reading, within))
		assertSold(t, ApplyAt(anchoredToSell(committed, slowDeclineBand), "", slowDeclineBand, reading, within), reasonSlowDecline)
	}
}

// Apply hands the overlay no clock, and a wait of zero or below is none: the tick that goes pending
// activates a ladder in buy beside the pending row and sells a self-armed stopLoss, and a stamped
// pending ladder is acted on at once.
func TestApplyWithoutAClockAndAZeroWaitWaitNothingOnTheSlowDecline(t *testing.T) {
	slowDeclineWaitAlone(t)
	trade, ai := watchedTrade(), slowDeclineBlock(true)
	opened := testutil.At("18:00:00")
	pending, activation := quietPending(slowDeclineLastFill), quietActivation(trade, slowDeclineBand)
	pendingTrade := withRow(trade, pending, opened)

	assertActivated(t, Apply(trade, "", slowDeclineBand, ai), pending, activation)
	assertSoldWith(t, Apply(restingIn(trade, "stopLoss"), "", slowDeclineBand, ai), reasonSlowDecline, pending)
	assertActivated(t, Apply(pendingTrade, "", slowDeclineBand, ai), activation)
	assertActivated(t, ApplyAt(pendingTrade, "", slowDeclineBand, ai, time.Time{}), activation)
	assertSold(t, Apply(restingIn(pendingTrade, "stopLoss"), "", slowDeclineBand, ai), reasonSlowDecline)

	for _, wait := range []time.Duration{0, -saleWait} {
		withWaitAfterTrigger(t, wait)
		assertActivated(t, ApplyAt(trade, "", slowDeclineBand, ai, opened), pending, activation)
		assertSoldWith(t, ApplyAt(restingIn(trade, "stopLoss"), "", slowDeclineBand, ai, opened), reasonSlowDecline, pending)
		assertActivated(t, ApplyAt(pendingTrade, "", slowDeclineBand, ai, opened.Add(time.Minute)), activation)
	}
}

// The wait is each gate's own: a waiting flat market episode neither holds back capital protection's
// trigger nor lends it its wait, which stays capitalprotection.WaitAfterTrigger — none when that is
// zero though the sale wait is set, its own alone when both are, longer or shorter than the sale's.
func TestCapitalProtectionKeepsItsOwnWaitBesideTheSaleWait(t *testing.T) {
	capitalProtectionAlone(t)
	start := capitalProtectionLadder(false)
	ai, fill := withBlock(solBlock()), start.PositionPrice
	atTheDrawdown, _ := drawdownPrices(t, start)
	opened := testutil.At("13:00:00")
	activation := CapitalProtectionStopLossActivatedRow(fill, -handLossPct(start, atTheDrawdown))

	withWaitAfterTrigger(t, saleWait)
	withCapitalProtectionWaitAfterTrigger(t, 0)
	triggered := capitalProtectionTriggered(start)
	if slices.ContainsFunc(triggered.Reasons, func(reason string) bool { return strings.Contains(reason, "waits") }) {
		t.Fatalf("with no wait of its own the triggered row names none, got %q", triggered.Reasons)
	}
	assertActivated(t, ApplyAt(start, "", atTheDrawdown, ai, opened), triggered, activation)
	assertSoldWith(t, ApplyAt(restingIn(start, "stopLoss"), "", atTheDrawdown, ai, opened), reasonCapitalProtection, triggered)
	assertActivated(t, ApplyAt(withRow(start, triggered, opened), "", atTheDrawdown, ai, opened.Add(time.Minute)), activation)

	for _, sale := range []time.Duration{saleWait, 8 * time.Hour, 0} {
		withWaitAfterTrigger(t, sale)
		withCapitalProtectionWaitAfterTrigger(t, 4*time.Hour)
		triggered = capitalProtectionTriggered(start)
		if !slices.Contains(triggered.Reasons, "the sale waits 4h0m0s from the trigger") {
			t.Fatalf("sale wait %v: the triggered row names capital protection's own wait, got %q", sale, triggered.Reasons)
		}
		assertWaits(t, ApplyAt(start, "", atTheDrawdown, ai, opened), "", triggered)
		trade := withRow(start, triggered, opened)
		assertUntouched(t, ApplyAt(trade, "", atTheDrawdown, ai, opened.Add(4*time.Hour-time.Second)), "")
		assertActivated(t, ApplyAt(trade, "", atTheDrawdown, ai, opened.Add(4*time.Hour)), activation)
	}
}

// Capital protection acts on the tick its own trigger stands, with no wait of its own, while the
// flat market's episode opened on the same tick waits out the sale wait: the flat market's
// triggered row stands beside capital protection's and its activation, which names capital
// protection alone.
func TestAWaitingFlatMarketEpisodeDoesNotHoldBackCapitalProtection(t *testing.T) {
	flatMarketAlone(t)
	withCapitalProtectionExit(t, true)
	withWaitAfterTrigger(t, saleWait)
	withCapitalProtectionWaitAfterTrigger(t, 0)
	start := capitalProtectionLadder(false)
	block := flatMarketMarket(start).SmartTakeLoss
	block.CapitalProtectionSmcBearish = true
	ai, fill := withBlock(block), start.PositionPrice
	atTheDrawdown, _ := drawdownPrices(t, start)
	opened := testutil.At("13:00:00")

	assertActivated(t, ApplyAt(start, "", atTheDrawdown, ai, opened),
		triggeredBy(start, ai), capitalProtectionTriggered(start),
		CapitalProtectionStopLossActivatedRow(fill, -handLossPct(start, atTheDrawdown)))
	if st := rebuildState(withRow(start, triggeredBy(start, ai), opened)); !st.flatMarketActive || !st.flatMarketTriggeredAt.Equal(opened) {
		t.Fatalf("the flat market episode stands stamped beside capital protection's, got %+v", st)
	}
}
