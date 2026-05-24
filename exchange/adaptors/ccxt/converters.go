package ccxt

import (
	"strconv"
	"strings"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

// Most CCXT fields are pointers because CCXT's JSON deserialisation must
// distinguish "field absent" from "field present and zero". Mercury's existing
// aggregates use value types, so every conversion goes through these tiny
// helpers — keeps the call sites readable and centralises the nil handling.

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefFloat(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func derefBool(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}

// formatFloat renders a float as a decimal string with no scientific notation.
// Mercury's aggregates carry prices/quantities as strings (Binance API style),
// so every CCXT float field gets formatted at the adaptor boundary. The -1
// precision tells strconv to use the shortest representation that round-trips
// cleanly — matches what go-binance returns over the wire.
func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// parseOrderID interprets CCXT's stringly-typed order id as a Binance-style
// int64. Crypto.com also exposes int-shaped order ids over its v1 API. If the
// id is empty or non-numeric we return zero — callers downstream already
// handle PendingOrder == 0 as "no pending order" and the tests for non-zero
// happen on the platform side via DB state. Sliding a parse error through
// silently keeps the boundary simple; the original id is still recoverable
// via the upstream CCXT Order.Info map if a caller ever needs it.
func parseOrderID(p *string) int64 {
	if p == nil {
		return 0
	}
	v, _ := strconv.ParseInt(*p, 10, 64)
	return v
}

// normalizeOrderStatus translates CCXT's unified status vocabulary
// ("open"/"closed"/"canceled"/etc.) into the Binance-style strings that
// mercury's downstream handlers (cron's HandlePendingOrder, hermes's strategy
// checks, agora's update-trade consumer) already key on. Mercury was written
// against the Binance REST surface, so it expects "NEW", "PARTIALLY_FILLED",
// "FILLED", "CANCELED", "REJECTED", "EXPIRED" — uppercase. Without this
// mapping, the cron's CCXT path saw "CLOSED" (CCXT's "closed" uppercased),
// matched none of the dispatch branches in HandlePendingOrder, and left the
// trade with pending_order != 0 forever — that was the symptom in the recent
// agora logs.
//
// The amount/filled split for "open" lets us distinguish "NEW" (nothing
// filled yet) from "PARTIALLY_FILLED" (some volume traded but order is still
// resting on the book). CCXT's typed Order does not surface a separate
// status for partials, so we synthesise it here.
func normalizeOrderStatus(ccxtStatus string, amount, filled float64) string {
	switch strings.ToLower(ccxtStatus) {
	case "open":
		if filled > 0 && (amount == 0 || filled < amount) {
			return "PARTIALLY_FILLED"
		}
		return "NEW"
	case "closed":
		// CCXT collapses "fully filled" and "closed by the exchange" into
		// the same bucket. If nothing actually filled we treat it as an
		// EXPIRED/CANCELED-equivalent so HandlePendingOrder takes the
		// "clear pending_order" branch instead of trying to record a zero
		// fill in trades_histories.
		if filled <= 0 {
			return "EXPIRED"
		}
		return "FILLED"
	case "canceled", "cancelled":
		return "CANCELED"
	case "rejected":
		return "REJECTED"
	case "expired":
		return "EXPIRED"
	default:
		return strings.ToUpper(ccxtStatus)
	}
}

// fromCCXTOrder maps CCXT's pointer-heavy Order shape onto mercury's
// CreateOrderResponse. Used by Buy/Sell/MarketBuy/MarketSell — the callers
// upstream copy `OrderID` directly into `trade.PendingOrder` and treat the
// string fields as Binance-style decimal strings.
//
// The platform stores symbols in "BTC/USDC" form, which is exactly the form
// CCXT emits back; no normalisation needed on the return path.
//
// `Cost` × 1000 / `Cost` math etc. lives in mercury's trade actions on the
// callers' side; we just pass through what the exchange said.
func fromCCXTOrder(o ccxt.Order) aggregates.CreateOrderResponse {
	amount := derefFloat(o.Amount)
	filled := derefFloat(o.Filled)
	return aggregates.CreateOrderResponse{
		Symbol:                   derefString(o.Symbol),
		OrderID:                  parseOrderID(o.Id),
		ClientOrderID:            derefString(o.ClientOrderId),
		TransactTime:             derefInt64(o.Timestamp),
		Price:                    formatFloat(derefFloat(o.Price)),
		OrigQuantity:             formatFloat(amount),
		ExecutedQuantity:         formatFloat(filled),
		CummulativeQuoteQuantity: formatFloat(derefFloat(o.Cost)),
		Status:                   normalizeOrderStatus(derefString(o.Status), amount, filled),
		Type:                     strings.ToUpper(derefString(o.Type)),
		Side:                     strings.ToUpper(derefString(o.Side)),
		TimeInForce:              "GTC", // CCXT does not expose TIF on the typed Order; mercury defaults to GTC for limit orders
	}
}

// fromCCXTOrderFull maps a CCXT Order to mercury's richer Order struct used by
// GetOrder. Most fields overlap with CreateOrderResponse; the extra ones
// (Time, UpdateTime, StopPrice) come from the same CCXT fields.
func fromCCXTOrderFull(o ccxt.Order) aggregates.Order {
	amount := derefFloat(o.Amount)
	filled := derefFloat(o.Filled)
	return aggregates.Order{
		Symbol:                   derefString(o.Symbol),
		OrderID:                  parseOrderID(o.Id),
		ClientOrderID:            derefString(o.ClientOrderId),
		Price:                    formatFloat(derefFloat(o.Price)),
		OrigQuantity:             formatFloat(amount),
		ExecutedQuantity:         formatFloat(filled),
		CummulativeQuoteQuantity: formatFloat(derefFloat(o.Cost)),
		Status:                   normalizeOrderStatus(derefString(o.Status), amount, filled),
		Type:                     strings.ToUpper(derefString(o.Type)),
		Side:                     strings.ToUpper(derefString(o.Side)),
		StopPrice:                formatFloat(derefFloat(o.TriggerPrice)),
		Time:                     derefInt64(o.Timestamp),
		UpdateTime:               derefInt64(o.Timestamp),
	}
}

// fromCCXTOrderCancel maps a CCXT Order to mercury's CancelOrderResponse.
// CCXT does not distinguish "cancel response" from "order"; we re-shape the
// same data.
func fromCCXTOrderCancel(o ccxt.Order) aggregates.CancelOrderResponse {
	amount := derefFloat(o.Amount)
	filled := derefFloat(o.Filled)
	return aggregates.CancelOrderResponse{
		Symbol:                   derefString(o.Symbol),
		OrigClientOrderID:        derefString(o.ClientOrderId),
		OrderID:                  parseOrderID(o.Id),
		ClientOrderID:            derefString(o.ClientOrderId),
		TransactTime:             derefInt64(o.Timestamp),
		Price:                    formatFloat(derefFloat(o.Price)),
		OrigQuantity:             formatFloat(amount),
		ExecutedQuantity:         formatFloat(filled),
		CummulativeQuoteQuantity: formatFloat(derefFloat(o.Cost)),
		Status:                   normalizeOrderStatus(derefString(o.Status), amount, filled),
		Type:                     strings.ToUpper(derefString(o.Type)),
		Side:                     strings.ToUpper(derefString(o.Side)),
	}
}

// fromCCXTTrade maps a CCXT trade (single fill row) into mercury's Trade
// shape. Order id is parsed as int64 from CCXT's *string field.
//
// Commission asset is NOT on the typed CCXT Fee struct (which has only Rate +
// Cost). The currency lives in the raw `Info` map under the per-exchange key.
// CCXT's unified naming on the parsed map is "currency", so we pull it out
// defensively. Mercury's downstream fee math (CalculateFees, getFees) is
// hostile to an empty commission asset, so we fall back to the trade's base
// asset (first leg of the symbol) which is what Binance defaults to when
// commission is paid in the bought asset — same heuristic the go-binance
// adaptor's GetFeeDetails uses today.
func fromCCXTTrade(t ccxt.Trade) aggregates.Trade {
	feeCost := ""
	if t.Fee.Cost != nil {
		feeCost = formatFloat(*t.Fee.Cost)
	}
	feeAsset := extractFeeCurrency(t.Info, t.Symbol)
	parsedID, _ := strconv.ParseInt(derefString(t.Id), 10, 64)
	parsedOrderID, _ := strconv.ParseInt(derefString(t.Order), 10, 64)
	isBuyer := strings.EqualFold(derefString(t.Side), "buy")
	isMaker := strings.EqualFold(derefString(t.TakerOrMaker), "maker")
	return aggregates.Trade{
		ID:              parsedID,
		Symbol:          derefString(t.Symbol),
		OrderID:         parsedOrderID,
		Price:           formatFloat(derefFloat(t.Price)),
		Quantity:        formatFloat(derefFloat(t.Amount)),
		QuoteQuantity:   formatFloat(derefFloat(t.Cost)),
		Commission:      feeCost,
		CommissionAsset: feeAsset,
		Time:            derefInt64(t.Timestamp),
		IsBuyer:         isBuyer,
		IsMaker:         isMaker,
	}
}

// extractFeeCurrency pulls the unified `fee.currency` value out of CCXT's raw
// Info map. Defensively typed because Info is `map[string]any` populated from
// the exchange-specific JSON. Falls back to the trade's base asset if neither
// the unified key nor the raw key are present.
func extractFeeCurrency(info map[string]any, symbol *string) string {
	if info != nil {
		if rawFee, ok := info["fee"]; ok {
			if feeMap, ok := rawFee.(map[string]any); ok {
				if cur, ok := feeMap["currency"].(string); ok && cur != "" {
					return cur
				}
			}
		}
		// Some exchanges expose the asset under "commissionAsset" directly
		// (Binance native shape). Try that before giving up.
		if cur, ok := info["commissionAsset"].(string); ok && cur != "" {
			return cur
		}
	}
	if symbol == nil {
		return ""
	}
	parts := strings.SplitN(*symbol, "/", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
