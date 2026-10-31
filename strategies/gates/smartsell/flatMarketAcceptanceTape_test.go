package smartsell

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/flatmarket"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The material of the flat market gate's acceptance tests (flatMarketAcceptance_test.go):
// the loader of the real 1h tapes the flat market package keeps in its testdata, the two
// ladders of real fills those tests replay on them, and the oracles the gate's answers are
// checked against, each written out from the tape and the constants alone. The ETH
// ladder is shallow and is read at its own fills. The LINK ladder is deep: it is read at
// all its fills, past the shared sale's depth bound, and at its first fills, the deepest
// depth that bound lets the sale be made at (chainlinkShallow).

const (
	// sophosLookbackBars is FlatMarketBars of sophos' shared/smartsell/lowinterest
	// package: how many closed 1h bars sophos serves the flat market gate. Mercury cannot
	// import sophos, so the tests that stand in for it write the number out.
	sophosLookbackBars = 24
	// dashboardRows is how many rows of the SMC trend dashboard sophos reads, one per
	// timeframe, and quietBodies how many of a window's bodies are quiet in both replays.
	dashboardRows = 3
	quietBodies   = 21
	// barStamp and secondStamp are how the tests write the open of a bar (UTC, to the
	// minute) and the stamp of a fill or of a tick (UTC, to the second).
	barStamp    = "2006-01-02 15:04"
	secondStamp = "2006-01-02 15:04:05"
)

// tapeFields are the fields of a bar of a tape, in the order the file lists them: the open
// time in ms, then the open, high, low and close as the exchange's decimal strings.
var tapeFields = []string{"openTime", "open", "high", "low", "close"}

// replayPercentage and replayIgnoreLastDepths are the cap and the depth bound of the shared
// sale the replays were measured at. The replays set them themselves, as TestMain leaves
// the package's at off, so that a retune of SaleTriggerPercentage or SaleIgnoreLastDepths
// moves no replay: they measure the sale at these numbers, the ETH ladder inside the
// bound and the LINK ladder past it, and what the constants are tuned to is the business
// of the tests of the action.
const (
	replayPercentage       = 1.0
	replayIgnoreLastDepths = 3
)

// replayFlatMarket readies a replay: it skips the test, naming the constant, when a retune
// has made its measurements meaningless (the gate switched off or a number of its reading
// moved from the one it was measured at), and otherwise runs it with the flat market gate
// on, the shared sale's cap at replayPercentage and its depth bound at replayIgnoreLastDepths,
// and every other rule off, so that what a tick hands back is the gate's alone whatever the
// other switches ship as. Unlike flatMarketAlone it asks the gate for no market first: a
// gate that has stopped reading fails the replay and never skips it. It fails the test when
// the depth bound no longer lies between the two ladders.
func replayFlatMarket(t *testing.T) {
	t.Helper()
	if !flatmarket.Enabled {
		t.Skip("flatmarket.Enabled is off: the gate watches, reads and holds nothing")
	}
	for _, constant := range []struct {
		name                string
		inForce, measuredAs float64
	}{
		{"flatmarket.BodyShare", flatmarket.BodyShare, 0.5},
		{"flatmarket.QuietShare", flatmarket.QuietShare, 0.85},
		{"flatmarket.MaxMovePct", flatmarket.MaxMovePct, 5.0},
		{"flatmarket.MinSmcTrend", float64(flatmarket.MinSmcTrend), 3},
	} {
		if constant.inForce != constant.measuredAs {
			t.Skipf("%s is %v, the replays were measured at %v", constant.name, constant.inForce, constant.measuredAs)
		}
	}
	if deepest := lastDepthFills - replayIgnoreLastDepths; len(ethereumOrders) > deepest || len(chainlinkOrders) <= deepest || deepest < 1 {
		t.Fatalf("fixture drifted: the depth bound must let the ETH ladder be sold and hold the LINK ladder, got %d depths of %d", deepest, lastDepthFills)
	}
	withFlatMarket(t, true)
	withAllowedLoss(t, replayPercentage)
	withSaleIgnoreLastDepths(t, replayIgnoreLastDepths)
	withQuietDecline(t, false)
	withSwingDecline(t, false)
	withEarlyTakeProfit(t, false)
	withCapitalProtectionExit(t, false)
}

// utcAt is a UTC stamp written to the second, as the real ladders list their fills.
func utcAt(text string) time.Time {
	at, err := time.Parse(secondStamp, text)
	if err != nil {
		panic(err)
	}
	return at.UTC()
}

