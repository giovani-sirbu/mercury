package actions

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/tradelog"
)

func CancelPendingOrder(event events.Events) (events.Events, error) {
	if event.Trade.PendingOrder != 0 {
		client, clientError := event.Exchange.Client()
		if clientError != nil {
			return events.Events{}, clientError
		}
		_, err := client.CancelOrder(event.Trade.PendingOrder, event.Trade.Symbol)

		if err != nil {
			return tradelog.SaveError(event, err)
		}
		event.Trade.PendingOrder = 0
		return event, nil
	} else {
		return event, nil
	}
}
