package smarttakeloss

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
	"github.com/giovani-sirbu/mercury/trades/profit"
)

// The trail on a real tape: backtest 251, trade 147110, BTC/USDT, the bars Binance served
// for the minutes after the stopLoss was activated. The trade had four fills and a break
// even of about 39850; the flat market gate had triggered at 23:00 on 2024-01-23 and held
// its sale (the loss was over the cap), and at 23:26:30 the price bounced to 39730.29, the
// loss came within the cap and the gate activated the stopLoss. The next print, in the same
// millisecond, hit the committed sale as it stood then ("sell at flat market 39730.58"),
// booked at a loss of about half a percent; BTC then rose over break even within the hour
// and over the next hours. Under the trail that print HOLDS, the rises over the trailing
// take profit re-anchor the ladder and the sale comes in the 00:55 bar, under the tolerance
// of the last anchor, at a gain.
//
// testdata/btcusdt-1m-2024-01-23.json is the klines array as the exchange serves it
// ([open time in ms, open, high, low, close, ...]), the 125 one-minute bars from 23:26 UTC
// on 2024-01-23 to 01:30 UTC on 2024-01-24, copied bar for bar. The tape has no ticks: a
// bar is walked as its low and then its high, the fall checked before the rise, which is
// the order a collapse is the worse for the ladder in.

const tapeFile = "testdata/btcusdt-1m-2024-01-23.json"

// tapeLossCapPct is the cap backtest 251 ran at: between the loss of the print the
// episode opened on and held its sale (about 1.16% of the used amount) and the loss of
// the print it activated on (about 0.30%). The replay pins it there whatever
// SellLossTriggerPercentage is tuned to, because the cap only places the activation at its
// print: the trail the test is about asks no trigger once the stopLoss is activated.
const tapeLossCapPct = 1.0

// tapeBar is one bar of the tape: when it opened and the extremes the walk visits.
type tapeBar struct {
	opened    time.Time
	high, low float64
}

// loadTape reads the bars of the tape, and fails unless they are consecutive minutes
// starting at the minute the activation was made in.
func loadTape(t *testing.T) []tapeBar {
	t.Helper()
	raw, err := os.ReadFile(tapeFile)
	if err != nil {
		t.Fatalf("reading the tape: %v", err)
	}
	var klines [][]any
	if err := json.Unmarshal(raw, &klines); err != nil {
		t.Fatalf("decoding the tape: %v", err)
	}
	number := func(kline []any, index int) float64 {
		switch value := kline[index].(type) {
		case float64:
			return value
		case string:
			parsed, err := strconv.ParseFloat(value, 64)
			if err != nil {
				t.Fatalf("a field of the tape is no number: %v", err)
			}
			return parsed
		}
		t.Fatalf("a field of the tape is neither a number nor a string: %v", kline[index])
		return 0
	}
	bars := make([]tapeBar, len(klines))
	for index, kline := range klines {
		bars[index] = tapeBar{opened: time.UnixMilli(int64(number(kline, 0))).UTC(), high: number(kline, 2), low: number(kline, 3)}
		if index > 0 && bars[index].opened.Sub(bars[index-1].opened) != time.Minute {
			t.Fatalf("the tape has a gap before the bar of %v", bars[index].opened)
		}
	}
	if len(bars) == 0 || !bars[0].opened.Equal(time.Date(2024, time.January, 23, 23, 26, 0, 0, time.UTC)) {
		t.Fatalf("the tape must start at the minute of the activation, got %v bars from %v", len(bars), bars[0].opened)
	}
	return bars
}

// trade147110 is trade 147110 as the activation found it: its four fills, at the prices
// and quantities the exchange filled, and the pair's row (nine depths, a percentage of
// 1.8, a tolerance of 0.2, a trailing take profit of 0.6, a multiplier of two) as the
// activation read it. The live trade paid commissions; this one pays none, which only moves
// the loss the rows name.
func trade147110(t *testing.T) aggragates.Trades {
	t.Helper()
	trade := flatMarketLadder(false)
	trade.Symbol = "BTC/USDT"
	trade.History = nil
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 5, PriceFilter: 2}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 1.8, Tolerance: 0.2, TrailingTakeProfit: 0.6, Multiplier: 2, Depths: 9}}
	for index, fill := range []struct {
		quantity, price float64
		at              time.Time
	}{
		{0.00361, 41667.06, time.Date(2024, time.January, 19, 19, 6, 0, 0, time.UTC)},
		{0.00722, 40852, time.Date(2024, time.January, 22, 5, 4, 0, 0, time.UTC)},
		{0.01444, 40053.72, time.Date(2024, time.January, 22, 19, 17, 0, 0, time.UTC)},
		{0.02888, 39270.4, time.Date(2024, time.January, 23, 9, 16, 0, 0, time.UTC)},
	} {
		trade.History = append(trade.History, aggragates.TradesHistory{Type: "BUY", Quantity: fill.quantity, Price: fill.price, OrderId: int64(index + 1), CreatedAt: fill.at})
		trade.PositionPrice = fill.price
	}
	if breakEven := ladder.AverageEntryPrice(trade); math.Abs(breakEven-39849.94) > 0.01 || ladder.ConfiguredDepths(trade) != 9 || ladder.CountFilledEntries(trade) != 4 {
		t.Fatalf("fixture drifted: four fills of nine depths breaking even at 39849.94, got %v", breakEven)
	}
	return trade
}