// tapeWindowBlock is the block sophos serves at a tick whose last closed bar opened at
// lastOpen: the sophosLookbackBars bars of the tape that end at it, oldest first, on the
// five flatMarket arrays, and the counts of the Trend Direction row as given. It fails the
// test on a tape it cannot read and on a bar that is missing: none opens at lastOpen, fewer
// than the look-back end there, or two bars of the window are not one bar apart.
func tapeWindowBlock(t *testing.T, name string, lastOpen time.Time, down, up, valid int) aggragates.SmartSellIndicators {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("lowinterest", "testdata", name))
	if err != nil {
		t.Fatalf("read the tape: %v", err)
	}
	var tape struct {
		Interval string          `json:"interval"`
		Fields   []string        `json:"fields"`
		Bars     [][]json.Number `json:"bars"`
	}
	if err := json.Unmarshal(raw, &tape); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if tape.Interval != "1h" || !slices.Equal(tape.Fields, tapeFields) {
		t.Fatalf("%s: the tape holds %s bars of %q, want 1h bars of %q", name, tape.Interval, tape.Fields, tapeFields)
	}
	bars := make([][5]float64, len(tape.Bars))
	for index, cells := range tape.Bars {
		if len(cells) != len(tapeFields) {
			t.Fatalf("%s: bar %d holds %d values, want %d", name, index, len(cells), len(tapeFields))
		}
		for slot, cell := range cells {
			if bars[index][slot], err = cell.Float64(); err != nil {
				t.Fatalf("%s: bar %d, %s: %v", name, index, tapeFields[slot], err)
			}
		}
	}

	end := slices.IndexFunc(bars, func(bar [5]float64) bool { return int64(bar[0]) == lastOpen.UnixMilli() })
	if end < 0 || end+1 < sophosLookbackBars {
		t.Fatalf("%s: want %d bars ending at the bar opening %s, the tape has none or fewer", name, sophosLookbackBars, lastOpen.UTC().Format(barStamp))
	}
	var block aggragates.SmartSellIndicators
	window := bars[end+1-sophosLookbackBars : end+1]
	for index, bar := range window {
		if index > 0 && int64(bar[0])-int64(window[index-1][0]) != time.Hour.Milliseconds() {
			t.Fatalf("%s: a bar is missing before the bar opening %s", name, time.UnixMilli(int64(bar[0])).UTC().Format(barStamp))
		}
		block.FlatMarketBarOpens = append(block.FlatMarketBarOpens, int64(bar[0]))
		block.FlatMarketOpenPrices = append(block.FlatMarketOpenPrices, bar[1])
		block.FlatMarketHighs = append(block.FlatMarketHighs, bar[2])
		block.FlatMarketLows = append(block.FlatMarketLows, bar[3])
		block.FlatMarketCloses = append(block.FlatMarketCloses, bar[4])
	}
	block.SmcTrendDown, block.SmcTrendUp, block.SmcTrendValid = down, up, valid
	return block
}

// realFill is an executed entry order of a real ladder: the stamp of its history row, the
// price it paid and the quantity it bought.
type realFill struct {
	at              time.Time
	price, quantity float64
}

func realFillAt(stamp string, price, quantity float64) realFill {
	return realFill{utcAt(stamp), price, quantity}
}

// ethereumOrders are the two fills of the ladder on the ETH pair, chainlinkOrders the six
// of the ladder on the LINK pair.
var (
	ethereumOrders = []realFill{
		realFillAt("2022-06-07 20:06:35", 1862.57, 0.0793),
		realFillAt("2022-06-07 22:48:23", 1788.53, 0.1586),
	}
	chainlinkOrders = []realFill{
		realFillAt("2022-05-05 03:10:00", 12.21, 5.55),
		realFillAt("2022-05-05 09:52:00", 11.89, 11.1),
		realFillAt("2022-05-05 14:26:47", 11.58, 22.2),
		realFillAt("2022-05-05 15:08:27", 11.03, 44.4),
		realFillAt("2022-05-06 07:48:32", 10.74, 88.8),
		realFillAt("2022-05-06 13:37:32", 10.46, 177.6),
	}
)

