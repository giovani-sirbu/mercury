package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
)

func ShouldHold(event events.Events) (events.Events, error) {
	if !event.Trade.Strategy.Params.Cooldown {
		return event, nil
	}
	if event.Trade.PositionType == "buy" && event.Params.CoolDownIndicators.RiskScore > 30 {
		msg := fmt.Sprintf("Postion %s was hold do the high risk score %f", event.Trade.PositionType, event.Params.CoolDownIndicators.RiskScore)
		err := fmt.Errorf(msg)
		return SaveError(event, err)
	}
	if event.Trade.PositionType == "sell" && !event.Params.CoolDownIndicators.ShouldTakeProfit {
		msg := fmt.Sprintf("Postion %s was hold do should take profit indicator %t", event.Trade.PositionType, event.Params.CoolDownIndicators.ShouldTakeProfit)
		err := fmt.Errorf(msg)
		return SaveError(event, err)
	}
	return event, nil
}