// bookedProfit is what a sale of the whole position at price books, the net profit the
// close would assign to the trade, by the figure the loss cap reads (negative for a loss).
func bookedProfit(trade aggragates.Trades, price float64) float64 {
	sale := trade
	sale.PositionType, sale.PositionPrice = "sellLoss", price
	return profit.SimulatedCloseProfit(events.Events{Trade: sale})
}

// tapeStep is one decision of the walk: when, at which print, and what the trail did.
type tapeStep struct {
	at    time.Time
	price float64
	step  saleStep
}

// walkTape walks the bars from the one after the activation's, each as its low and then
// its high, handing every print to Apply (the proposal the ladder makes is position, the
// reading is ai) and writing back what the engines write, until the trail sells. It
// returns the re-anchors and the sale in order, the number of prints held between them,
// and the trade as the last tick left it. Every print that is not a re-anchor or the
// sale must be a hold.
func walkTape(t *testing.T, trade aggragates.Trades, bars []tapeBar, position string, ai aggragates.AIIndicators) (steps []tapeStep, held int, last aggragates.Trades) {
	t.Helper()
	for _, bar := range bars[1:] {
		for _, price := range []float64{bar.low, bar.high} {
			var got Result
			trade, got = engineStep(trade, position, price, bar.opened, ai)
			switch {
			case got.Reason != "":
				assertSold(t, got, reasonFlatMarket)
				return append(steps, tapeStep{bar.opened, price, stepSell}), held, trade
			case got.Activated:
				assertReanchored(t, got)
				steps = append(steps, tapeStep{bar.opened, price, stepReanchor})
			default:
				assertTrailHeld(t, got)
				held++
			}
		}
	}
	t.Fatalf("the trail never sold within the tape, got %v after %d prints held", steps, held)
	return nil, 0, trade
}

// The replay of trade 147110. The tick that activated the stopLoss is driven through
// Apply like the one before it (the over-the-cap tick that opened the episode and held
// the sale), the print after it holds, and the walk of the bars re-anchors at the high of
// the 00:43 bar and at the high of the 00:52 bar and sells in the 00:55 bar, under the
// tolerance of the second anchor, at a price over break even: the booked result is a gain
// where the sale of the next-tick rule booked a loss. The walk is the same whatever the
// ladder proposes and whether the market still reads or not, and the episode writes no row
// from the activation on. It is the same with the flat market gate alone and with every
// decline switched on beside it, none of which has a reading to act on over this ladder.
//
// The cap is the one the tape was measured at (tapeLossCapPct), not the shipped one, so a
// retune of SellLossTriggerPercentage cannot move the activation off its print. The depth bound
// and the floor are the shipped ones, which the trade's four fills of nine depths must
// pass: a retune that leaves it no activation skips the replay, naming them.
func TestTheTrailOnTheTapeOfTrade147110HoldsReanchorsTwiceAndSellsAtAGain(t *testing.T) {
	for name, switches := range map[string][5]bool{
		"the flat market gate alone": {},
		"every decline switched on":  {true, true, true, true, true},
	} {
		t.Run(name, func(t *testing.T) {
			filled, ceiling := ladder.CountFilledEntries(trade147110(t)), ladder.ConfiguredDepths(trade147110(t))
			if (SellLossIgnoreLastDepths > 0 && filled > ceiling-SellLossIgnoreLastDepths) || (SellLossStartAtDepth > 0 && filled < SellLossStartAtDepth) {
				t.Skipf("SellLossIgnoreLastDepths or SellLossStartAtDepth leaves the trade's %d fills of %d depths no activation, which the tape was measured at", filled, ceiling)
			}
			flatMarketAlone(t)
			setGridSwitches(t, switches)
			withEarlyTakeProfitFlatMarket(t, false)
			withAllowedLoss(t, tapeLossCapPct)
			withSellLossIgnoreLastDepths(t, SellLossIgnoreLastDepths)
			withSellLossStartAtDepth(t, SellLossStartAtDepth)
			replayTrade147110(t)
		})
	}
}

