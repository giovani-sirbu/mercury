package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
)

func CheckOldFuturesOrders(event events.Events) (events.Events, error) {
	fmt.Println("checkOldFuturesOrders")
	return event, nil
}
