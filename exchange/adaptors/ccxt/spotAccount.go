package ccxt

import (
	"strconv"

	"github.com/adshao/go-binance/v2/common"
	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

// stableQuoteAssets are the fiat-pegged quote currencies for which the
// "{asset}/BTC" pair does NOT exist on Binance (and most exchanges) — the
// usual market is BTC/{asset} instead, so to value the stable in BTC we
// invert the BTC/stable price.
var stableQuoteAssets = map[string]bool{
	"USDC":  true,
	"USDT":  true,
	"BUSD":  true,
	"FDUSD": true,
	"TUSD":  true,
	"DAI":   true,
	"EUR":   true,
}

// GetUserAssets returns the user's balances per asset, in mercury's
// UserAssetRecord shape. CCXT's FetchBalance gives quantities only; the
// legacy Binance adaptor relied on /sapi/v3/asset/getUserAsset to also return
// the BTC valuation per asset, which agora's FetchUserAssets sums to drive
// the dashboard "total balance" figure.
//
// Without enrichment, FetchUserAssets only counts the BTC row (because every
// other asset's BtcValuation is empty/zero, and its dispatch loop bails out
// of the totaling branch). Stablecoins and altcoins silently disappear from
// the displayed balance — that was the bug observed on the dashboard after
// switching to the CCXT backend.
//
// To match the legacy behaviour we fetch tickers for the relevant pairs and
// compute BtcValuation locally. Direction is handled per asset family:
//   - BTC itself: leave BtcValuation empty; FetchUserAssets handles BTC via a
//     dedicated `Free + Freeze` branch.
//   - Stable quotes (USDC/USDT/...): the market on the exchange is
//     BTC/{stable}, so BtcValuation = quantity / BTC-quote-price.
//   - Everything else: the market is {asset}/BTC, so BtcValuation =
//     quantity × ticker.Last.
//
// All ticker lookups happen in a single FetchTickers call to keep the cost
// to one extra round-trip per FetchUserAssets cache miss (cached for 1h
// upstream — so a handful of extra ticker fetches per day per exchange).
func getUserAssets(e aggregates.Exchange) ([]aggregates.UserAssetRecord, *common.APIError) {
	client, err := newClient(e)
	if err != nil {
		return nil, apiError(err)
	}
	balances, ccxtErr := client.FetchBalance()
	if ccxtErr != nil {
		return nil, apiError(ccxtErr)
	}

	// Pass 1: collect non-zero balances into mercury's shape.
	out := make([]aggregates.UserAssetRecord, 0, len(balances.Balances))
	for asset, b := range balances.Balances {
		free := derefFloat(b.Free)
		used := derefFloat(b.Used)
		if free == 0 && used == 0 {
			continue
		}
		out = append(out, aggregates.UserAssetRecord{
			Asset:  asset,
			Free:   formatFloat(free),
			Locked: formatFloat(used),
		})
	}
	if len(out) == 0 {
		return out, nil
	}

	// Pass 2: enrich with BtcValuation via a single FetchTickers call.
	enrichBtcValuation(client, out)
	return out, nil
}

// enrichBtcValuation populates the BtcValuation field on each non-BTC asset
// in-place. Errors are best-effort — if a ticker is missing the asset still
// shows up in the dashboard list, just without a BTC equivalent (the legacy
// adaptor's behaviour was identical on missing markets).
func enrichBtcValuation(client ccxt.IExchange, assets []aggregates.UserAssetRecord) {
	var symbols []string
	for _, a := range assets {
		if a.Asset == "BTC" {
			continue
		}
		if stableQuoteAssets[a.Asset] {
			symbols = append(symbols, "BTC/"+a.Asset)
		} else {
			symbols = append(symbols, a.Asset+"/BTC")
		}
	}
	if len(symbols) == 0 {
		return
	}
	tickers, ccxtErr := client.FetchTickers(ccxt.WithFetchTickersSymbols(symbols))
	if ccxtErr != nil {
		return
	}

	for i, a := range assets {
		if a.Asset == "BTC" {
			continue
		}
		freeFloat, _ := strconv.ParseFloat(a.Free, 64)
		lockedFloat, _ := strconv.ParseFloat(a.Locked, 64)
		quantity := freeFloat + lockedFloat
		if quantity == 0 {
			continue
		}
		var btcValue float64
		if stableQuoteAssets[a.Asset] {
			t, ok := tickers.Tickers["BTC/"+a.Asset]
			if !ok || t.Last == nil || *t.Last == 0 {
				continue
			}
			btcValue = quantity / *t.Last
		} else {
			t, ok := tickers.Tickers[a.Asset+"/BTC"]
			if !ok || t.Last == nil {
				continue
			}
			btcValue = quantity * *t.Last
		}
		if btcValue > 0 {
			assets[i].BtcValuation = formatFloat(btcValue)
		}
	}
}

// GetProfile returns the account-level metadata. CCXT's FetchAccounts returns
// a list of accounts; the spot account is typically the first. Mercury's
// Account aggregates carries CanTrade/CanWithdraw/CanDeposit/Permissions —
// CCXT does not expose these uniformly, so we infer from FetchBalance success
// (we got here ⇒ reading works) and leave the rest of the bits as defaults
// that downstream consumers tolerate.
func getProfile(e aggregates.Exchange) (aggregates.Account, *common.APIError) {
	client, err := newClient(e)
	if err != nil {
		return aggregates.Account{}, apiError(err)
	}
	balances, ccxtErr := client.FetchBalance()
	if ccxtErr != nil {
		return aggregates.Account{}, apiError(ccxtErr)
	}
	var assets []aggregates.UserAssetRecord
	for asset, b := range balances.Balances {
		free := derefFloat(b.Free)
		used := derefFloat(b.Used)
		if free == 0 && used == 0 {
			continue
		}
		assets = append(assets, aggregates.UserAssetRecord{
			Asset:  asset,
			Free:   formatFloat(free),
			Locked: formatFloat(used),
		})
	}
	return aggregates.Account{
		CanTrade:    true,
		CanWithdraw: true,
		CanDeposit:  true,
		AccountType: "SPOT",
		Permissions: []string{"SPOT"},
		Balances:    assets,
	}, nil
}

// APIKeyPermission reports which capabilities the API key has. CCXT does not
// expose a unified permissions endpoint — each exchange differs. For Binance
// specifically the closest is `sapi/v1/account/apiRestrictions`, but CCXT v4
// has not surfaced this in the unified API.
//
// Pragmatic default: if FetchBalance succeeds we know the key can READ. We
// optimistically claim spot+margin trading is enabled (the actual trade call
// will surface a permission error from the exchange if not). This mirrors
// mercury's existing check-and-call pattern in CheckExchangeBalanceAndPermissions
// — the legacy path also relies on the order placement failing if
// permissions are wrong.
//
// Mercury's CreateTrade flow only KEYS on EnableSpotAndMarginTrading. Setting
// it true here removes the false negatives we'd get if CCXT returned nothing
// useful.
func getAPIKeyPermission(e aggregates.Exchange) (aggregates.APIKeyPermission, *common.APIError) {
	client, err := newClient(e)
	if err != nil {
		return aggregates.APIKeyPermission{}, apiError(err)
	}
	if _, ccxtErr := client.FetchBalance(); ccxtErr != nil {
		return aggregates.APIKeyPermission{}, apiError(ccxtErr)
	}
	return aggregates.APIKeyPermission{
		EnableReading:              true,
		EnableSpotAndMarginTrading: true,
	}, nil
}
