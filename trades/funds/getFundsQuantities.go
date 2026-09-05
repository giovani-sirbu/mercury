// Package funds reads the exchange wallet and decides whether a trade can
// afford its next order.
package funds

import (
	"errors"
	"strconv"
	"strings"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/fees"
	"github.com/giovani-sirbu/mercury/trades/ladder"
	"github.com/giovani-sirbu/mercury/trades/quantities"
)

// GetFundsQuantities returns the wallet balance still free for the trade's
// next order, the quantity that order needs and the asset both are counted
// in. It also verifies the API key can trade the symbol at all.
func GetFundsQuantities(event events.Events) (float64, float64, string, error) {
	client, clientErr := event.Exchange.Client()
	if clientErr != nil {
		return 0, 0, "", clientErr
	}

	// get user assets (and check IP restrictions if any)
	assets, assetsErr := client.GetUserAssets() // Get user balance
	sellAction := false
	if assetsErr != nil {
		return 0, 0, "", assetsErr
	}

	// check if spot & margin trading is enabled
	permissions, permissionsErr := client.APIKeyPermission()
	if permissionsErr != nil {
		return 0, 0, "", permissionsErr
	}
	if !permissions.EnableSpotAndMarginTrading {
		return 0, 0, "", errors.New("Spot & Margin Trading is not enabled")
	}

	// check if symbol is whitelisted
	priceInString := strconv.FormatFloat(event.Trade.PositionPrice, 'f', -1, 64)
	_, err := client.Sell(event.Trade.Symbol, 0, priceInString)
	if err != nil {
		// -2010 is code for whitelisted symbol
		if err.Code == -2010 {
			return 0, 0, "", err
		}
	}

	strategySettings := event.Trade.StrategyPair.StrategySettings
	filledEntries := ladder.CountFilledEntries(event.Trade)
	// Same row-selection contract as Buy: entry N reads row N-1, missing rows
	// fall back to the base row 0.
	settingsIndex := ladder.SettingsIndexOrBase(strategySettings, filledEntries)

	pairSymbols := strings.Split(event.Trade.Symbol, "/")
	multiplier := strategySettings[settingsIndex].Multiplier

	var assetSymbol string
	var neededQuantity float64

	if event.Trade.PositionType == "sell" || event.Trade.PositionType == "takeProfit" || event.Trade.PositionType == "sellParent" {
		sellAction = true
	}

	if sellAction {
		buyQty, sellQty := quantities.GetGrossQuantities(event)
		assetSymbol = pairSymbols[0]
		neededQuantity = buyQty - sellQty

		// Literal asset fees only — see sell.go for the rationale.
		feeInBase, feeInQuote := fees.CalculateFees(event)
		neededQuantity -= feeInBase

		if event.Trade.Inverse {
			neededQuantity = (sellQty - buyQty - feeInQuote) * event.Trade.PositionPrice
			assetSymbol = pairSymbols[1]
		}
	} else {
		assetSymbol = pairSymbols[1]
		quantityType := "BUY"
		if event.Trade.Inverse {
			quantityType = "SELL"
			assetSymbol = pairSymbols[0]
		}
		neededQuantity = ladder.GetLatestQuantityByHistory(event.Trade.History, quantityType) * multiplier
		if !event.Trade.Inverse {
			neededQuantity *= event.Trade.PositionPrice
		}
	}

	remainedQuantity := GetAssetBudget(assets, assetSymbol)

	if !event.Trade.Inverse {
		remainedQuantity = remainedQuantity - helpers.FindUsedAmount(event.Params.InverseUsedAmount, assetSymbol)
		neededQuantity = helpers.ToFixed(neededQuantity, int(event.Trade.StrategyPair.TradeFilters.LotSize))
	}

	return remainedQuantity, neededQuantity, assetSymbol, nil
}
