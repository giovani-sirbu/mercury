package ladder

import (
	"sort"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func GetLatestQuantityByHistory(history []aggragates.TradesHistory, historyType string) float64 {
	if len(history) == 0 {
		return 0
	}

	// sort older history first
	sort.SliceStable(history, func(i, j int) bool {
		if history[i].Type != historyType {
			return true
		}
		return history[i].Quantity < history[j].Quantity
	})

	return history[len(history)-1].Quantity
}
