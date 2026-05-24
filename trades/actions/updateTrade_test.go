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
	if got.Trade.Logs[0].Message != "NEW_TO_BUY" {
		t.Errorf("log message = %q, want NEW_TO_BUY", got.Trade.Logs[0].Message)
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

func TestUpdateTradeSkipsDuplicateLogs(t *testing.T) {
	var produced []string
	// UpdateTrade builds the message as "<OldPosition>_TO_<PositionType>" before
	// uppercasing, and the dedupe check compares the pre-uppercase form against
	// the last log message. Storing the lowercase form here exercises the
	// duplicate short-circuit.
	event := events.Events{
		Broker: newBrokerWithRecorder(&produced),
		Trade: aggragates.Trades{
			PositionType: "buy",
			Logs: []aggragates.TradesLogs{
				{Message: "new_TO_buy"},
			},
		},
		Params: aggragates.Params{OldPosition: "new"},
	}

	got, err := UpdateTrade(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got.Trade.Logs) != 1 {
		t.Fatalf("expected duplicate log to be skipped, got %d logs", len(got.Trade.Logs))
	}
	if len(produced) != 0 {
		t.Errorf("expected no message produced when duplicate, got %d", len(produced))
	}
}
