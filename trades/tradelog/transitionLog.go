package tradelog

import (
	"strconv"
	"strings"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TransitionLog is the INFO row of the chain's transition from Params.OldPosition to the trade's
// PositionType: its key (aggragates.TransitionKey) and readable text at PositionPrice, unstamped.
func TransitionLog(event events.Events) aggragates.TradesLogs {
	oldPosition, position := event.Params.OldPosition, event.Trade.PositionType
	key := aggragates.TransitionKey(oldPosition, position)
	return aggragates.TradesLogs{
		Percentage: event.Params.Percentage,
		Message:    transitionMessage(key, oldPosition, event.Trade, event.Params.Quantity),
		MessageKey: key,
		Type:       aggragates.LOG_INFO,
		Price:      event.Trade.PositionPrice,
		Quantity:   event.Params.Quantity,
		TradeID:    event.Trade.ID,
	}
}

// transitionMessage is a transition's readable sentence at the trade's price, " (qty <q>)" right after it when quantity is above zero.
func transitionMessage(key, oldPosition string, trade aggragates.Trades, quantity float64) string {
	before, after := transitionWords(key, oldPosition, trade, quantity)
	message := before + " " + formatNumber(trade.PositionPrice)
	if quantity > 0 {
		message += " (qty " + formatNumber(quantity) + ")"
	}
	return message + after + "."
}

// transitionWords is the text of a transition's key around its price: the words before it and after it.
func transitionWords(key, oldPosition string, trade aggragates.Trades, quantity float64) (string, string) {
	switch key {
	case "NEW_TO_BUY":
		return "New position opened: first depth filled at", ""
	case "NEW_TO_SELL":
		return "New short position opened at", ""
	case "_TO_BUY":
		return "Child trade opened: first depth filled at", ""
	case "BUY_TO_BUY":
		return sameSideWords(trade, "long", "Resting buy order cancelled, the next depth is re-anchored at"), ""
	case "SELL_TO_SELL":
		return sameSideWords(trade, "short", "Short position updated at"), ""
	case "BUY_TO_TRAILING_BUY":
		return "Preparing to increase the position size: trailing buy armed at", ""
	case "TRAILING_BUY_TO_TRAILING_BUY":
		return "Secured a better price for the next depth: trailing buy moved to", ""
	case "TRAILING_BUY_TO_BUY":
		if quantity > 0 {
			return "Position size increased: new depth filled at", ""
		}
		return "Trailing buy released without a fill, back to waiting at", ""
	case "BUY_TO_TAKE_PROFIT":
		return "Preparing to close the position: take profit armed at", ""
	case "TAKE_PROFIT_TO_TAKE_PROFIT":
		return "Secured a better closing price: take profit raised to", ""
	case "TAKE_PROFIT_TO_BUY":
		return "Take profit released, the position stays open and waits at", ""
	case "BUY_TO_SELL":
		return "Position closed on your request: sold at", ""
	case "TAKE_LOSS_TO_TAKE_LOSS":
		return "Take loss moved to", ", the position still closes at a loss"
	case "SELL_TO_BUY":
		return "Position reopened after the market price shift, back to buy at", ""
	case "IMPASSE_TO_SELL_PARENT":
		return "Children closed in profit: parent position selling at", ""
	case "SELL_PARENT_TO_SELL_PARENT":
		return "Parent sale price updated to", ""
	case "SELL_PARENT_TO_BUY":
		return "Parent sale released, the position stays open at", ""
	}
	switch trade.PositionType {
	case aggragates.PositionSell:
		return "Position closed: sold at", ""
	case aggragates.PositionTakeLoss:
		return "Position closed at a loss: sold at", ""
	case aggragates.PositionImpasse:
		return "Impasse reached at", ": all depths are filled, a child trade takes over until the position can close"
	}
	if oldPosition == "" {
		return "Position moved to " + stateWords(trade.PositionType) + " at", ""
	}
	return "Position moved from " + stateWords(oldPosition) + " to " + stateWords(trade.PositionType) + " at", ""
}

// sameSideWords is a same-side row's text: a futures close, a futures stop loss moved on the side, else otherwise.
func sameSideWords(trade aggragates.Trades, side, otherwise string) string {
	if trade.Status == aggragates.Closed {
		return "Position closed at"
	}
	if last := len(trade.History) - 1; last >= 0 && trade.History[last].Status == aggragates.HistoryStatusUpdatedStopLoss {
		return "Secured a better price for the " + side + " position: stop loss moved to"
	}
	return otherwise
}

// stateWords is a camelCase state in lower-case words ("trailingBuy" → "trailing buy"); "" stays "".
func stateWords(state string) string {
	return strings.ToLower(strings.ReplaceAll(aggragates.UpperSnake(state), "_", " "))
}

// formatNumber is a price or quantity in its shortest exact decimal form.
func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
