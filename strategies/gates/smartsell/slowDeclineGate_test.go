package smartsell

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The shared pending of the slow decline gate, tabled apart from the code: what a
// pending ladder's new fill meets from each of the two declines, one reading from
// each, and what the gate makes of the pair. The staircase is the swing decline's
// series (stairBlock); the quiet decline reads the leg the blocks below serve
// beside it, and is taken on the fill window they serve to mark their window as
// read. Every expectation is built from the row builders and from
// swingdecline.Trigger through stairReading, so the tables hold under whatever
// constants the rule is tuned to.

// judgedQuiet are the five states of the quiet decline's reading of a new fill:
// the switch off, no window read — so the reading cannot be taken — and, with the
// window read, a reading that holds on the leg on and quiet, one that holds on
// the decline read recently, and one that misses. serve is what sophos adds to
// the block; reasons are what a row names for the state, the break reasons for
// the state that misses.
var judgedQuiet = []struct {
	name    string
	on      bool
	serve   func(aggragates.SmartSellIndicators) aggragates.SmartSellIndicators
	reasons []string
}{
	{"switched off", false, withBreaks, quietDeclineBreakReasons},
	{"no window read", true, withoutTheWindow, nil},
	{"holds on the leg on and quiet", true, withLegOnAndQuiet, slowDeclineReasons},
	{"holds on the decline read recently", true, withTheDeclineReadRecently, quietDeclineRecentReasons},
	{"misses", true, withBreaks, quietDeclineBreakReasons},
}

// withTheDeclineReadRecently is the block with a bar the verdict stood on among
// the last closed bars sophos looks back over, the oldest of those bars opening
// before the staircase ladder's fifth fill: what holds a new fill on the decline
// read recently, the leg not on and quiet, and the fill window that marks the
// window as read.
func withTheDeclineReadRecently(block aggragates.SmartSellIndicators) aggragates.SmartSellIndicators {
	block.QuietDeclineRecentAt = stairOpen(29).UnixMilli()
	block.QuietDeclineRecentFrom = stairOpen(28).UnixMilli()
	block.QuietDeclineRecentReasons = quietDeclineRecentReasons
	block.QuietDeclineFillFrom = fillWindowFrom.UnixMilli()
	return block
}

// judgedSwing are the five states of the swing decline's reading of the fifth
// fill, which sits in bar 30: the switch off, no series served, the series closed
// to the bar before the fill's — the reading cannot be taken yet — and the series
// closed to the fill's bar reading a decline that holds, and one that misses.
var judgedSwing = []struct {
	name  string
	on    bool
	serve func() aggragates.SmartSellIndicators
}{
	{"switched off", false, stairFifthBlock},
	{"no series served", true, func() aggragates.SmartSellIndicators { return withoutTheSeries(stairFifthBlock()) }},
	{"bar not closed", true, func() aggragates.SmartSellIndicators { return stairBlock(stairTurns, 29) }},
	{"holds", true, stairFifthBlock},
	{"misses", true, func() aggragates.SmartSellIndicators { return stairBlock(stairBounce, 30) }},
}

// judgement is what the gate makes of a new fill: it is confirmed on the features
// that held, cancelled naming the features that missed, or it waits — no row, and
// no sale, the fill staying unjudged.
type judgement struct {
	outcome  string
	features []string
}

func judgedConfirm(features ...string) judgement { return judgement{"confirm", features} }
func judgedCancel(features ...string) judgement  { return judgement{"cancel", features} }

var judgedWait = judgement{outcome: "wait"}

// judgements is the table, quiet decline down and swing decline across, in the
// order of judgedQuiet and judgedSwing. A reading applies when its feature is on
// and, for the swing decline, the series is served; it is taken when the quiet
// decline is served its fill window and when the series has closed the fill's
// bar. The fill is confirmed as soon as a taken reading holds, cancelled once
// every applicable reading is taken and none holds, and waits otherwise — also
// when nothing applies at all.
var judgements = [5][5]judgement{
	/* quiet off          */ {judgedWait, judgedWait, judgedWait, judgedConfirm(FeatureSwingDecline), judgedCancel(FeatureSwingDecline)},
	/* no window          */ {judgedWait, judgedWait, judgedWait, judgedConfirm(FeatureSwingDecline), judgedWait},
	/* holds, leg quiet   */ {judgedConfirm(FeatureQuietDecline), judgedConfirm(FeatureQuietDecline), judgedConfirm(FeatureQuietDecline), judgedConfirm(FeatureQuietDecline, FeatureSwingDecline), judgedConfirm(FeatureQuietDecline)},
	/* holds, read recent */ {judgedConfirm(FeatureQuietDecline), judgedConfirm(FeatureQuietDecline), judgedConfirm(FeatureQuietDecline), judgedConfirm(FeatureQuietDecline, FeatureSwingDecline), judgedConfirm(FeatureQuietDecline)},
	/* misses             */ {judgedCancel(FeatureQuietDecline), judgedCancel(FeatureQuietDecline), judgedWait, judgedConfirm(FeatureSwingDecline), judgedCancel(FeatureQuietDecline, FeatureSwingDecline)},
}

