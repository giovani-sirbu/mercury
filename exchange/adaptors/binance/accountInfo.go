package binanceAdaptor

import (
	"context"

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
// if /getUserAsset errors (older API keys lack permission for it but still
// return the same data inside the account endpoint).
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
		return profileInfo.Balances, nil
	}
	var clientInfoResult []aggregates.UserAssetRecord
	copier.Copy(&clientInfoResult, &clientInfo)
	return clientInfoResult, nil
}

// APIKeyPermission returns the scopes attached to the configured API key.
// Callers should check EnableSpotAndMarginTrading before placing orders.
func (e Binance) APIKeyPermission() (aggregates.APIKeyPermission, *common.APIError) {
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
