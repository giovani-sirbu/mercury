package testutil

import "github.com/giovani-sirbu/mercury/events"

// NopUpdateTrade stands in for the updateTrade action: it returns the event
// untouched, so a gate test can read the logs a hold appended without a broker.
func NopUpdateTrade(event events.Events) (events.Events, error) {
	return event, nil
}