// Every pairing of the two readings, the switches included, is judged as the
// table says, on a ladder pending on both declines at its fourth fill whose fifth is new
// and unjudged, with the early take profit off so no latch joins the row: the
// row of a confirmation names the features that held with the reasons they
// served, that of a cancellation every feature that missed, a waiting fill writes
// nothing and the ladder reads it as unjudged, and the shared action acts on the
// tick only when the fill ends confirmed, whatever reading confirmed it — the trailingBuy
// of the ladder in buy activated at a print under its break even, naming the features
// that held, and the ladder in trailingBuy sold — never over a cancelled fill or one still
// waiting, and nothing is activated at a print over the break even, where the row
// stands alone. What the tick wrote is what the fold of the events reads back:
// pending from the new fill on the features that held, or not pending.
func TestANewFillIsJudgedAsTheTableSaysForEveryPairingOfTheReadings(t *testing.T) {
	withEarlyTakeProfit(t, false)
	trade := pendingAtFourthWithTheFifth(quietFinding(nil), swingFinding(nil))
	fill := stairPrices[4]
	swingHeld := stairReading(t, trade, stairFifthBlock(), true).Reasons
	swingBroke := stairReading(t, trade, stairBlock(stairBounce, 30), false).Breaks

	for i, quiet := range judgedQuiet {
		for j, swing := range judgedSwing {
			want := judgements[i][j]
			t.Run(quiet.name+" / "+swing.name, func(t *testing.T) {
				withQuietDecline(t, quiet.on)
				withSwingDecline(t, swing.on)
				block := quiet.serve(swing.serve())

				var findings []Finding
				for _, feature := range want.features {
					switch {
					case feature == FeatureQuietDecline:
						findings = append(findings, quietFinding(quiet.reasons))
					case want.outcome == "confirm":
						findings = append(findings, swingFinding(swingHeld))
					default:
						findings = append(findings, swingFinding(swingBroke))
					}
				}

				armed, acts := armingPrice(trade), want.outcome == "confirm"
				ticked, got := engineTick(trade, "trailingBuy", armed, stairOpen(31), withBlock(block))
				st := rebuildState(ticked)
				var judged []Row
				switch want.outcome {
				case "confirm":
					judged = []Row{ConfirmedRow(fill, findings...)}
					if !st.slowDeclinePending || st.slowDeclinePendingFrom != fill || !slices.Equal(st.pendingFeatures, want.features) {
						t.Errorf("confirmed: want the ladder pending from the new fill on %v, got %+v", want.features, st)
					}
				case "cancel":
					judged = []Row{CancelledRow(fill, findings...)}
					if st.slowDeclinePending || len(st.pendingFeatures) != 0 {
						t.Errorf("cancelled: want the ladder not pending, got %+v", st)
					}
				default:
					if len(got.Rows) != 0 || len(ticked.Logs) != len(trade.Logs) || len(ticked.StrategyEvents) != len(trade.StrategyEvents) {
						t.Fatalf("a waiting fill writes nothing, got %+v", got.Rows)
					}
					if watched := quiet.on || swing.on; watched && !slowDeclineFillUnjudged(ticked, st) {
						t.Errorf("a waiting fill stays unjudged, got %+v", st)
					}
				}

				if acts {
					assertActivated(t, got, append(judged, ActivatedRow(fill, -handLossPct(trade, armed), want.features...))...)
					assertSoldWith(t, Apply(restingIn(trade, "trailingBuy"), "trailingBuy", armed, withBlock(block)), reasonSlowDecline, judged...)
				} else {
					assertWaits(t, got, "trailingBuy", judged...)
					assertWaits(t, Apply(restingIn(trade, "trailingBuy"), "trailingBuy", armed, withBlock(block)), "trailingBuy", judged...)
				}
				if gain := applyAtAGain(trade, "", withBlock(block)); gain.Position != "" || gain.Reason != "" || gain.Activated || !sameRows(gain.Rows, judged) {
					t.Errorf("over the break even the ladder in buy is activated nothing and the row stands alone, got %+v", gain)
				}
			})
		}
	}
}

