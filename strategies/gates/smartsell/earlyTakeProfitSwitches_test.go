package smartsell

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// withEarlyTakeProfitSwingDecline is withIndecision for the latch on the swing decline.
func withEarlyTakeProfitSwingDecline(t *testing.T, on bool) {
	t.Helper()
	previous := earlyTakeProfitSwingDecline
	earlyTakeProfitSwingDecline = on
	t.Cleanup(func() { earlyTakeProfitSwingDecline = previous })
}

// withEarlyTakeProfitLinearDecline is withIndecision for the latch on the linear decline.
func withEarlyTakeProfitLinearDecline(t *testing.T, on bool) {
	t.Helper()
	previous := earlyTakeProfitLinearDecline
	earlyTakeProfitLinearDecline = on
	t.Cleanup(func() { earlyTakeProfitLinearDecline = previous })
}

// withEveryEarlyTakeProfitLatch runs the calling test with the three switchable latches on.
func withEveryEarlyTakeProfitLatch(t *testing.T) {
	t.Helper()
	withEarlyTakeProfitSwingDecline(t, true)
	withEarlyTakeProfitLinearDecline(t, true)
	withEarlyTakeProfitFlatMarket(t, true)
}

// withTheFeaturesOff runs the calling test with the gates that write rows of their own off and the sale unbounded.
func withTheFeaturesOff(t *testing.T) {
	t.Helper()
	withEarlyTakeProfit(t, true)
	withQuietDecline(t, false)
	withSwingDecline(t, false)
	withLinearDecline(t, false)
	withFlatMarket(t, false)
	withCapitalProtectionExit(t, false)
	withAllowedLoss(t, anyLoss)
	withSaleIgnoreLastDepths(t, 0)
}

// switchScene is one tick that latches on a single reading, with the row it writes switched on and the rows switched off.
type switchScene struct {
	trade aggragates.Trades
	price float64
	ai    aggragates.AIIndicators
	at    time.Time
	on    Row
	off   []Row
}

// A latch switch off removes the latch of its reading alone: the tick writes no latch and the ladder reads unlatched.
func TestAnEarlyTakeProfitLatchSwitchOffRemovesOnlyItsOwnReading(t *testing.T) {
	cases := []struct {
		name    string
		set     func(*testing.T, bool)
		prepare func(*testing.T) switchScene
	}{
		{"the swing decline on the row of the tick", withEarlyTakeProfitSwingDecline, func(t *testing.T) switchScene {
			withTheFeaturesOff(t)
			withBothDeclines(t)
			trade, block := stairFourth(), stairFourthBlock()
			reading, holds := readsAsOf(trade, block)
			needsAScenario(t, rebuildState(trade).swingDeclineWatched && holds, "a staircase the swing decline reads at the fourth fill")
			return switchScene{trade, gainPrint(trade), withBlock(block), stairOpen(26),
				stairPendingRow(t), []Row{PendingRow(stairPrices[3], swingFinding(reading.Reasons))}}
		}},
		{"the swing decline on a judged newest fill", withEarlyTakeProfitSwingDecline, func(t *testing.T) switchScene {
			withTheFeaturesOff(t)
			withBothDeclines(t)
			pending, block := withRows(stairFourth(), PendingRow(stairPrices[3], quietFinding(slowDeclineReasons))), stairFourthBlock()
			reading, holds := readsAsOf(pending, block)
			needsAScenario(t, rebuildState(pending).swingDeclineWatched && holds, "a staircase the swing decline reads at the fourth fill")
			return switchScene{pending, gainPrint(pending), withBlock(block), stairOpen(26),
				LatchedRow(stairPrices[3], swingFinding(reading.Reasons)), nil}
		}},
		{"the linear decline", withEarlyTakeProfitLinearDecline, func(t *testing.T) switchScene {
			onlyTheLinearDecline(t)
			trade, block := linearScene(linearTrendMet)
			return switchScene{trade, trade.PositionPrice, withBlock(block), testutil.At("23:00:00"),
				linearRow(t, trade, block), nil}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withEveryEarlyTakeProfitLatch(t)
			scene := c.prepare(t)

			latched, got := tickResting(scene.trade, "", scene.price, scene.at, scene.ai)
			assertRow(t, got, scene.on)
			assertNoSale(t, got, "")
			if !rebuildState(latched).earlyTakeProfit {
				t.Fatalf("fixture drifted: the switch on, the tick must latch the ladder, got %+v", got)
			}

			c.set(t, false)
			unlatched, got := tickResting(scene.trade, "", scene.price, scene.at, scene.ai)
			assertRows(t, got, scene.off...)
			assertNoSale(t, got, "")
			if latchNamed(got) || rebuildState(unlatched).earlyTakeProfit {
				t.Fatalf("the switch off, the tick must latch nothing, got %+v", got)
			}
		})
	}
}
