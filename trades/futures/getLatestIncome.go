package futures

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/giovani-sirbu/mercury/events"
)

// GetLatestIncome sums the realized PNL records of the most recently closed
// position: every income record within timeWindow of the newest one.
func GetLatestIncome(event events.Events, timeWindow time.Duration) (float64, error) {
	// Init futures client
	client, clientError := event.Exchange.FuturesClient()
	if clientError != nil {
		return 0, clientError
	}

	// Fetch income history for the symbol
	income, incomeErr := client.GetIncomeHistory(event.Trade.Symbol)
	if incomeErr != nil {
		return 0, incomeErr
	}

	if len(income) == 0 {
		return 0, fmt.Errorf("couldn't fetch income")
	}

	// Sort income records by timestamp (descending, most recent first)
	sort.Slice(income, func(i, j int) bool {
		return income[i].Time > income[j].Time
	})

	// Group records within the same time window for the latest closed position
	var totalPNL float64
	latestTime := time.UnixMilli(income[0].Time) // Convert int64 to time.Time
	for _, record := range income {
		recordTime := time.UnixMilli(record.Time) // Convert int64 to time.Time
		// Only include records within the time window of the latest record
		if latestTime.Sub(recordTime) <= timeWindow {
			pnl, err := strconv.ParseFloat(record.Income, 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse PNL: %v", err)
			}
			totalPNL += pnl
		} else {
			break // Exit once we go beyond the time window
		}
	}

	if totalPNL == 0 {
		return 0, fmt.Errorf("no realized PNL found in the time window")
	}

	return totalPNL, nil
}
