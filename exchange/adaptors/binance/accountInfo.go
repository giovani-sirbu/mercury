package binanceAdaptor

import (
	"context"
	"strconv"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/jinzhu/copier"
)

// GetProfile returns the /account payload.
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

// GetUserAssets returns the per-asset balances, falling back to GetProfile when /getUserAsset errors.
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

// heldBalances drops the zero rows the account endpoint lists.
func heldBalances(balances []aggregates.UserAssetRecord) []aggregates.UserAssetRecord {
	held := make([]aggregates.UserAssetRecord, 0, len(balances))
	for _, balance := range balances {
		if assetQuantity(balance) > 0 {
			held = append(held, balance)
		}
	}
	return held
}

// assetQuantity totals Free, Locked and Freeze.
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

// APIKeyPermission returns the API key's scopes; the testnet serves no /sapi
// restrictions endpoint, so they are read off the account there.
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

// testNetPermission derives the scopes from the account endpoint: reading, and trading from CanTrade.
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
