package actions

import (
	"fmt"
	"os"
	"testing"
	_ "unsafe" // go:linkname

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/flatmarket"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/quietdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// quietDecline and flatMarketHoldInitialDepth are the smartsell package's own copies of
// quietdecline.Enabled and flatmarket.HoldInitialDepth, read once at its init, before any
// TestMain runs; the entry holds read them, so a pin of the constants alone moves nothing.
//
//go:linkname quietDecline github.com/giovani-sirbu/mercury/strategies/gates/smartsell.quietDecline
var quietDecline bool

//go:linkname flatMarketHoldInitialDepth github.com/giovani-sirbu/mercury/strategies/gates/smartsell.flatMarketHoldInitialDepth
var flatMarketHoldInitialDepth bool

// startStepFromDepth is the depthspacing package's own switch, not a copy: the rows here assert
// the cascade starting at step 1, so it is pinned off.
//
//go:linkname startStepFromDepth github.com/giovani-sirbu/mercury/strategies/gates/cooldown/depthspacing.startStepFromDepth
var startStepFromDepth bool

// TestMain runs every test with the quiet decline's and the flat market's first-fill holds on and
// the flat market reading the window the fixtures serve, and the depth-spacing cascade starting at
// step 1, whatever the constants say: they are hand-tuned, and the tests here assert the holds. It
// stops before any test when the pin did not reach the holds.
func TestMain(m *testing.M) {
	startStepFromDepth = false
	quietdecline.Enabled, quietDecline = true, true
	flatmarket.Enabled, flatmarket.HoldInitialDepth, flatMarketHoldInitialDepth = true, true, true
	flatmarket.MinSmcTrend, flatmarket.RangeBars = 3, 24
	flatmarket.BodyShare, flatmarket.QuietShare, flatmarket.MaxMovePct, flatmarket.MaxCloseRangePct = 0.75, 0.9, 2.0, 3.0
	if err := pinReachedTheHolds(); err != nil {
		fmt.Fprintln(os.Stderr, "TestMain:", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// pinReachedTheHolds is the error of a pin that moved nothing: a verdict of the quiet decline and a
// flat window must each hold a first fill (smartsell.EntryHold). The linked variables are copies the
// composer reads; if one was renamed or is no longer what the hold reads, the hold is not there.
func pinReachedTheHolds() error {
	trade := testutil.NewHoldTrade("buy", false)
	trade.Strategy.Params = aggragates.StrategyParams{SmartSell: true}
	side := aggragates.SideLong

	verdict := aggragates.AIIndicators{SmartSell: aggragates.SmartSellIndicators{QuietDeclineVerdict: true}}
	if hold := smartsell.EntryHold(trade, side, verdict); !hold.Held() || hold.Gate != smartsell.GateEntryHold {
		return fmt.Errorf("the quiet decline's verdict holds no first fill after the pin: smartsell.quietDecline (linked) is no longer the copy EntryHold reads")
	}
	block := flatMarketBlock(side, flatMarketFirstBar)
	if !flatMarketReading(trade, block).Holds {
		return fmt.Errorf("the flat window fixture reads no hold under the pinned flatmarket knobs")
	}
	if hold := smartsell.EntryHold(trade, side, aggragates.AIIndicators{SmartSell: block}); !hold.Held() || hold.Gate != smartsell.GateFlatMarket {
		return fmt.Errorf("the flat window holds no first fill after the pin: smartsell.flatMarketHoldInitialDepth (linked) is no longer the copy EntryHold reads")
	}
	return nil
}
