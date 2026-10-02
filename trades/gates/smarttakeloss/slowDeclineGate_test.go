package smarttakeloss

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
// series (stairBlock); the quiet decline reads the band and the leg the blocks
// below serve beside it. Every expectation is built from the row builders and from
// swingdecline.Trigger through stairReading, so the tables hold under whatever
// constants the rule is tuned to.

// judgedQuiet are the five states of the quiet decline's reading of a new fill:
// the switch off, no band served — so the reading cannot be taken — and, with the
// band served, a reading that holds on the leg on and quiet, one that holds on
// the decline read recently, and one that misses. serve is what sophos adds to
// the block; reasons are what a row names for the state, the break reasons for
// the state that misses.
var judgedQuiet = []struct {
	name    string
	on      bool
	serve   func(aggragates.SmartTakeLossIndicators) aggragates.SmartTakeLossIndicators
	reasons []string
}{
	{"switched off", false, withBreaks, slowDeclineBreakReasons},
	{"no band served", true, withoutTheBand, nil},
	{"holds on the leg on and quiet", true, withLegOnAndQuiet, slowDeclineReasons},
	{"holds on the decline read recently", true, withTheDeclineReadRecently, slowDeclineRecentReasons},
	{"misses", true, withBreaks, slowDeclineBreakReasons},
}

// withTheDeclineReadRecently is the block with a bar the verdict stood on among
// the last closed bars sophos looks back over, the oldest of those bars opening
// before the staircase ladder's fifth fill: what holds a new fill on the decline
// read recently, the leg not on and quiet.
func withTheDeclineReadRecently(block aggragates.SmartTakeLossIndicators) aggragates.SmartTakeLossIndicators {
	block.SlowDeclineRecentAt = stairOpen(29).UnixMilli()
	block.SlowDeclineRecentFrom = stairOpen(28).UnixMilli()
	block.SlowDeclineRecentReasons = slowDeclineRecentReasons
	return block
}

// judgedSwing are the five states of the swing decline's reading of the fifth
// fill, which sits in bar 30: the switch off, no series served, the series closed
// to the bar before the fill's — the reading cannot be taken yet — and the series
// closed to the fill's bar reading a decline that holds, and one that misses.
var judgedSwing = []struct {
	name  string
	on    bool
	serve func() aggragates.SmartTakeLossIndicators
}{
	{"switched off", false, stairFifthBlock},
	{"no series served", true, func() aggragates.SmartTakeLossIndicators { return withoutTheSeries(stairFifthBlock()) }},
	{"bar not closed", true, func() aggragates.SmartTakeLossIndicators { return stairBlock(stairTurns, 29) }},
	{"holds", true, stairFifthBlock},
	{"misses", true, func() aggragates.SmartTakeLossIndicators { return stairBlock(stairBounce, 30) }},
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
// decline is served its band and when the series has closed the fill's bar. The
// fill is confirmed as soon as a taken reading holds, cancelled once every
// applicable reading is taken and none holds, and waits otherwise — also when
// nothing applies at all.
var judgements = [5][5]judgement{
	/* quiet off          */ {judgedWait, judgedWait, judgedWait, judgedConfirm(FeatureSwingDecline), judgedCancel(FeatureSwingDecline)},
	/* no band            */ {judgedWait, judgedWait, judgedWait, judgedConfirm(FeatureSwingDecline), judgedWait},
	/* holds, leg quiet   */ {judgedConfirm(FeatureQuietDecline), judgedConfirm(FeatureQuietDecline), judgedConfirm(FeatureQuietDecline), judgedConfirm(FeatureQuietDecline, FeatureSwingDecline), judgedConfirm(FeatureQuietDecline)},
	/* holds, read recent */ {judgedConfirm(FeatureQuietDecline), judgedConfirm(FeatureQuietDecline), judgedConfirm(FeatureQuietDecline), judgedConfirm(FeatureQuietDecline, FeatureSwingDecline), judgedConfirm(FeatureQuietDecline)},
	/* misses             */ {judgedCancel(FeatureQuietDecline), judgedCancel(FeatureQuietDecline), judgedWait, judgedConfirm(FeatureSwingDecline), judgedCancel(FeatureQuietDecline, FeatureSwingDecline)},
}

// Every pairing of the two readings, the switches included, is judged as the
// table says, on a ladder pending on both declines at its fourth fill whose fifth is new
// and unjudged, with the indecision direction off so no latch joins the row: the
// row of a confirmation names the features that held with the reasons they
// served, that of a cancellation every feature that missed, a waiting fill writes
// nothing and the ladder reads it as unjudged, and the band sells on a tick only
// when the fill ends confirmed with a band served — never over a cancelled fill
// or one still waiting. What the tick wrote is what the fold of the events reads
// back: pending from the new fill on the features that held, or not pending.
func TestANewFillIsJudgedAsTheTableSaysForEveryPairingOfTheReadings(t *testing.T) {
	withIndecision(t, false)
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

				ticked, got := engineTick(trade, "", slowDeclineBand+1, stairOpen(31), withBlock(block))
				st := rebuildState(ticked)
				switch want.outcome {
				case "confirm":
					assertRow(t, got, ConfirmedRow(fill, findings...))
					if !st.slowDeclinePending || st.slowDeclinePendingFrom != fill || !slices.Equal(st.pendingFeatures, want.features) {
						t.Errorf("confirmed: want the ladder pending from the new fill on %v, got %+v", want.features, st)
					}
				case "cancel":
					assertRow(t, got, CancelledRow(fill, findings...))
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

				sells := want.outcome == "confirm" && block.SlowDeclineSellBand > 0
				if sold := got.Reason == reasonSellBand; sold != sells || (got.Position == "sellLoss") != sells {
					t.Errorf("want a sale at the band %v, got %+v", sells, got)
				}
			})
		}
	}
}

