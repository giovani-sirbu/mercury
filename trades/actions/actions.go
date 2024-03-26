package actions

import (
	"encoding/json"
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
)

// TODO: action to update trade
func UpdateTrade(event []byte) {
	var eventDetails events.Events
	json.Unmarshal(event, &eventDetails)
	fmt.Println("update trade", eventDetails.Trade.Symbol)
}

// TODO: action to cancel pending order
func CancelPendingOrder(event []byte) {
	var eventDetails events.Events
	json.Unmarshal(event, &eventDetails)
	fmt.Println("cancel order", eventDetails.Trade.Symbol)
}

// TODO: action to to verify if account still have funds
func HasFunds(event []byte) {
	var eventDetails events.Events
	json.Unmarshal(event, &eventDetails)
	fmt.Println("has funds", eventDetails.Trade.Symbol)
}

// TODO: action to create buy order
func Buy(event []byte) {
	var eventDetails events.Events
	json.Unmarshal(event, &eventDetails)
	fmt.Println("Buy order", eventDetails.Trade.Symbol)
}

// TODO: action to create sell order
func Sell(event []byte) {
	var eventDetails events.Events
	json.Unmarshal(event, &eventDetails)
	fmt.Println("Sell order", eventDetails.Trade.Symbol)
}

// TODO: action to create a new trade with same settings
func DuplicateTrade(event []byte) {
	var eventDetails events.Events
	json.Unmarshal(event, &eventDetails)
	fmt.Println("Duplicate trade order", eventDetails.Trade.Symbol)
}
