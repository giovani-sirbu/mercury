package tests

import (
	"testing"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange"
	exchangeAggregates "github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// buyFailingExchange returns an exchange whose Buy/MarketBuy/Sell/MarketSell
// always returns a Binance APIError. Used to drive the SaveError chain.
func buyFailingExchange() exchange.Exchange {
	apiErr := &common.APIError{Code: -1013, Message: "<APIError> code=-1013, msg=quantity too small"}
	customActions := exchangeAggregates.Actions{
		Buy: func(symbol string, qty float64, price string) (exchangeAggregates.CreateOrderResponse, *common.APIError) {
			return exchangeAggregates.CreateOrderResponse{}, apiErr
		},
		Sell: func(symbol string, qty float64, price string) (exchangeAggregates.CreateOrderResponse, *common.APIError) {
			return exchangeAggregates.CreateOrderResponse{}, apiErr
		},
		MarketBuy: func(symbol string, qty float64) (exchangeAggregates.CreateOrderResponse, *common.APIError) {
			return exchangeAggregates.CreateOrderResponse{}, apiErr
		},
		MarketSell: func(symbol string, qty float64) (exchangeAggregates.CreateOrderResponse, *common.APIError) {
			return exchangeAggregates.CreateOrderResponse{}, apiErr
		},
		GetUserAssets: func() ([]exchangeAggregates.UserAssetRecord, *common.APIError) {
			return []exchangeAggregates.UserAssetRecord{{Asset: "USDC", Free: "1000"}, {Asset: "BTC", Free: "0.01"}}, nil
		},
		APIKeyPermission: func() (exchangeAggregates.APIKeyPermission, *common.APIError) {
			return exchangeAggregates.APIKeyPermission{EnableSpotAndMarginTrading: true}, nil
		},
	}
	return exchange.Exchange{IsCustom: true, CustomActions: customActions}
}

// TestAPIError_BuyRoutesThroughSaveErrorOnFailedSubsequentOrder pins the
// Buy → SaveError chain when the limit-buy call fails. SaveError flips
// Status to Blocked and reverts PositionType/PositionPrice to the prior
// state.
func TestAPIError_BuyRoutesThroughSaveErrorOnFailedSubsequentOrder(t *testing.T) {
	trade := scenarioBuildTrade("buy", 98000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := events.Events{
		Exchange: buyFailingExchange(),
		Trade:    trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": EmptyUpdateTrade,
		},
		Params: aggragates.Params{OldPosition: "active", OldPositionPrice: 100000},
	}

	got, err := actions.Buy(event)
	if err == nil {
		t.Fatal("expected Buy to surface exchange APIError")
	}
	if got.Trade.Status != aggragates.Blocked {
		t.Errorf("Trade.Status = %q, want blocked after APIError", got.Trade.Status)
	}
	if got.Trade.PositionType != "active" {
		t.Errorf("PositionType = %q, want active (reverted by SaveError)", got.Trade.PositionType)
	}
	if len(got.Trade.Logs) != 1 || got.Trade.Logs[0].Type != aggragates.LOG_ERROR {
		t.Errorf("expected one LOG_ERROR entry, got %+v", got.Trade.Logs)
	}
}

// TestAPIError_BuyRoutesThroughSaveErrorOnFailedFirstMarketBuy mirrors the
// prior test for the first-buy (MarketBuy) endpoint. Same failure mode,
// same SaveError side effects.
func TestAPIError_BuyRoutesThroughSaveErrorOnFailedFirstMarketBuy(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)
	trade.StrategyPair.StrategySettings[0].InitialBid = 1 // skip GetUserAssets, hit MarketBuy

	event := events.Events{
		Exchange: buyFailingExchange(),
		Trade:    trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": EmptyUpdateTrade,
		},
		Params: aggragates.Params{OldPosition: "new", OldPositionPrice: 100000},
	}

	got, err := actions.Buy(event)
	if err == nil {
		t.Fatal("expected MarketBuy APIError to surface")
	}
	if got.Trade.Status != aggragates.Blocked {
		t.Errorf("Trade.Status = %q, want blocked", got.Trade.Status)
	}
}

// TestAPIError_SellRoutesThroughSaveErrorOnFailedLimitSell pins the Sell
// failure path. The Sell action sets PendingOrder to the response's ID
// (zero on APIError) and then routes to SaveError.
func TestAPIError_SellRoutesThroughSaveErrorOnFailedLimitSell(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 101000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := events.Events{
		Exchange: buyFailingExchange(),
		Trade:    trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": EmptyUpdateTrade,
		},
		Params: aggragates.Params{OldPosition: "active", OldPositionPrice: 100000},
	}

	got, err := actions.Sell(event)
	if err == nil {
		t.Fatal("expected Sell to surface exchange APIError")
	}
	if got.Trade.Status != aggragates.Blocked {
		t.Errorf("Trade.Status = %q, want blocked after sell APIError", got.Trade.Status)
	}
	if got.Trade.PendingOrder != 0 {
		t.Errorf("PendingOrder = %d, want 0 (response was empty)", got.Trade.PendingOrder)
	}
}

// TestAPIError_SellRoutesThroughSaveErrorOnFailedInverseBuyBack mirrors
// the prior Sell test for an inverse close (which routes to client.Buy
// instead of client.Sell).
func TestAPIError_SellRoutesThroughSaveErrorOnFailedInverseBuyBack(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 99000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)

	event := events.Events{
		Exchange: buyFailingExchange(),
		Trade:    trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": EmptyUpdateTrade,
		},
		Params: aggragates.Params{OldPosition: "active", OldPositionPrice: 100000},
	}

	got, err := actions.Sell(event)
	if err == nil {
		t.Fatal("expected inverse Sell to surface APIError")
	}
	if got.Trade.Status != aggragates.Blocked {
		t.Errorf("Trade.Status = %q, want blocked", got.Trade.Status)
	}
}
