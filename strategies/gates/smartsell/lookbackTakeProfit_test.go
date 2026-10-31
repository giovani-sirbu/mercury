package smartsell

import (
	"math"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// Trade 117291: the indecision stood on the bar open 08:00, the first fill came at 12:22:37.
var (
	lbStood      = time.Date(2022, time.June, 9, 8, 0, 0, 0, time.UTC)
	lbFirstFill  = time.Date(2022, time.June, 9, 12, 22, 37, 0, time.UTC)
	lbLookReason = []string{"indecision read on the 1h bar opening 2022-06-09 08:00 UTC, one of the last closed bars", "smc-trend 1h: indecision, every configured timeframe within the tolerance (1h 4/9, 4h 6/9)"}
)

// lbLookback is a block whose flag is false and whose look-back names the bar that read the indecision, on a read window.
func lbLookback() aggragates.SmartSellIndicators {
	return aggragates.SmartSellIndicators{QuietDeclineFillFrom: lbBar(0).UnixMilli(), IndecisionRecentAt: lbStood.UnixMilli(), IndecisionRecentReasons: lbLookReason}
}

// lbDeeper is the trade with one more fill, its quantity double the one before's.
func lbDeeper(trade aggragates.Trades, price float64, at time.Time) aggragates.Trades {
	trade = withFill(trade, price, at)
	trade.History[len(trade.History)-1].Quantity = math.Pow(2, float64(len(trade.History)-1))
	return trade
}

// With the early take profit watching from the first fill (arm depth one), the first fill, 4 h 22 min
// after the bar, is latched on its first print, once: one row at its price naming the bar's reasons, and not a second whether the look-back is still served, no
// longer served or the ladder deepens. It is not pending, so the shared action does nothing,
// activating no trailingBuy and selling nothing, an arming proposal included — and where the quiet
// decline's look-back names the same bar, a ladder opened after it goes pending, and the action
// acts on it from that tick: the trailingBuy of the ladder in buy activated, the ladder in trailingBuy sold,
// the dashboard served (the shared sale reads it: without one it holds, SMC trend not read).
func TestLookbackAFirstFillStartsLatchedOnceAndNotPending(t *testing.T) {
	lbGate(t, false, true)
	withEarlyTakeProfitArmDepth(t, 1)
	trade := withFill(testutil.LadderTrade(false), lbPrice(0), lbFirstFill)
	var rows []Row
	for hour := 12; hour < 60; hour++ {
		if hour == 20 || hour == 36 || hour == 40 {
			trade = lbDeeper(trade, lbPrice(len(trade.History)), lbBar(hour).Add(2*time.Minute))
		}
		block := lbLookback()
		if hour >= 44 { // sophos looks back no further
			block.IndecisionRecentAt, block.IndecisionRecentReasons = 0, nil
		}
		for _, position := range []string{"", "trailingBuy"} {
			price := map[string]float64{"": lbBelow, "trailingBuy": armingPrice(trade)}[position]
			var got Result
			trade, got = engineTick(trade, position, price, lbBar(hour).Add(30*time.Minute), withBlock(block))
			if rows = append(rows, got.Rows...); got.Reason != "" || got.Position != position || got.Activated {
				t.Fatalf("hour %d on %q at %v: a latched ladder that is not pending is neither activated nor sold, an arming proposal included, got %+v", hour, position, price, got)
			}
		}
	}
	want := "smartSell: early take profit latched (indecision: " + strings.Join(lbLookReason, ", ") + ")"
	if st := rebuildState(trade); len(rows) != 1 || rows[0].Message != want || rows[0].Event != EventLatched || rows[0].Price != lbPrice(0) || !st.earlyTakeProfit || st.slowDeclinePending {
		t.Fatalf("rows %+v, want the one latch at the first fill's price, %q, and a ladder latched and not pending", rows, want)
	}

	quiet := lbLookback()
	quiet.QuietDeclineRecentAt, quiet.QuietDeclineRecentFrom, quiet.QuietDeclineFillFrom, quiet.QuietDeclineRecentReasons = lbStood.UnixMilli(), lbStood.Add(-23*time.Hour).UnixMilli(), lbBar(0).UnixMilli(), quietDeclineRecentReasons
	assertRow(t, Apply(withFill(testutil.LadderTrade(false), lbPrice(0), lbFirstFill), "", lbBand+1, withBlock(quiet)), LatchedRow(lbPrice(0), indecisionFinding(lbLookReason)))
	standing := withFill(withFill(testutil.LadderTrade(false), lbPrice(0), lbStood.Add(-90*time.Minute)), lbPrice(1), lbFirstFill)
	assertActedOn(t, standing, withBlock(withEveryRowTrend(quiet)), PendingRow(lbPrice(1), indecisionFinding(lbLookReason), quietFinding(quietDeclineRecentReasons)))
}

// From break even up the take profit of a ladder the look-back latched arms at the last buy's
// level, the move read from the position price, where the average entry price's would arm it
// later; a last buy not under the average arms at the average's, under break even nothing
// moves, and the unlatched ladder reads its input throughout. Latched by the engines' own tick, on the
// first fill: the early take profit watches from it here (arm depth one).
func TestLookbackTheTakeProfitOfALatchedLadderArmsFromTheLastBuy(t *testing.T) {
	lbGate(t, false, false)
	withEarlyTakeProfitArmDepth(t, 1)
	for name, tc := range map[string]struct {
		prices []float64
		lower  bool
	}{
		"the four fills of trade 117291":          {[]float64{40.56, 39.61, 38.68, 37.13}, true},
		"two fills, the last buy over break even": {[]float64{40.56, 39.61}, true},
		"a last buy over the average":             {[]float64{37.13, 38.68}, false},
	} {
		t.Run(name, func(t *testing.T) {
			plain := withFill(testutil.LadderTrade(false), tc.prices[0], lbFirstFill)
			latched, got := engineTick(plain, "", lbBelow, lbFirstFill.Add(time.Minute), withBlock(lbLookback()))
			for index, price := range tc.prices[1:] {
				at := lbFirstFill.Add(time.Duration(index+1) * time.Hour)
				plain, latched = lbDeeper(plain, price, at), lbDeeper(latched, price, at)
			}
			average, last, step := ladder.AverageEntryPrice(latched), latched.PositionPrice, takeProfitStep(latched)
			if len(got.Rows) != 1 || got.Rows[0].Event != EventLatched || (last < average) != tc.lower {
				t.Fatalf("fixture drifted: the first print must latch the ladder (%+v) and its last buy %v sit under its average %v: %v", got, last, average, tc.lower)
			}
			var armedFrom, armedPlain float64
			for price := average * 0.9; price < average*1.1; price += average / 20000 {
				input := (price - average) / price * 100
				want := input
				if input >= 0 {
					want = math.Max(input, (price-last)/price*100)
				}
				if gotLatched, gotPlain := TakeProfitPercentage(latched, price, input), TakeProfitPercentage(plain, price, input); gotLatched != want || gotPlain != input {
					t.Fatalf("at %v: latched read %v, want %v; unlatched read %v, want its input %v", price, gotLatched, want, gotPlain, input)
				}
				if armedFrom == 0 && want >= step {
					armedFrom = price
				}
				if armedPlain == 0 && input >= step {
					armedPlain = price
				}
			}
			slack, wantFrom := average/10000, math.Max(average, math.Min(average, last)/(1-step/100))
			if math.Abs(armedFrom-wantFrom) > slack || math.Abs(armedPlain-average/(1-step/100)) > slack || (tc.lower && armedFrom >= armedPlain-slack) {
				t.Errorf("armed from %v (want the last buy's level %v) and unlatched from %v (want the average's %v)", armedFrom, wantFrom, armedPlain, average/(1-step/100))
			}
		})
	}
}

// lbRandomLife plays one life drawn from seed — a staircase, two to eight fills, and per print a trend,
// series, flag, look-back, price and now and then a hold — on the process memo or, `fresh`, an empty one.
// The life is played as the engines play it (lbLifeStepping): the print that activates the trailingBuy leaves
// the ladder resting in trailingBuy, where the prints after it find it.
func lbRandomLife(seed int64, fresh bool) []lbPrint {
	rng := rand.New(rand.NewSource(seed))
	closes, need := lbTape(2+rng.Intn(8)), max(swingdecline.MinSmcTrendDown, 0)
	if rng.Intn(2) == 0 {
		for bar := lbLegBars + rng.Intn(len(closes)-lbLegBars); bar < len(closes); bar++ {
			closes[bar] *= 1 + 0.05*rng.Float64()
		}
	}
	var fills []lbFill
	for bar, count := lbLegBars+rng.Intn(3), 2+rng.Intn(7); bar < len(closes)-2 && len(fills) < count; bar += 1 + rng.Intn(6) {
		fills = append(fills, lbFill{bar, lbPrice(len(fills))})
	}
	return lbLifeStepping("", closes, fills, fills[0].bar, len(closes)+2, func(hour, minute int, trade *aggragates.Trades, block *aggragates.SmartSellIndicators) float64 {
		if fresh {
			swingMemo = newSwingDeclineMemo(swingDeclineMemoBound, swingdecline.TriggerAtLastBar)
		}
		block.SmcTrendDown = []int{0, max(need-1, 0), need, need, 3}[rng.Intn(5)]
		if older := slices.Clone(block.SeriesCloses); rng.Intn(6) == 0 && len(older) > 3 {
			older[len(older)/2] *= 1.07
			block.SeriesCloses = older
		}
		if rng.Intn(240) == 0 {
			block.Indecision, block.QuietDeclineBreakReasons = true, indecisionReasons
		}
		if rng.Intn(40) == 0 {
			block.IndecisionRecentAt, block.IndecisionRecentReasons = lbBar(rng.Intn(hour)).UnixMilli(), lbLookReason
		}
		if rng.Intn(320) == 0 && len(trade.History) > 0 {
			*trade = heldBy(*trade, lbBar(hour).Add(lbMinutes[minute]))
		}
		return []float64{lbBelow, lbBand + 1}[rng.Intn(2)]
	})
}

// Over three hundred random lives, on every print: the memo answers as a fresh read does; the
// indecision latches a ladder once, naming the flag's reasons before the look-back's; a held ladder
// writes only its reset and latch and activates and sells nothing; a swing trigger needs a ladder
// watched, unheld, unpending; an activation (from buy) answers trailingBuy, sells nothing and ends on its
// activation row, a sale writes none, and the rows hold no activation row but on an activation. A
// re-anchor is an Activated answer with no activation row: it answers trailingBuy, sells nothing and is
// made only of a ladder whose trailingBuy a gate has already activated. A ladder so activated never
// has a second activation row written.
func TestLookbackRandomLivesKeepTheGatesRules(t *testing.T) {
	lbGate(t, true, true)
	withSwingMemo(t, swingMemo)
	var triggers, held, activations, reanchors, sales int
	for seed := int64(1); seed <= 300; seed++ {
		latches, fresh := 0, lbRandomLife(seed, true)
		for index, step := range lbRandomLife(seed, false) {
			if !reflect.DeepEqual(step.result, fresh[index].result) {
				t.Fatalf("seed %d at %s: the memo answered %+v, a fresh read %+v", seed, step.at, step.result, fresh[index].result)
			}
			if step.before.depthPriorityHeld {
				if held++; step.result.Reason != "" || step.result.Activated {
					t.Fatalf("seed %d at %s: a held ladder acted %+v", seed, step.at, step.result)
				}
			}
			result := step.result
			wrote := slices.ContainsFunc(result.Rows, func(row Row) bool {
				return row.Event == EventActivated
			})
			committed := step.before.slowDeclineActivated || step.before.flatMarketActivated
			if wrote && (!result.Activated || committed) {
				t.Fatalf("seed %d at %s: an activation row is written on an activation of a ladder not activated yet and on no other tick, got %+v", seed, step.at, result)
			}
			if result.Activated && wrote {
				activations++
				if last := result.Rows[len(result.Rows)-1]; result.Position != last.State || result.Reason != "" || last.Event != EventActivated {
					t.Fatalf("seed %d at %s: an activation answers the state its row names, sells nothing and ends on its activation row, got %+v", seed, step.at, result)
				}
			}
			if result.Activated && !wrote {
				reanchors++
				in := step.before.slowDeclineActivatedIn + step.before.flatMarketActivatedIn
				if !committed || result.Position != in || result.Reason != "" {
					t.Fatalf("seed %d at %s: a re-anchor answers the state a gate activated and sells nothing, got %+v with state %+v", seed, step.at, result, step.before)
				}
			}
			if result.Reason != "" {
				sales++
			}
			for _, row := range step.result.Rows {
				if slices.Contains(row.Features, FeatureEarlyTakeProfit) {
					if latches++; latches > 1 || step.before.earlyTakeProfit || !step.before.earlyTakeProfitWatched {
						t.Fatalf("seed %d at %s: %q latches a ladder in state %+v, the %dth latch of its life", seed, step.at, row.Message, step.before, latches)
					}
				}
				if want := map[bool][]string{true: indecisionReasons, false: lbLookReason}[step.block.Indecision]; row.Event == EventLatched && (step.block.Indecision || step.block.IndecisionRecentAt > 0) && !slices.Equal(row.Reasons, want) {
					t.Fatalf("seed %d at %s: the latch names %q, want %q", seed, step.at, row.Reasons, want)
				}
				if step.before.depthPriorityHeld && row.Event != EventReset && row.Event != EventLatched {
					t.Fatalf("seed %d at %s: a held ladder wrote %q", seed, step.at, row.Message)
				}
				if strings.Contains(row.Message, "swing decline triggered") {
					if triggers++; step.before.slowDeclinePending || step.before.depthPriorityHeld || !step.before.swingDeclineWatched {
						t.Fatalf("seed %d at %s: %q on a ladder in state %+v", seed, step.at, row.Message, step.before)
					}
				}
			}
		}
	}
	needsAScenario(t, triggers > 0 && held > 0, "lives with a swing decline trigger and a hold")
	t.Logf("300 lives: %d swing decline triggers, %d held prints, %d activations, %d re-anchors, %d sales", triggers, held, activations, reanchors, sales)
	if activations == 0 || reanchors == 0 || sales == 0 {
		t.Fatalf("fixture drifted: lives that trigger the swing decline must activate a trailingBuy, re-anchor it and sell, got %d activations, %d re-anchors and %d sales", activations, reanchors, sales)
	}
}
