package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
)

func CreateFuturesOrders(event events.Events) (events.Events, error) {
	fmt.Println("CreateFuturesOrders")
	return event, nil
}
