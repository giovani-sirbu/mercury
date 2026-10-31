package profit

import (
	"math"
	"testing"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange"
	exchangeAggregates "github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TestCalculateUSDProfit_ShortCircuitsWhenProfitAssetAlreadyUSD pins the
// fast path: when ProfitAsset already contains "USD" (USDT, USDC, BUSD,
// etc.) the function returns trade.Profit verbatim with no price lookup.
func TestCalculateUSDProfit_ShortCircuitsWhenProfitAssetAlreadyUSD(t *testing.T) {
	cases := []struct {
		asset  string
		profit float64
	}{
		{"USDT", 123.45},
		{"USDC", 67.89},
		{"BUSD", 10},
	}
	for _, c := range cases {
		event := events.Events{Trade: aggragates.Trades{ProfitAsset: c.asset, Profit: c.profit}}
		if got := CalculateUSDProfit(event); got != c.profit {
			t.Errorf("asset %s: got %v, want %v", c.asset, got, c.profit)
		}
	}
}

// TestCalculateUSDProfit_ConvertsViaWsPricesSnapshot pins the in-process
// price path. With BTC profit and a populated WsPrices map, no exchange
// call is needed.
func TestCalculateUSDProfit_ConvertsViaWsPricesSnapshot(t *testing.T) {
	event := events.Events{
		Trade:    aggragates.Trades{ProfitAsset: "BTC", Profit: 0.005},
		WsPrices: map[string]float64{"BTC/USDT": 100000},
	}
	const want = 500.0 // 0.005 * 100000
	if got := CalculateUSDProfit(event); math.Abs(got-want) > 1e-9 {
		t.Errorf("CalculateUSDProfit = %v, want %v", got, want)
	}
}

// TestCalculateUSDProfit_FallsBackToCustomExchangeWhenWsPricesMissing pins
// the fallback chain when WsPrices is nil or the symbol is absent.
func TestCalculateUSDProfit_FallsBackToCustomExchangeWhenWsPricesMissing(t *testing.T) {
	customActions := exchangeAggregates.Actions{
		GetPrice: func(symbol string) (float64, *common.APIError) {
			if symbol != "BTC/USDT" {
				return 0, &common.APIError{Message: "wrong symbol"}
			}
			return 100000, nil
		},
	}
	event := events.Events{
		Trade:    aggragates.Trades{ProfitAsset: "BTC", Profit: 0.01},
		Exchange: exchange.Exchange{IsCustom: true, CustomActions: customActions},
	}
	const want = 1000.0
	if got := CalculateUSDProfit(event); math.Abs(got-want) > 1e-9 {
		t.Errorf("CalculateUSDProfit = %v, want %v", got, want)
	}
}

// TestCalculateUSDProfit_ReturnsZeroOnExchangePriceError covers the
// error branch when the fallback exchange call fails.
func TestCalculateUSDProfit_ReturnsZeroOnExchangePriceError(t *testing.T) {
	customActions := exchangeAggregates.Actions{
		GetPrice: func(symbol string) (float64, *common.APIError) {
			return 0, &common.APIError{Message: "boom"}
		},
	}
	event := events.Events{
		Trade:    aggragates.Trades{ProfitAsset: "BTC", Profit: 1},
		Exchange: exchange.Exchange{IsCustom: true, CustomActions: customActions},
	}
	if got := CalculateUSDProfit(event); got != 0 {
		t.Errorf("CalculateUSDProfit = %v, want 0 on price error", got)
	}
}

// TestCalculateUSDProfit_IgnoresZeroWsPriceAndFallsBackToExchange documents
// that a WsPrices entry of 0 is treated as "missing" — the fallback path
// runs. Prevents stale/uninitialised cache entries from poisoning the
// output with a 0 result.
func TestCalculateUSDProfit_IgnoresZeroWsPriceAndFallsBackToExchange(t *testing.T) {
	customActions := exchangeAggregates.Actions{
		GetPrice: func(symbol string) (float64, *common.APIError) {
			return 50000, nil
		},
	}
	event := events.Events{
		Trade:    aggragates.Trades{ProfitAsset: "BTC", Profit: 0.02},
		WsPrices: map[string]float64{"BTC/USDT": 0}, // stale/empty
		Exchange: exchange.Exchange{IsCustom: true, CustomActions: customActions},
	}
	const want = 1000.0 // 0.02 * 50000 from fallback
	if got := CalculateUSDProfit(event); math.Abs(got-want) > 1e-9 {
		t.Errorf("CalculateUSDProfit = %v, want %v", got, want)
	}
}