// replayTrade147110 is the replay itself, under the switches the caller set.
func replayTrade147110(t *testing.T) {
	t.Helper()
	bars := loadTape(t)
	trade := trade147110(t)
	fresh := trade
	ai, gone := flatMarketMarket(trade), withBlock(aggragates.SmartTakeLossIndicators{})
	fill, breakEven := trade.PositionPrice, ladder.AverageEntryPrice(trade)
	row := trailRow(trade)

	// 23:00: the loss is over the cap, the episode opens and the sale is held.
	const overTheCap, activation, nextPrint = 39387.70, 39730.29, 39730.58
	trade, got := engineStep(trade, "", overTheCap, bars[0].opened.Add(-26*time.Minute), ai)
	assertWaits(t, got, "", triggeredBy(fresh, ai), FlatMarketSaleHeldRow(fill, -handLossPct(fresh, overTheCap)))

	// 23:26:30: the loss is within the cap and the stopLoss is activated, anchored at the print.
	trade, got = engineStep(trade, "", activation, bars[0].opened, ai)
	assertActivated(t, got, FlatMarketStopLossActivatedRow(fill, -handLossPct(fresh, activation)))
	if trade.PositionType != "stopLoss" || trade.PositionPrice != activation {
		t.Fatalf("the activation leaves the ladder in stopLoss anchored at the print, got %q at %v", trade.PositionType, trade.PositionPrice)
	}
	if loss := handLossPct(fresh, activation); loss <= 0 || loss > tapeLossCapPct {
		t.Fatalf("fixture drifted: the activation is made on a real loss within the cap, got %v", loss)
	}

	// The next print, 0.0007% over the anchor, is the row the user saw sold: it holds, whatever
	// the ladder proposes and whether the market reads, and it is no exit. The sale of the old
	// rule would have booked a loss there.
	if move := moveAgainst(nextPrint, activation); move <= 0 || move >= 0.001 {
		t.Fatalf("fixture drifted: the print after the activation is a hair over the anchor, got a move of %v", move)
	}
	for _, position := range addSideProposals {
		for _, reading := range []aggragates.AIIndicators{ai, gone} {
			assertTrailHeld(t, Apply(trade, position, nextPrint, reading))
		}
	}
	if ExitReached(trade, nextPrint, ai.SmartTakeLoss) || ExitReached(trade, nextPrint, aggragates.SmartTakeLossIndicators{}) {
		t.Error("the print after the activation is no exit")
	}
	if net := bookedProfit(trade, nextPrint); net >= 0 {
		t.Errorf("fixture drifted: the sale of the next-tick rule books a loss at the print after the activation, got %v", net)
	}
	rows := len(trade.Logs)

	want := []tapeStep{
		{time.Date(2024, time.January, 24, 0, 43, 0, 0, time.UTC), 39993.23, stepReanchor},
		{time.Date(2024, time.January, 24, 0, 52, 0, 0, time.UTC), 40234.95, stepReanchor},
		{time.Date(2024, time.January, 24, 0, 55, 0, 0, time.UTC), 40153.40, stepSell},
	}
	for _, position := range addSideProposals {
		for name, reading := range map[string]aggragates.AIIndicators{"the market served": ai, "none served": gone} {
			steps, held, last := walkTape(t, trade, bars, position, reading)
			if len(steps) != len(want) {
				t.Fatalf("%q, %s: the trail did %v, want %v", position, name, steps, want)
			}
			for index, step := range steps {
				if !step.at.Equal(want[index].at) || step.price != want[index].price || step.step != want[index].step {
					t.Fatalf("%q, %s: step %d is %v, want %v", position, name, index, step, want[index])
				}
			}
			if held == 0 || len(last.Logs) != rows {
				t.Errorf("%q, %s: the walk holds between its steps and writes no row, got %d prints held and %d rows after %d", position, name, held, len(last.Logs), rows)
			}
			if first, second := steps[0].price, steps[1].price; last.PositionPrice != second || moveAgainst(first, activation) <= row.TrailingTakeProfit || moveAgainst(second, first) <= row.TrailingTakeProfit {
				t.Errorf("%q, %s: each re-anchor is over the trailing take profit of the anchor before it, and the ladder ends anchored at the second, got %v", position, name, last.PositionPrice)
			}
		}
	}

	// The sale is under the tolerance of the last anchor, over break even, and books a gain.
	second, sale := want[1].price, want[2].price
	bound := priceAtMove(trade, second, -row.Tolerance)
	if !(sale < bound) || !(sale > breakEven) {
		t.Errorf("the sale at %v must sit under %v, the tolerance of the last anchor, and over break even %v", sale, bound, breakEven)
	}
	if net := bookedProfit(trade, sale); net <= 0 {
		t.Errorf("the sale books a gain, got %v", net)
	} else {
		t.Logf("the trail books %.4f at %v; the sale at the print after the activation booked %.4f", net, sale, bookedProfit(trade, nextPrint))
	}

	// ExitReached agrees at the end of the walk: an exit under the last anchor's tolerance and none
	// at the print before it. The state is rebuilt from the trade the walk left.
	_, _, last := walkTape(t, trade, bars, "", gone)
	if !ExitReached(last, sale, gone.SmartTakeLoss) || ExitReached(last, bound*(1+1e-9), gone.SmartTakeLoss) {
		t.Errorf("ExitReached must read true at %v and false just over %v after the last re-anchor", sale, bound)
	}
	if state := rebuildState(last); !state.flatMarketActive || !state.flatMarketActivated || state.flatMarketFrom != fill {
		t.Errorf("the episode stands activated at the newest fill through the walk, got %+v", state)
	}
}
