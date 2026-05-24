package tests

import (
	"strings"
	"testing"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange"
	exchangeAggregates "github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// hasFundsCustomActions returns a baseline custom-actions struct that lets
// each test override individual function fields while keeping the rest of
// the happy-path stubs intact.
func hasFundsCustomActions() exchangeAggregates.Actions {
	return exchangeAggregates.Actions{
		GetUserAssets: func() ([]exchangeAggregates.UserAssetRecord, *common.APIError) {
			return []exchangeAggregates.UserAssetRecord{{Asset: "USDC", Free: "1000"}}, nil
		},
		APIKeyPermission: func() (exchangeAggregates.APIKeyPermission, *common.APIError) {
			return exchangeAggregates.APIKeyPermission{EnableSpotAndMarginTrading: true}, nil
		},
		Sell: func(symbol string, qty float64, price string) (exchangeAggregates.CreateOrderResponse, *common.APIError) {
			return exchangeAggregates.CreateOrderResponse{}, nil
		},
	}
}

func hasFundsBaseTrade() aggragates.Trades {
	trade := scenarioBuildTrade("buy", 100000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)
	return trade
}

// TestHasFundsEdge_RejectsWhenSpotMarginTradingDisabled covers the
// permission-check branch inside GetFundsQuantities: if the API key does
// not have spot/margin trading enabled, HasFunds bails out with the
// dedicated error before ever computing balance math.
func TestHasFundsEdge_RejectsWhenSpotMarginTradingDisabled(t *testing.T) {
	customActions := hasFundsCustomActions()
	customActions.APIKeyPermission = func() (exchangeAggregates.APIKeyPermission, *common.APIError) {
		return exchangeAggregates.APIKeyPermission{EnableSpotAndMarginTrading: false}, nil
	}

	event := events.Events{
		Exchange: exchange.Exchange{IsCustom: true, CustomActions: customActions},
		Trade:    hasFundsBaseTrade(),
		Events:   map[string]func(events.Events) (events.Events, error){"updateTrade": EmptyUpdateTrade},
	}

	_, err := actions.HasFunds(event)
	if err == nil {
		t.Fatal("expected HasFunds to reject when spot/margin trading is disabled")
	}
	if !strings.Contains(err.Error(), "Spot & Margin Trading") {
		t.Errorf("expected permission error, got: %v", err)
	}
}

// TestHasFundsEdge_PropagatesSymbolNotWhitelisted2010Error pins the
// short-circuit when the pre-flight Sell ping returns Binance code -2010
// (symbol not whitelisted). GetFundsQuantities returns the APIError
// unchanged so the caller knows the issue is symbol-side, not balance-side.
func TestHasFundsEdge_PropagatesSymbolNotWhitelisted2010Error(t *testing.T) {
	customActions := hasFundsCustomActions()
	customActions.Sell = func(symbol string, qty float64, price string) (exchangeAggregates.CreateOrderResponse, *common.APIError) {
		return exchangeAggregates.CreateOrderResponse{}, &common.APIError{Code: -2010, Message: "symbol not whitelisted"}
	}

	event := events.Events{
		Exchange: exchange.Exchange{IsCustom: true, CustomActions: customActions},
		Trade:    hasFundsBaseTrade(),
		Events:   map[string]func(events.Events) (events.Events, error){"updateTrade": EmptyUpdateTrade},
	}

	_, err := actions.HasFunds(event)
	if err == nil {
		t.Fatal("expected HasFunds to surface the -2010 whitelist error")
	}
}

// TestHasFundsEdge_IgnoresNon2010APIErrorsFromSellPing covers the negative
// branch of the pre-flight Sell: Binance errors that are NOT -2010 (e.g.,
// quantity too small) are not treated as fatal — HasFunds keeps going and
// runs the normal balance check.
func TestHasFundsEdge_IgnoresNon2010APIErrorsFromSellPing(t *testing.T) {
	customActions := hasFundsCustomActions()
	customActions.Sell = func(symbol string, qty float64, price string) (exchangeAggregates.CreateOrderResponse, *common.APIError) {
		return exchangeAggregates.CreateOrderResponse{}, &common.APIError{Code: -1013, Message: "quantity too small"}
	}

	event := events.Events{
		Exchange: exchange.Exchange{IsCustom: true, CustomActions: customActions},
		Trade:    hasFundsBaseTrade(),
		Events:   map[string]func(events.Events) (events.Events, error){"updateTrade": EmptyUpdateTrade},
	}

	got, err := actions.HasFunds(event)
	if err != nil {
		t.Fatalf("HasFunds rejected on non-2010 sell-ping error: %v", err)
	}
	if got.Trade.PositionType != "buy" {
		t.Errorf("PositionType changed unexpectedly to %q", got.Trade.PositionType)
	}
}

// TestHasFundsEdge_InverseUsedAmountReducesAvailableBalance covers the
// spot-side adjustment for trades operating on the same quote asset as an
// active inverse trade: GetFundsQuantities subtracts the reserved inverse
// amount from the remaining balance before comparing against needed qty.
//
// Wallet holds 100 USDC, an inverse position has 90 USDC reserved -> only
// 10 USDC free for the next spot buy. A buy needing ~50 USDC must reject.
func TestHasFundsEdge_InverseUsedAmountReducesAvailableBalance(t *testing.T) {
	customActions := hasFundsCustomActions()
	customActions.GetUserAssets = func() ([]exchangeAggregates.UserAssetRecord, *common.APIError) {
		return []exchangeAggregates.UserAssetRecord{{Asset: "USDC", Free: "100"}}, nil
	}

	trade := scenarioBuildTrade("buy", 100000, false)
	// One prior buy so the multiplier branch runs (needed = lastQty * multiplier * price).
	// Last qty 0.0005 BTC, multiplier 2 -> 0.001 BTC * 100000 = 100 USDC needed.
	scenarioAppendHistory(&trade, "BUY", 0.0005, 100000, "", 0)

	event := events.Events{
		Exchange: exchange.Exchange{IsCustom: true, CustomActions: customActions},
		Trade:    trade,
		Events:   map[string]func(events.Events) (events.Events, error){"updateTrade": EmptyUpdateTrade},
		Params: aggragates.Params{
			InverseUsedAmount: []aggragates.UsedAmountResult{
				{UsedAmount: 90, QuoteCurrency: "USDC"},
			},
		},
	}

	_, err := actions.HasFunds(event)
	if err == nil {
		t.Fatal("expected HasFunds to reject after subtracting InverseUsedAmount")
	}
	if !strings.Contains(err.Error(), "Insufficient funds") {
		t.Errorf("expected insufficient-funds error, got: %v", err)
	}
}

// TestHasFundsEdge_NilCustomActionsFallsThroughIfPermissionMissing checks
// the defensive code: a custom-actions struct with no APIKeyPermission
// field panics because it's invoked without a nil check. Documenting the
// requirement protects future refactors.
func TestHasFundsEdge_NilCustomActionsFallsThroughIfPermissionMissing(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when APIKeyPermission custom-actions field is nil")
		}
	}()

	customActions := exchangeAggregates.Actions{
		GetUserAssets: func() ([]exchangeAggregates.UserAssetRecord, *common.APIError) {
			return []exchangeAggregates.UserAssetRecord{}, nil
		},
		// APIKeyPermission deliberately omitted.
	}

	event := events.Events{
		Exchange: exchange.Exchange{IsCustom: true, CustomActions: customActions},
		Trade:    hasFundsBaseTrade(),
		Events:   map[string]func(events.Events) (events.Events, error){"updateTrade": EmptyUpdateTrade},
	}
	_, _ = actions.HasFunds(event)
}
