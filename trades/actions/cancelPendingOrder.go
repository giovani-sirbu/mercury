package actions

import (
	"github.com/giovani-sirbu/mercury/events"
)

func CancelPendingOrder(event events.Events) (events.Events, error) {
	if event.Trade.PendingOrder != 0 {
		client, clientError := event.Exchange.Client()
		if clientError != nil {
			return events.Events{}, clientError
		}
		_, err := client.CancelOrder(event.Trade.PendingOrder, event.Trade.Symbol)

		if err != nil {
			return SaveError(event, err)
		}
		return event, nil
	} else {
		event.Trade.PendingOrder = 0
		return event, nil
	}
}
