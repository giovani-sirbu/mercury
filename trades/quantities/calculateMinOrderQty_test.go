package quantities

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestCalculateMinOrderQtyReturnsZeroWhenFiltersMissing(t *testing.T) {
	trade := aggragates.Trades{PositionPrice: 100}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{}

	if got := CalculateMinOrderQty(trade); got != 0 {
		t.Errorf("expected 0 when MinNotional / LotSize are zero, got %v", got)
	}
}

func TestCalculateMinOrderQtyReturnsZeroWhenPositionPriceIsZero(t *testing.T) {
	trade := aggragates.Trades{PositionPrice: 0}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{MinNotional: 5, LotSize: 5}

	if got := CalculateMinOrderQty(trade); got != 0 {
		t.Errorf("expected 0 when PositionPrice is zero, got %v", got)
	}
}

func TestCalculateMinOrderQtySpotDividesMinNotionalByPriceAndAddsLotStep(t *testing.T) {
	trade := aggragates.Trades{PositionPrice: 100}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{MinNotional: 10, LotSize: 2}

	got := CalculateMinOrderQty(trade)
	// quantity = 10 / 100 = 0.1, + 10^-2 = 0.11, ToFixed(0.11, 2) = 0.11
	if math.Abs(got-0.11) > 1e-9 {
		t.Errorf("CalculateMinOrderQty spot = %v, want 0.11", got)
	}
}

func TestCalculateMinOrderQtyInverseDividesMinNotionalByPriceAndRoundsToLotSize(t *testing.T) {
	trade := aggragates.Trades{PositionPrice: 100, Inverse: true}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{MinNotional: 10.123, LotSize: 2}

	got := CalculateMinOrderQty(trade)
	// quantity = 10.123 / 100 = 0.10123, ToFixed(0.10123, 2) = 0.10
	if math.Abs(got-0.10) > 1e-9 {
		t.Errorf("CalculateMinOrderQty inverse = %v, want 0.10", got)
	}
}
