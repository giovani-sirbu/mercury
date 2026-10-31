package tradelog

import (
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// transitionOf is the event of a chain moving the trade from old to position at price with quantity filled.
func transitionOf(old, position string, price, quantity float64) events.Events {
	return events.Events{
		Trade:  aggragates.Trades{ID: 7, PositionType: position, PositionPrice: price},
		Params: aggragates.Params{OldPosition: old, Quantity: quantity, Percentage: -1.5},
	}
}

// futuresTransitionOf is a same-side futures event at 98.5: the trade at status, its history rows at statuses in order.
func futuresTransitionOf(position string, status aggragates.Status, quantity float64, statuses ...string) events.Events {
	event := transitionOf(position, position, 98.5, quantity)
	event.Trade.Status = status
	for _, historyStatus := range statuses {
		event.Trade.History = append(event.Trade.History, aggragates.TradesHistory{Status: historyStatus})
	}
	return event
}

// assertInfoRow fails unless row is the unstamped INFO row of trade 7 at 98.5 with quantity.
func assertInfoRow(t *testing.T, label string, row aggragates.TradesLogs, quantity float64) {
	t.Helper()
	if row.Type != aggragates.LOG_INFO || row.Price != 98.5 || row.Quantity != quantity || row.TradeID != 7 || row.Percentage != -1.5 || !row.CreatedAt.IsZero() {
		t.Errorf("%s: the row is an unstamped INFO row at the price, got %+v", label, row)
	}
}

// Every transition's row carries its key and a readable sentence at the price, the quantity right
// after it when one filled; a fill out of the trailing buy is the new depth, without one its release.
func TestTransitionLogWordsEveryTransition(t *testing.T) {
	for _, c := range []struct {
		old, position string
		quantity      float64
		key, message  string
	}{
		{"new", "buy", 0, "NEW_TO_BUY", "New position opened: first depth filled at 98.5."},
		{"new", "buy", 2.5, "NEW_TO_BUY", "New position opened: first depth filled at 98.5 (qty 2.5)."},
		{"new", "sell", 2.5, "NEW_TO_SELL", "New short position opened at 98.5 (qty 2.5)."},
		{"", "buy", 3, "_TO_BUY", "Child trade opened: first depth filled at 98.5 (qty 3)."},
		{"buy", "buy", 0, "BUY_TO_BUY", "Resting buy order cancelled, the next depth is re-anchored at 98.5."},
		{"sell", "sell", 0, "SELL_TO_SELL", "Short position updated at 98.5."},
		{"buy", "trailingBuy", 0, "BUY_TO_TRAILING_BUY", "Preparing to increase the position size: trailing buy armed at 98.5."},
		{"trailingBuy", "trailingBuy", 0, "TRAILING_BUY_TO_TRAILING_BUY", "Secured a better price for the next depth: trailing buy moved to 98.5."},
		{"trailingBuy", "buy", 3, "TRAILING_BUY_TO_BUY", "Position size increased: new depth filled at 98.5 (qty 3)."},
		{"trailingBuy", "buy", 0, "TRAILING_BUY_TO_BUY", "Trailing buy released without a fill, back to waiting at 98.5."},
		{"buy", "takeProfit", 0, "BUY_TO_TAKE_PROFIT", "Preparing to close the position: take profit armed at 98.5."},
		{"takeProfit", "takeProfit", 0, "TAKE_PROFIT_TO_TAKE_PROFIT", "Secured a better closing price: take profit raised to 98.5."},
		{"takeProfit", "buy", 0, "TAKE_PROFIT_TO_BUY", "Take profit released, the position stays open and waits at 98.5."},
		{"buy", "sell", 4, "BUY_TO_SELL", "Position closed on your request: sold at 98.5 (qty 4)."},
		{"takeProfit", "sell", 4, "TAKE_PROFIT_TO_SELL", "Position closed: sold at 98.5 (qty 4)."},
		{"trailingBuy", "sell", 4, "TRAILING_BUY_TO_SELL", "Position closed: sold at 98.5 (qty 4)."},
		{"sellParent", "sell", 4, "SELL_PARENT_TO_SELL", "Position closed: sold at 98.5 (qty 4)."},
		{"takeLoss", "sell", 4, "TAKE_LOSS_TO_SELL", "Position closed: sold at 98.5 (qty 4)."},
		{"takeLoss", "takeLoss", 3, "TAKE_LOSS_TO_TAKE_LOSS", "Take loss moved to 98.5 (qty 3), the position still closes at a loss."},
		{"takeLoss", "takeLoss", 0, "TAKE_LOSS_TO_TAKE_LOSS", "Take loss moved to 98.5, the position still closes at a loss."},
		{"takeProfit", "takeLoss", 4, "TAKE_PROFIT_TO_TAKE_LOSS", "Position closed at a loss: sold at 98.5 (qty 4)."},
		{"trailingBuy", "takeLoss", 4, "TRAILING_BUY_TO_TAKE_LOSS", "Position closed at a loss: sold at 98.5 (qty 4)."},
		{"buy", "takeLoss", 4, "BUY_TO_TAKE_LOSS", "Position closed at a loss: sold at 98.5 (qty 4)."},
		{"sell", "buy", 0, "SELL_TO_BUY", "Position reopened after the market price shift, back to buy at 98.5."},
		{"trailingBuy", "impasse", 0, "TRAILING_BUY_TO_IMPASSE", "Impasse reached at 98.5: all depths are filled, a child trade takes over until the position can close."},
		{"impasse", "sellParent", 0, "IMPASSE_TO_SELL_PARENT", "Children closed in profit: parent position selling at 98.5."},
		{"sellParent", "sellParent", 0, "SELL_PARENT_TO_SELL_PARENT", "Parent sale price updated to 98.5."},
		{"sellParent", "buy", 0, "SELL_PARENT_TO_BUY", "Parent sale released, the position stays open at 98.5."},
		{"buy", "forceTrailingBuy", 0, "BUY_TO_FORCE_TRAILING_BUY", "Position moved from buy to force trailing buy at 98.5."},
		{"", "trailingBuy", 0, "_TO_TRAILING_BUY", "Position moved to trailing buy at 98.5."},
	} {
		row := TransitionLog(transitionOf(c.old, c.position, 98.5, c.quantity))
		if row.MessageKey != c.key || row.Message != c.message {
			t.Errorf("%s → %s: row %q keyed %q, want %q keyed %q", c.old, c.position, row.Message, row.MessageKey, c.message, c.key)
		}
		assertInfoRow(t, c.old+" → "+c.position, row, c.quantity)
	}
}

// A futures same-side row reads the close when the trade closed, else the stop loss moved when the
// last history row is a stop-loss update, else the plain same-side text; the key never changes.
func TestTransitionLogWordsTheFuturesSameSideRows(t *testing.T) {
	for _, c := range []struct {
		name, position string
		status         aggragates.Status
		quantity       float64
		statuses       []string
		key, message   string
	}{
		{"closed long", "buy", aggragates.Closed, 3, nil, "BUY_TO_BUY", "Position closed at 98.5 (qty 3)."},
		{"closed short", "sell", aggragates.Closed, 3, nil, "SELL_TO_SELL", "Position closed at 98.5 (qty 3)."},
		{"closed after a stop-loss update", "buy", aggragates.Closed, 3, []string{aggragates.HistoryStatusUpdatedStopLoss}, "BUY_TO_BUY", "Position closed at 98.5 (qty 3)."},
		{"long stop loss moved", "buy", aggragates.Active, 0, []string{"FILLED", aggragates.HistoryStatusUpdatedStopLoss}, "BUY_TO_BUY", "Secured a better price for the long position: stop loss moved to 98.5."},
		{"short stop loss moved", "sell", aggragates.Active, 0, []string{"FILLED", aggragates.HistoryStatusUpdatedStopLoss}, "SELL_TO_SELL", "Secured a better price for the short position: stop loss moved to 98.5."},
		{"stop-loss update not last", "sell", aggragates.Active, 0, []string{aggragates.HistoryStatusUpdatedStopLoss, "FILLED"}, "SELL_TO_SELL", "Short position updated at 98.5."},
	} {
		row := TransitionLog(futuresTransitionOf(c.position, c.status, c.quantity, c.statuses...))
		if row.MessageKey != c.key || row.Message != c.message {
			t.Errorf("%s: row %q keyed %q, want %q keyed %q", c.name, row.Message, row.MessageKey, c.message, c.key)
		}
		assertInfoRow(t, c.name, row, c.quantity)
	}
}

// The price and quantity print in their shortest exact form, never rounded to a fixed width.
func TestTransitionLogPrintsTheShortestExactNumbers(t *testing.T) {
	if got := TransitionLog(transitionOf("buy", "trailingBuy", 0.000012345, 0)).Message; got != "Preparing to increase the position size: trailing buy armed at 0.000012345." {
		t.Errorf("small price: %q", got)
	}
	if got := TransitionLog(transitionOf("new", "buy", 64250, 0.0015)).Message; got != "New position opened: first depth filled at 64250 (qty 0.0015)." {
		t.Errorf("whole price and a small quantity: %q", got)
	}
}
