package actions

import (
	"strings"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/log"
)

// fiatConversionSymbol pegs the quote asset used to convert non-USD profit
// into USD. Kept as USDT for now to preserve historical behavior; note that
// getSymbolPrice in getFees.go uses USDC — the inconsistency should be
// resolved in a separate change.
const fiatConversionSymbol = "USDT"

// CalculateUSDProfit converts trade.Profit into USD. Lookup order:
//
//  1. trade.ProfitAsset already pegged to USD (USDT/USDC/etc.) — return
//     trade.Profit verbatim.
//  2. event.WsPrices contains the <ProfitAsset>/USDT pair — multiply.
//     Hermes-fed snapshot; zero exchange round-trips, deterministic in
//     tests.
//  3. Fallback: event.Exchange.Client().GetPrice. Honors IsCustom so the
//     virtual exchange (backtesting) and test stubs can substitute a
//     deterministic price.
//
// Returns 0 on any price-lookup error (logged with context). Callers
// should not treat 0 as a hard failure — a closed trade with zero
// USDProfit is still valid downstream.
//
// Signature was changed from (trade aggragates.Trades) to (event events.Events)
// to remove the os.Getenv + crypto.Decrypt + fresh-Client construction
// hidden inside the previous implementation. The exchange client comes from
// the event, already built and decrypted upstream, which also makes the
// function unit-testable.
func CalculateUSDProfit(event events.Events) float64 {
	trade := event.Trade

	if strings.Contains(trade.ProfitAsset, "USD") {
		return trade.Profit
	}

	symbol := trade.ProfitAsset + "/" + fiatConversionSymbol

	if p, ok := event.WsPrices[symbol]; ok && p > 0 {
		return trade.Profit * p
	}

	client, clientErr := event.Exchange.Client()
	if clientErr != nil {
		log.Error(clientErr.Error(), "Client", "CalculateUSDProfit")
		return 0
	}
	price, priceErr := client.GetPrice(symbol)
	if priceErr != nil {
		log.Error(priceErr.Error(), "GetPrice", "CalculateUSDProfit")
		return 0
	}
	return trade.Profit * price
}