// realLadder is a long spot parent under the flag on a pair priced to filters and
// configured at percentage, whose history holds these entry orders in order: the ladder
// fixture of testutil with the quantity each order bought.
func realLadder(symbol string, filters aggragates.TradeFilters, percentage float64, orders []realFill) aggragates.Trades {
	entries := make([]testutil.LadderFill, len(orders))
	for index, order := range orders {
		entries[index] = testutil.LadderFill{Price: order.price, At: order.at}
	}
	trade := testutil.LadderTrade(false, entries...)
	trade.Symbol, trade.StrategyPair.TradeFilters = symbol, filters
	trade.StrategyPair.StrategySettings[0].Percentage = percentage
	for index, order := range orders {
		trade.History[index].Quantity = order.quantity
	}
	return trade
}

func ethereumLadder(orders []realFill) aggragates.Trades {
	return realLadder("ETH/USDT", aggragates.TradeFilters{LotSize: 4, PriceFilter: 2}, 2.2, orders)
}

func chainlinkLadder(orders []realFill) aggragates.Trades {
	return realLadder("LINK/USDT", aggragates.TradeFilters{LotSize: 2, PriceFilter: 2}, 2.4, orders)
}

// chainlinkShallow is the first fills of the LINK ladder, as many as the shared sale's
// depth bound lets a sale be made at: its ceiling less the depths it needs left
// (replayIgnoreLastDepths). replayFlatMarket has failed the test where that leaves the ladder
// no shallow depth or no deep one.
func chainlinkShallow() []realFill {
	return chainlinkOrders[:lastDepthFills-replayIgnoreLastDepths]
}

// strayOfCloses is the mean of the first close and the last and the stray of the closes
// between them, written out from the closes alone: the one farthest from the mean, in
// percent of the mean and signed, the earlier of two as far off, and zero where none lies
// off the mean.
func strayOfCloses(closes []float64) (mean, stray float64) {
	mean = (closes[0] + closes[len(closes)-1]) / 2
	for _, closed := range closes[1 : len(closes)-1] {
		if deviation := (closed - mean) / mean * 100; math.Abs(deviation) > math.Abs(stray) {
			stray = deviation
		}
	}
	return mean, stray
}

// holdingReasons are the reasons the reading of a long ladder names over the window the
// block serves when it holds, written out from the window and the constants alone: the
// frame, the quiet bodies, the fall, the closes against their mean and the trend. It fails
// the test when the window does not hold the quiet bodies the replays were measured with.
// The quiet share and the move bound are converted to floats before they are printed, as
// the cap is in saleHeldMessage, and the mean in the shortest form of itself rounded to
// the eighth decimal, as the reason prints it.
func holdingReasons(t *testing.T, block aggragates.SmartSellIndicators, percentage float64) []string {
	t.Helper()
	opens, last := block.FlatMarketBarOpens, len(block.FlatMarketBarOpens)-1
	limit, quiet := flatmarket.BodyShare*percentage, 0
	for index, opened := range block.FlatMarketOpenPrices {
		if math.Abs(block.FlatMarketCloses[index]-opened)/opened*100 < limit {
			quiet++
		}
	}
	if quiet != quietBodies {
		t.Fatalf("fixture drifted: %d quiet bodies on the tape's window, want %d", quiet, quietBodies)
	}
	fall := (block.FlatMarketHighs[0] - block.FlatMarketLows[last]) / block.FlatMarketHighs[0] * 100
	mean, stray := strayOfCloses(block.FlatMarketCloses)
	return []string{
		fmt.Sprintf("1h bars %s to %s UTC", time.UnixMilli(opens[0]).UTC().Format(barStamp), time.UnixMilli(opens[last]).UTC().Format(barStamp)),
		fmt.Sprintf("quiet bodies %d/%d under %.2f%%, over %.0f%%", quiet, len(opens), limit, float64(flatmarket.QuietShare)*100),
		fmt.Sprintf("fall %.1f%% under %.1f%%", fall, float64(flatmarket.MaxMovePct)),
		fmt.Sprintf(
			"closes within %.1f%% of the mean %s, at most %.1f%%",
			math.Abs(stray), strconv.FormatFloat(math.Round(mean*1e8)/1e8, 'f', -1, 64), float64(flatmarket.MaxMovePct),
		),
		fmt.Sprintf("smc trend direction down %d/%d of %d needed", block.SmcTrendDown, block.SmcTrendValid, flatmarket.MinSmcTrend),
	}
}

// triggeredMessage, saleHeldMessage and saleHeldAtDepthMessage are the texts of the
// episode's rows: the reasons of the reading in parentheses; the loss and the cap with two
// decimals, or the depth the ladder has filled, its ceiling and the deepest depth the sale
// is made at, for a sale the shared action holds. The cap is converted to a float, so that
// a cap retuned to a whole number is still a number the verb prints (an untyped whole
// constant would be an int and fail vet).
func triggeredMessage(reasons []string) string {
	return "smartSell: flat market triggered (" + strings.Join(reasons, ", ") + ")"
}