// A fill that waits changes nothing and is judged on the first tick its bar has
// closed, whatever the ticks in between served: the shared action never acts
// over it, for an unjudged fill leaves the gate's trigger unmet, in buy or in
// trailingBuy, at any print; nothing is written however many ticks go by, and then
// the series reading it settles it — confirmed by the swing decline alone while
// the quiet decline still misses, and acted on at once, from that very tick,
// cancelled when both miss — or the quiet decline settles it earlier, holding with
// the bar still open. The row of the judgement is read at a print over the ladder's
// break even (gainPrint) with a buy order resting, where it stands alone.
func TestAWaitingFillIsJudgedWhenItsBarCloses(t *testing.T) {
	withQuietDecline(t, true)
	withSwingDecline(t, true)
	withEarlyTakeProfit(t, false)
	both := []Finding{quietFinding(nil), swingFinding(nil)}
	fill := stairPrices[4]
	pending := pendingAtFourthWithTheFifth(both...)

	waited := func(t *testing.T) aggragates.Trades {
		t.Helper()
		trade := pending
		waiting := withBlock(withBreaks(stairBlock(stairTurns, 29)))
		for minute := 1; minute <= 3; minute++ {
			for _, price := range []float64{slowDeclineBand - 1, slowDeclineBand, slowDeclineBand + 1} {
				assertUntouched(t, Apply(trade, "", price, waiting), "")
			}
			for _, position := range armingProposals {
				assertUntouched(t, Apply(trade, position, armingPrice(trade), waiting), position)
			}
			next, got := engineTick(trade, "trailingBuy", armingPrice(trade), fifthFillAt.Add(time.Duration(minute)*time.Minute), waiting)
			assertUntouched(t, got, "trailingBuy")
			if !reflect.DeepEqual(next, trade) {
				t.Fatalf("a waiting tick wrote into the trade at minute %d", minute)
			}
			trade = next
		}
		if st := rebuildState(trade); !slowDeclineFillUnjudged(trade, st) {
			t.Fatal("the fill must still wait after the ticks")
		}
		assertSoldNever(t, trade, waiting)
		return trade
	}

	t.Run("the bar closes on a decline that holds", func(t *testing.T) {
		closed := withBreaks(stairFifthBlock())
		reasons := stairReading(t, pending, closed, true).Reasons
		waiting := waited(t)
		confirmed := ConfirmedRow(fill, swingFinding(reasons))
		trade, got := tickAtAGain(waiting, "", stairOpen(31), withBlock(closed))
		assertRow(t, got, confirmed)
		assertNoSale(t, got, "")
		if st := rebuildState(trade); !st.slowDeclinePending || st.slowDeclinePendingFrom != fill || !slices.Equal(st.pendingFeatures, []string{FeatureSwingDecline}) {
			t.Errorf("pending from the new fill on the swing decline alone, got %+v", st)
		}
		if len(trade.Logs) != len(pending.Logs)+1 || len(trade.StrategyEvents) != len(pending.StrategyEvents)+1 {
			t.Errorf("one row and one event for the whole wait and its end, got %d and %d over %d and %d", len(trade.Logs), len(trade.StrategyEvents), len(pending.Logs), len(pending.StrategyEvents))
		}
		assertActedOn(t, waiting, withBlock(closed), confirmed)
	})

	t.Run("the bar closes on a rally", func(t *testing.T) {
		rally := withBreaks(stairBlock(stairBounce, 30))
		broke := stairReading(t, pending, rally, false).Breaks
		waiting := waited(t)
		cancelled := CancelledRow(fill, quietFinding(quietDeclineBreakReasons), swingFinding(broke))
		trade, got := engineTick(waiting, "trailingBuy", armingPrice(waiting), stairOpen(31), withBlock(rally))
		assertRow(t, got, cancelled)
		assertNoSale(t, got, "trailingBuy")
		if st := rebuildState(trade); st.slowDeclinePending {
			t.Errorf("cancelled on both readings, got %+v", st)
		}
		assertSoldNever(t, waiting, withBlock(rally), cancelled)
	})

	t.Run("the quiet decline holds before the bar closes", func(t *testing.T) {
		open := withLegOnAndQuiet(stairBlock(stairTurns, 29))
		waiting := waited(t)
		confirmed := ConfirmedRow(fill, quietFinding(slowDeclineReasons))
		_, got := tickAtAGain(waiting, "", stairOpen(31), withBlock(open))
		assertRow(t, got, confirmed)
		assertNoSale(t, got, "")
		assertActedOn(t, waiting, withBlock(open), confirmed)
	})
}

