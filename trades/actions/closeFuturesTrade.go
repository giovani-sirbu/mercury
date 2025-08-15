package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
)

func CloseFuturesTrade(event events.Events) (events.Events, error) {
	fmt.Println("Close position")
	return event, nil
}
