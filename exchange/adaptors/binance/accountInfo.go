package binanceAdaptor

import (
	"context"
	"strconv"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/jinzhu/copier"
)

// GetProfile returns the /account endpoint payload (commissions, permissions,
// balances).
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

// GetUserAssets returns the per-asset balance list. Falls back to GetProfile
// if /getUserAsset errors (older API keys lack permission for it, and the spot
// testnet does not serve the endpoint at all, but the account endpoint carries
// the same balances).
func (e Binance) GetUserAssets() ([]aggregates.UserAssetRecord, *common.APIError) {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return nil, initErr
	}
	clientInfo, err := client.NewGetUserAsset().NeedBtcValuation(true).Do(context.Background())
	if err != nil {
		profileInfo, profileErr := e.GetProfile()
		if profileErr != nil {
			return nil, profileErr
		}
		return heldBalances(profileInfo.Balances), nil
	}
	var clientInfoResult []aggregates.UserAssetRecord
	copier.Copy(&clientInfoResult, &clientInfo)
	return clientInfoResult, nil
}

// heldBalances drops the empty rows. /api/v3/account lists every asset the
// exchange knows about, nearly all of them at zero, while the endpoint it
// stands in for here reports only what the account actually holds. Callers
// count and value these rows, so the two sources have to agree on what a row
// means — otherwise a wallet reads as hundreds of empty assets.
func heldBalances(balances []aggregates.UserAssetRecord) []aggregates.UserAssetRecord {
	held := make([]aggregates.UserAssetRecord, 0, len(balances))
	for _, balance := range balances {
		if assetQuantity(balance) > 0 {
			held = append(held, balance)
		}
	}
	return held
}

// assetQuantity totals the buckets a balance row can hold funds in. The account
// endpoint fills only Free and Locked; /sapi rows add Freeze.
func assetQuantity(balance aggregates.UserAssetRecord) float64 {
	var total float64
	for _, amount := range []string{balance.Free, balance.Locked, balance.Freeze} {
		parsed, err := strconv.ParseFloat(amount, 64)
		if err != nil {
			continue
		}
		total += parsed
	}
	return total
}

// APIKeyPermission returns the scopes attached to the configured API key.
// Callers should check EnableSpotAndMarginTrading before placing orders.
//
// The spot testnet serves only the /api/v3 family, so the /sapi/v1 restrictions
// endpoint answers 404 there with an empty body — no code, no message. Every
// caller then failed on an error that could say nothing beyond "<APIError>
// rsp=", which is why testnet could neither list assets nor create a trade.
// Read the scopes off the account endpoint instead, which the testnet serves.
func (e Binance) APIKeyPermission() (aggregates.APIKeyPermission, *common.APIError) {
	if e.TestNet {
		return e.testNetPermission()
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

// testNetPermission derives the scopes from the account endpoint. A testnet key
// carries no withdrawal or transfer rights, so only the two fields the account
// payload can answer are filled and the rest stay false — callers gate on
// EnableSpotAndMarginTrading, which CanTrade answers directly.
func (e Binance) testNetPermission() (aggregates.APIKeyPermission, *common.APIError) {
	profile, profileErr := e.GetProfile()
	if profileErr != nil {
		return aggregates.APIKeyPermission{}, profileErr
	}

	return aggregates.APIKeyPermission{
		EnableReading:              true,
		EnableSpotAndMarginTrading: profile.CanTrade,
	}, nil
}
