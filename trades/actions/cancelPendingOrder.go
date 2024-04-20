package actions

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange"
)

func CancelPendingOrder(event events.Events) (events.Events, error) {
	if event.Trade.PendingOrder != 0 {
		exchangeInit := exchange.Exchange{Name: event.Exchange.Name, ApiKey: event.Exchange.ApiKey, ApiSecret: event.Exchange.ApiSecret, TestNet: event.Exchange.TestNet}
		client, clientError := exchangeInit.Client()
		if clientError != nil {
			return events.Events{}, clientError
		}
		_, err := client.CancelOrder(event.Trade.PendingOrder, event.Trade.Symbol)

		if err != nil {
			return events.Events{}, err
		}
		return event, nil
	} else {
		event.Trade.PendingOrder = 0
		return event, nil
	}
}
