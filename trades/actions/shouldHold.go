package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
)

func ShouldHold(event events.Events) (events.Events, error) {
	if !event.Trade.Strategy.Params.Cooldown {
		return event, nil
	}
	if event.Trade.PositionType == "buy" && event.Params.CoolDownIndicators.MarketBearish ||
		(event.Trade.Inverse && event.Trade.PositionType == "sell" && event.Params.CoolDownIndicators.MarketBearish) {
		msg := fmt.Sprintf("Postion %s was hold do the bearish indicator", event.Trade.PositionType)
		err := fmt.Errorf(msg)
		return SaveError(event, err)
	}
	if event.Trade.PositionType == "sell" && event.Params.CoolDownIndicators.MarketBullish ||
		(event.Trade.Inverse && event.Trade.PositionType == "buy" && event.Params.CoolDownIndicators.MarketBullish) {
		msg := fmt.Sprintf("Postion %s was hold do the bullish indicator", event.Trade.PositionType)
		err := fmt.Errorf(msg)
		return SaveError(event, err)
	}
	return event, nil
}
