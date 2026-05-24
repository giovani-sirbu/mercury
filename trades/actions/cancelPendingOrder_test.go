package actions

import (
	"testing"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange"
	exchangeAggregates "github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestCancelPendingOrderReturnsEventUnchangedWhenNoPendingOrder(t *testing.T) {
	event := events.Events{Trade: aggragates.Trades{PendingOrder: 0}}

	got, err := CancelPendingOrder(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Trade.PendingOrder != 0 {
		t.Fatalf("expected PendingOrder to stay 0, got %d", got.Trade.PendingOrder)
	}
}

func TestCancelPendingOrderClearsPendingOrderOnSuccess(t *testing.T) {
	var calledWith int64

	customActions := exchangeAggregates.Actions{
		CancelOrder: func(orderId int64, symbol string) (exchangeAggregates.CancelOrderResponse, *common.APIError) {
			calledWith = orderId
			return exchangeAggregates.CancelOrderResponse{}, nil
		},
	}

	event := events.Events{
		Exchange: exchange.Exchange{IsCustom: true, CustomActions: customActions},
		Trade:    aggragates.Trades{PendingOrder: 42, Symbol: "BTC/USDT"},
	}

	got, err := CancelPendingOrder(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Trade.PendingOrder != 0 {
		t.Fatalf("expected PendingOrder to be cleared, got %d", got.Trade.PendingOrder)
	}
	if calledWith != 42 {
		t.Fatalf("expected CancelOrder called with id 42, got %d", calledWith)
	}
}
