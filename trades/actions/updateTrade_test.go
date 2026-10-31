package actions

import (
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/messagebroker"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func newBrokerWithRecorder(record *[]string) messagebroker.BrokerMethods {
	return messagebroker.BrokerMethods{
		ProduceWithCorrelation: func(topic string, value []byte, correlationID string, producer *messagebroker.Producer) error {
			*record = append(*record, topic)
			return nil
		},
	}
}

func TestUpdateTradeAppendsInfoLogAndProducesMessage(t *testing.T) {
	var produced []string
	event := events.Events{
		Broker: newBrokerWithRecorder(&produced),
		Trade: aggragates.Trades{
			PositionType:  "buy",
			PositionPrice: 100,
		},
		Params: aggragates.Params{OldPosition: "new", Quantity: 1},
	}

	got, err := UpdateTrade(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got.Trade.Logs) != 1 {
		t.Fatalf("expected one log appended, got %d", len(got.Trade.Logs))
	}
	if got.Trade.Logs[0].Message != "New position opened: first depth filled at 100 (qty 1)." || got.Trade.Logs[0].MessageKey != "NEW_TO_BUY" {
		t.Errorf("log row = %q keyed %q, want the readable entry keyed NEW_TO_BUY", got.Trade.Logs[0].Message, got.Trade.Logs[0].MessageKey)
	}
	if got.Trade.Logs[0].Type != aggragates.LOG_INFO {
		t.Errorf("log type = %q, want INFO", got.Trade.Logs[0].Type)
	}
	if len(produced) != 1 || produced[0] != "update-trade" {
		t.Errorf("expected one message produced on update-trade topic, got %v", produced)
	}
}

func TestUpdateTradeSkipsLogWhenPreventInfoLog(t *testing.T) {
	var produced []string
	event := events.Events{
		Broker: newBrokerWithRecorder(&produced),
		Trade:  aggragates.Trades{PositionType: "buy"},
		Params: aggragates.Params{OldPosition: "new", PreventInfoLog: true},
	}

	got, err := UpdateTrade(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got.Trade.Logs) != 0 {
		t.Fatalf("expected no logs appended, got %d", len(got.Trade.Logs))
	}
	if len(produced) != 1 {
		t.Errorf("expected message still produced, got %d", len(produced))
	}
}

// A transition the trade's newest row already names is written again and published: every chain
// that runs reaches agora, the trailing buy moved twice in a row included.
func TestUpdateTradeWritesARepeatedTransition(t *testing.T) {
	var produced []string
	event := events.Events{
		Broker: newBrokerWithRecorder(&produced),
		Trade: aggragates.Trades{
			PositionType:  "trailingBuy",
			PositionPrice: 97.5,
			Logs: []aggragates.TradesLogs{
				{Message: "Secured a better price for the next depth: trailing buy moved to 98.", MessageKey: "TRAILING_BUY_TO_TRAILING_BUY"},
			},
		},
		Params: aggragates.Params{OldPosition: "trailingBuy"},
	}

	got, err := UpdateTrade(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got.Trade.Logs) != 2 || got.Trade.Logs[1].MessageKey != "TRAILING_BUY_TO_TRAILING_BUY" || got.Trade.Logs[1].Message != "Secured a better price for the next depth: trailing buy moved to 97.5." {
		t.Fatalf("expected the repeated transition written again, got %+v", got.Trade.Logs)
	}
	if len(produced) != 1 {
		t.Errorf("expected the trade published, got %d", len(produced))
	}
}
