package crashguard

import (
	"fmt"
	"sync"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The in-process 5m print bucket per exchange account + symbol, for engines
// that do not inject events.FiveMinOHLC themselves.
var (
	printMu      sync.Mutex
	printBuckets = map[string]events.FiveMinOHLC{}
	printStarts  = map[string]int64{}
)

func lastFilledEntryPrice(trade aggragates.Trades) (float64, bool) {
	entrySide := "BUY"
	if trade.Inverse {
		entrySide = "SELL"
	}
	seen := make(map[int64]float64, len(trade.History))
	order := make([]int64, 0, len(trade.History))
	for index, history := range trade.History {
		if history.Type != entrySide || history.Quantity <= 0 || history.Price <= 1e-12 {
			continue
		}
		orderID := history.OrderId
		if orderID == 0 {
			orderID = -int64(index + 1)
		}
		if _, exists := seen[orderID]; !exists {
			order = append(order, orderID)
		}
		seen[orderID] = history.Price
	}
	if len(order) == 0 {
		return 0, false
	}
	price, ok := seen[order[len(order)-1]]
	return price, ok && price > 1e-12
}

func capitulationDisplaced(trade aggragates.Trades, price, lastFill float64, filled int) bool {
	if price <= 0 || lastFill <= 0 {
		return false
	}
	step := unwidenedGridStep(trade, filled)
	if step <= 0 {
		return false
	}
	threshold := float64(CapitulationDisplacementSteps) * step
	if trade.Inverse {
		return (price-lastFill)/lastFill >= threshold
	}
	return (lastFill-price)/lastFill >= threshold
}

// unwidenedGridStep is the price distance between the last fill and the
// next add: the HELD depth's row (filled−1), the same row the engine's
// percentage/tolerance thresholds use for the pending add.
func unwidenedGridStep(trade aggragates.Trades, filled int) float64 {
	settings := trade.StrategyPair.StrategySettings
	if len(settings) == 0 {
		return 0
	}
	row := ladder.SettingsIndexOrBase(settings, filled-1)
	pct := settings[row].Percentage + settings[row].Tolerance
	if pct <= 0 {
		return 0
	}
	return pct / 100
}

func capitulationReclaim(event events.Events, lastFill float64) bool {
	bucket := event.FiveMinOHLC
	if bucket.Last <= 0 {
		bucket = observeFiveMinPrint(event)
	}
	if bucket.Last <= 0 || bucket.High < bucket.Low {
		return false
	}
	mid := (bucket.High + bucket.Low) / 2
	if event.Trade.Inverse {
		return bucket.High >= lastFill && bucket.Last < mid
	}
	return bucket.Low <= lastFill && bucket.Last > mid
}

func observeFiveMinPrint(event events.Events) events.FiveMinOHLC {
	price := event.Trade.PositionPrice
	if price <= 0 && event.WsPrices != nil {
		price = event.WsPrices[event.Trade.Symbol]
	}
	ts := event.TickMillis()
	if price <= 0 || ts <= 0 || event.Trade.Symbol == "" {
		return events.FiveMinOHLC{}
	}
	start := helpers.FloorMillis(ts, 5)
	// Buckets are scoped by exchange account + symbol: sisyphus runs several
	// backtests in one process (each with its own ExchangeID) and hermes
	// serves several accounts, so a symbol-only key mixed prints across runs
	// and made the reclaim test depend on what else was running.
	key := fmt.Sprintf("%d:%s", event.Trade.ExchangeID, event.Trade.Symbol)
	printMu.Lock()
	defer printMu.Unlock()
	if printStarts[key] != start {
		bar := events.FiveMinOHLC{Open: price, High: price, Low: price, Last: price}
		printStarts[key] = start
		printBuckets[key] = bar
		return bar
	}
	bar := printBuckets[key]
	if price > bar.High {
		bar.High = price
	}
	if price < bar.Low {
		bar.Low = price
	}
	bar.Last = price
	printBuckets[key] = bar
	return bar
}
