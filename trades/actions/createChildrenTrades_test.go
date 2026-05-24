package actions

import (
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/messagebroker"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestCreateChildrenTradesShortCircuitsWhenChildrenExist(t *testing.T) {
	event := events.Events{
		ChildrenTrades: []aggragates.Trades{{Symbol: "ETH/USDT"}},
	}

	got, err := CreateChildrenTrades(event)
	if err != nil {
		t.Fatalf("expected no error when children already exist, got %v", err)
	}
	if got.Trade.Symbol != event.Trade.Symbol {
		t.Errorf("expected unchanged event when children exist")
	}
}

func TestCreateChildrenTradesProducesMessageAndReturnsPending(t *testing.T) {
	var topics []string
	broker := messagebroker.BrokerMethods{
		ProduceWithCorrelation: func(topic string, value []byte, correlationID string, producer *messagebroker.Producer) error {
			topics = append(topics, topic)
			return nil
		},
	}

	event := events.Events{
		Broker:        broker,
		CorrelationID: "abc-123",
		Trade:         aggragates.Trades{Symbol: "BTC/USDT"},
	}

	_, err := CreateChildrenTrades(event)
	if err == nil {
		t.Fatal("expected error indicating children not yet created")
	}
	if len(topics) != 1 || topics[0] != "create-children-trades" {
		t.Errorf("expected one message produced on create-children-trades topic, got %v", topics)
	}
}
