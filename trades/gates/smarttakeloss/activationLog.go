package smarttakeloss

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// ActivationLog is the one row writer the three engines share: it turns a
// Row Apply handed back into the INFO trade-log entry, stamped with the
// engine's own clock (wall time in hermes and live-testing, the simulated
// tick in backtesting). The engines only append it — hermes through the
// chain's updateTrade or publishLogs, backtesting with DB.Create, live-testing
// with SetTradeToStorage. The same writer serves the exit row built from
// ExitMessage. Price is the row's own (the activating fill, or the exit
// level), never trade.PositionPrice.
func ActivationLog(trade aggragates.Trades, row Row, at time.Time) aggragates.TradesLogs {
	return aggragates.TradesLogs{
		TradeID:   trade.ID,
		Message:   row.Message,
		Type:      aggragates.LOG_INFO,
		Price:     row.Price,
		CreatedAt: at,
		UpdatedAt: at,
	}
}