// A fill that waits changes nothing and is judged on the first tick its bar has
// closed, whatever the ticks in between served: the band alone never sells it,
// not at the band and not over it, nothing is written however many ticks go by,
// and then the series reading it settles it — confirmed by the swing decline
// alone while the quiet decline still misses, cancelled when both miss — or the
// quiet decline settles it earlier, holding with the bar still open.
func TestAWaitingFillIsJudgedWhenItsBarCloses(t *testing.T) {
	withIndecision(t, false)
	both := []Finding{quietFinding(nil), swingFinding(nil)}
	fill := stairPrices[4]
	pending := pendingAtFourthWithTheFifth(both...)

	waited := func(t *testing.T) aggragates.Trades {
		t.Helper()
		trade := pending
		for minute := 1; minute <= 3; minute++ {
			for _, price := range []float64{slowDeclineBand - 1, slowDeclineBand, slowDeclineBand + 1} {
				assertUntouched(t, Apply(trade, "", price, withBlock(withBreaks(stairBlock(stairTurns, 29)))), "")
			}
			next, got := engineTick(trade, "", slowDeclineBand+1, fifthFillAt.Add(time.Duration(minute)*time.Minute), withBlock(withBreaks(stairBlock(stairTurns, 29))))
			assertUntouched(t, got, "")
			if !reflect.DeepEqual(next, trade) {
				t.Fatalf("a waiting tick wrote into the trade at minute %d", minute)
			}
			trade = next
		}
		if st := rebuildState(trade); !slowDeclineFillUnjudged(trade, st) {
			t.Fatal("the fill must still wait after the ticks")
		}
		return trade
	}

	t.Run("the bar closes on a decline that holds", func(t *testing.T) {
		closed := withBreaks(stairFifthBlock())
		reasons := stairReading(t, pending, closed, true).Reasons
		trade, got := engineTick(waited(t), "", slowDeclineBand+1, stairOpen(31), withBlock(closed))
		assertRow(t, got, ConfirmedRow(fill, swingFinding(reasons)))
		assertForced(t, got, reasonSellBand)
		if st := rebuildState(trade); !st.slowDeclinePending || st.slowDeclinePendingFrom != fill || !slices.Equal(st.pendingFeatures, []string{FeatureSwingDecline}) {
			t.Errorf("pending from the new fill on the swing decline alone, got %+v", st)
		}
		if len(trade.Logs) != len(pending.Logs)+1 || len(trade.StrategyEvents) != len(pending.StrategyEvents)+1 {
			t.Errorf("one row and one event for the whole wait and its end, got %d and %d over %d and %d", len(trade.Logs), len(trade.StrategyEvents), len(pending.Logs), len(pending.StrategyEvents))
		}
	})

	t.Run("the bar closes on a rally", func(t *testing.T) {
		rally := withBreaks(stairBlock(stairBounce, 30))
		broke := stairReading(t, pending, rally, false).Breaks
		trade, got := engineTick(waited(t), "", slowDeclineBand+1, stairOpen(31), withBlock(rally))
		assertRow(t, got, CancelledRow(fill, quietFinding(slowDeclineBreakReasons), swingFinding(broke)))
		assertNoSale(t, got, "")
		if st := rebuildState(trade); st.slowDeclinePending {
			t.Errorf("cancelled on both readings, got %+v", st)
		}
	})

	t.Run("the quiet decline holds before the bar closes", func(t *testing.T) {
		open := withLegOnAndQuiet(stairBlock(stairTurns, 29))
		_, got := engineTick(waited(t), "", slowDeclineBand+1, stairOpen(31), withBlock(open))
		assertRow(t, got, ConfirmedRow(fill, quietFinding(slowDeclineReasons)))
		assertForced(t, got, reasonSellBand)
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
// features — alone and in every pair of them, under every combination of the three
// feature switches, the trade the scan passes over is neither pending nor latched,
// and the take profit reads what the fold of the events says — the move from the
// newest fill while pending, the move from the position price once latched —
// whichever way the scan answers. A state the scan hid from the fold would leave
// a pending ladder's take profit on the average entry price.
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

	for _, switches := range [][3]bool{{true, true, true}, {true, true, false}, {true, false, true}, {true, false, false}, {false, true, true}, {false, true, false}, {false, false, true}, {false, false, false}} {
		withQuietDecline(t, switches[0])
		withSwingDecline(t, switches[1])
		withIndecision(t, switches[2])
		for _, events := range sequences {
			carrying := withEvents(trade, events...)
			st := rebuildState(carrying)
			want := profitPercentage
			if st.slowDeclinePending && !st.depthPriorityHeld {
				want = math.Max(want, (price-st.lastFill().Price)/price*100)
			}
			if st.indecision {
				want = math.Max(want, (price-carrying.PositionPrice)/price*100)
			}
			if !carriesTakeProfitEvent(carrying) && (st.slowDeclinePending || st.indecision) {
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
