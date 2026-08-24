package binanceAdaptor

import (
	"context"
	"fmt"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/jinzhu/copier"
)

// GetProfile returns the /api/v3/account payload (commissions, permissions,
// balances). It is the only account endpoint the Binance Spot TESTNET serves:
// every /sapi/* route redirects there, so testnet reads go through it.
func (e Binance) GetProfile() (aggregates.Account, *common.APIError) {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return aggregates.Account{}, initErr
	}
	clientInfo, err := client.NewGetAccountService().Do(context.Background())
	if err != nil {
		return aggregates.Account{}, ApiError(err)
	}

	var clientInfoResult aggregates.Account
	copier.Copy(&clientInfoResult, &clientInfo)
	return clientInfoResult, nil
}

// GetUserAssets returns the per-asset balance list.
//
// Testnet: /sapi/v3/asset/getUserAsset does not exist on testnet.binance.vision,
// so the balances come straight from /api/v3/account (no BTC valuation —
// callers treat it as 0). Mainnet: getUserAsset first (carries btcValuation);
// on ANY failure (permission-less key, redirect/HTML, transport) fall back to
// the account balances, and surface both reasons when that fails as well so
// the caller never sees an empty error.
func (e Binance) GetUserAssets() ([]aggregates.UserAssetRecord, *common.APIError) {
	if e.TestNet {
		return e.accountBalances(nil)
	}

	client, initErr := InitExchange(e)
	if initErr != nil {
		return nil, initErr
	}
	clientInfo, err := client.NewGetUserAsset().NeedBtcValuation(true).Do(context.Background())
	if err != nil {
		return e.accountBalances(ApiError(err))
	}
	var clientInfoResult []aggregates.UserAssetRecord
	copier.Copy(&clientInfoResult, &clientInfo)
	return clientInfoResult, nil
}

// accountBalances maps /api/v3/account balances onto UserAssetRecord rows.
// primaryErr is the getUserAsset failure that triggered the fallback (nil on
// testnet) and is folded into the error when the account call fails too.
func (e Binance) accountBalances(primaryErr *common.APIError) ([]aggregates.UserAssetRecord, *common.APIError) {
	profileInfo, profileErr := e.GetProfile()
	if profileErr != nil {
		if primaryErr == nil {
			return nil, profileErr
		}
		return nil, &common.APIError{
			Code:    profileErr.Code,
			Message: fmt.Sprintf("getUserAsset: %s; account: %s", primaryErr.Message, profileErr.Message),
		}
	}
	return AccountBalancesToUserAssets(profileInfo.Balances), nil
}

// AccountBalancesToUserAssets is the pure /api/v3/account -> UserAssetRecord
// mapping: Asset/Free/Locked are kept, valuation columns stay empty (0) because
// the account endpoint carries none. Zero balances are dropped, like
// getUserAsset does, so callers iterate only held assets.
func AccountBalancesToUserAssets(balances []aggregates.UserAssetRecord) []aggregates.UserAssetRecord {
	assets := make([]aggregates.UserAssetRecord, 0, len(balances))
	for _, balance := range balances {
		if isZeroAmount(balance.Free) && isZeroAmount(balance.Locked) {
			continue
		}
		assets = append(assets, aggregates.UserAssetRecord{
			Asset:  balance.Asset,
			Free:   balance.Free,
			Locked: balance.Locked,
		})
	}
	return assets
}

// APIKeyPermission returns the scopes attached to the configured API key.
// Callers should check EnableSpotAndMarginTrading before placing orders.
//
// Testnet: /sapi/v1/account/apiRestrictions does not exist there, so the
// permission set is derived from the signed /api/v3/account call — a key
// that can read the account with canTrade=true may trade spot on testnet.
func (e Binance) APIKeyPermission() (aggregates.APIKeyPermission, *common.APIError) {
	if e.TestNet {
		profileInfo, profileErr := e.GetProfile()
		if profileErr != nil {
			return aggregates.APIKeyPermission{}, profileErr
		}
		return PermissionsFromAccount(profileInfo), nil
	}

	client, initErr := InitExchange(e)
	if initErr != nil {
		return aggregates.APIKeyPermission{}, initErr
	}
	var permissions aggregates.APIKeyPermission
	clientInfo, err := client.NewGetAPIKeyPermission().Do(context.Background())
	if err != nil {
		return permissions, ApiError(err)
	}

	copier.Copy(&permissions, &clientInfo)
	return permissions, nil
}

// PermissionsFromAccount is the pure account -> API-key permission mapping
// used where /sapi is unavailable: reading succeeded (we hold the payload),
// and spot trading follows canTrade.
func PermissionsFromAccount(account aggregates.Account) aggregates.APIKeyPermission {
	return aggregates.APIKeyPermission{
		EnableReading:              true,
		EnableSpotAndMarginTrading: account.CanTrade,
	}
}

func isZeroAmount(amount string) bool {
	for _, character := range amount {
		if character != '0' && character != '.' {
			return false
		}
	}
	return true
}