// gateDocument is the document of an event of the slow decline gate as the
// writers of either release stored it: the kind and the price, and the features
// when there are any.
func gateDocument(kind string, price float64, features ...string) string {
	if len(features) == 0 {
		return legacyDocument(kind, price)
	}
	listed, _ := json.Marshal(features)
	return fmt.Sprintf(`{"event":%q,"price":%v,"features":%s}`, kind, price, listed)
}

// The take profit's scan asks the events' filing alone before it folds anything,
// and what it skips must be what the fold never reads: for every event the gates
// have written — each of the two releases' gates, every kind, every set of
// features — alone and in every pair of them, under every combination of the four
// feature switches, the trade the scan passes over is neither pending nor latched,
// and the take profit reads what the fold of the events says — the move from the
// newest fill while pending, the move from the position price once latched —
// whichever way the scan answers. A state the scan hid from the fold would leave
// a pending ladder's take profit on the average entry price. The linear decline's
// switch is one of the four though no event names it: its latch is the
// indecision's, and the scan and the fold read the same whichever way it is set.
func TestTheTakeProfitScanNeverHidesAStateTheFoldReads(t *testing.T) {
	trade := sixDeep()
	trade.PositionPrice -= 6
	newest := rebuildState(trade).lastFill().Price
	if trade.PositionPrice >= newest {
		t.Fatal("fixture drifted: the position price must sit under the newest fill, so the two moves differ")
	}
	const price, profitPercentage = 250.0, 0.5

	var documents []aggragates.TradesStrategyEvents
	for _, gate := range []string{GateSlowDecline, gateSlowPattern, gateIndecision} {
		for _, kind := range []string{EventPending, EventCancelled, EventReset, EventLatched, EventSold, EventSaleHeld} {
			for _, features := range [][]string{nil, {FeatureQuietDecline}, {FeatureSwingDecline}, {FeatureIndecision}, {FeatureIndecision, FeatureQuietDecline, FeatureSwingDecline}} {
				documents = append(documents, eventOf(gate, gateDocument(kind, newest, features...)))
			}
		}
	}
	var sequences [][]aggragates.TradesStrategyEvents
	for _, first := range documents {
		sequences = append(sequences, []aggragates.TradesStrategyEvents{first})
		for _, second := range documents {
			sequences = append(sequences, []aggragates.TradesStrategyEvents{first, second})
		}
	}
	for _, gate := range []string{GateCapitalProtection, GateEntryHold} {
		for _, kind := range []string{EventPending, EventLatched, EventSold} {
			sequences = append(sequences, []aggragates.TradesStrategyEvents{eventOf(gate, gateDocument(kind, newest, FeatureQuietDecline, FeatureIndecision))})
		}
	}

	for bits := 15; bits >= 0; bits-- {
		switches := [4]bool{bits&8 != 0, bits&4 != 0, bits&2 != 0, bits&1 != 0}
		withQuietDecline(t, switches[0])
		withSwingDecline(t, switches[1])
		withEarlyTakeProfit(t, switches[2])
		withLinearDecline(t, switches[3])
		for _, events := range sequences {
			carrying := withEvents(trade, events...)
			st := rebuildState(carrying)
			want := profitPercentage
			if st.slowDeclinePending && !st.depthPriorityHeld {
				want = math.Max(want, (price-st.lastFill().Price)/price*100)
			}
			if st.earlyTakeProfit {
				want = math.Max(want, (price-carrying.PositionPrice)/price*100)
			}
			if !carriesTakeProfitEvent(carrying) && (st.slowDeclinePending || st.earlyTakeProfit) {
				t.Fatalf("switches %v, events %s: the scan passes over a trade the fold reads pending or latched: %+v", switches, eventDocuments(events), st)
			}
			if got := TakeProfitPercentage(carrying, price, profitPercentage); got != want {
				t.Fatalf("switches %v, events %s: the take profit reads %v, the fold says %v (%+v)", switches, eventDocuments(events), got, want, st)
			}
		}
	}
}

// eventDocuments lists the documents of events with the gates they are filed under,
// for the message of a failure.
func eventDocuments(events []aggragates.TradesStrategyEvents) string {
	listed := make([]string, len(events))
	for index, event := range events {
		listed[index] = event.Gate + " " + string(event.Data)
	}
	return fmt.Sprintf("%q", listed)
}
