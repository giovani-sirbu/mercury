package quantities

import (
	"strings"

	"github.com/giovani-sirbu/mercury/events"
)

// GetGrossQuantities sums the history's buy and sell quantities as stored,
// with no lot-size rounding and no price conversion; Type is compared
// case-insensitively because the producers disagree on case.
func GetGrossQuantities(event events.Events) (buyTotal, sellTotal float64) {
	for _, data := range event.Trade.History {
		if strings.ToLower(data.Type) == "buy" {
			buyTotal += data.Quantity
		} else {
			sellTotal += data.Quantity
		}
	}

	return buyTotal, sellTotal
}