func saleHeldMessage(lossPct float64) string {
	return fmt.Sprintf("smartSell: flat market sale held, profit %.2f%% of the used amount under %.2f%%", 0-lossPct, -float64(replayPercentage))
}

func saleHeldAtDepthMessage(filled, ceiling int) string {
	return fmt.Sprintf("smartSell: flat market sale held, depth %d of %d over %d", filled, ceiling, ceiling-replayIgnoreLastDepths)
}

// eventAt is an event as episodeEvents lists it: its kind and the price it carries.
func eventAt(kind string, price float64) string {
	return fmt.Sprintf("%s@%v", kind, price)
}

// episodeEvents lists the trade's flat market events in the order they were written,
// each as the fold reads it (readEvent): its kind and its price.
func episodeEvents(t *testing.T, trade aggragates.Trades) []string {
	t.Helper()
	var listed []string
	for _, event := range trade.StrategyEventsOf(aggragates.StrategyParamSmartSell, GateFlatMarket) {
		var data EventData
		if err := event.DecodeData(&data); err != nil {
			t.Fatalf("event %+v does not decode: %v", event, err)
		}
		listed = append(listed, eventAt(data.Event, data.Price))
	}
	return listed
}

// assertEpisodeRow fails unless row is the flat market gate's row of that kind: filed
// under the gate at the price given, naming no feature, worded exactly as message.
func assertEpisodeRow(t *testing.T, row Row, kind string, price float64, message string) {
	t.Helper()
	if row.Gate != GateFlatMarket || row.Event != kind || row.Price != price || row.Message != message || len(row.Features) != 0 {
		t.Fatalf("row = %+v, want the %q event of gate %q at %v worded %q", row, kind, GateFlatMarket, price, message)
	}
}

// lastBars is the block cut to its last n bars, the five arrays alike and the counts of the
// Trend Direction row as they were: the window a sophos that served fewer bars would send.
func lastBars(block aggragates.SmartSellIndicators, n int) aggragates.SmartSellIndicators {
	from := len(block.FlatMarketBarOpens) - n
	cut := block
	cut.FlatMarketBarOpens = slices.Clone(block.FlatMarketBarOpens[from:])
	cut.FlatMarketOpenPrices = slices.Clone(block.FlatMarketOpenPrices[from:])
	cut.FlatMarketHighs = slices.Clone(block.FlatMarketHighs[from:])
	cut.FlatMarketLows = slices.Clone(block.FlatMarketLows[from:])
	cut.FlatMarketCloses = slices.Clone(block.FlatMarketCloses[from:])
	return cut
}

// The floor of a reading is two bars, written out here apart from flatmarket.MinBars so that
// a retune of that constant is caught, the tests of its package being relative to it: a share
// of the bars and a move from the oldest bar to the newest need an oldest bar apart from a
// newest. The real bars of the ETH window cut to none, to one and to two: null arrays, none
// and one are not served and no reading is made over them; two are served and, being quiet
// and flat, hold for the ladder, as the whole window does.
func TestFlatMarketAcceptanceEthereumJune2022TheFloorOfAReadingIsTwoBars(t *testing.T) {
	replayFlatMarket(t)
	trade := ethereumLadder(ethereumOrders)
	full := tapeWindowBlock(t, "ethusdt-1h-2022-06.json", utcAt("2022-06-09 03:00:00"), flatmarket.MinSmcTrend, 0, dashboardRows)
	nulls := aggragates.SmartSellIndicators{SmcTrendDown: full.SmcTrendDown, SmcTrendValid: full.SmcTrendValid}
	for _, c := range []struct {
		name   string
		block  aggragates.SmartSellIndicators
		served bool
	}{
		{"null arrays", nulls, false},
		{"no bars", lastBars(full, 0), false},
		{"one bar", lastBars(full, 1), false},
		{"two bars", lastBars(full, 2), true},
		{"the whole window", full, true},
	} {
		reading := flatMarketReading(trade, c.block, withBlock(c.block))
		if served := flatmarket.BarsOf(c.block).Served(); served != c.served || (reading.Range.Of > 0) != c.served || reading.Holds != c.served {
			t.Errorf("%s: served %v, read over %d bars, holds %v, want all of them %v", c.name, served, reading.Range.Of, reading.Holds, c.served)
		}
	}
}
