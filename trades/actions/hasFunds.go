package actions

import (
	"errors"
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strconv"
	"strings"
)

func GetAssetBudget(assets []aggregates.UserAssetRecord, assetSymbol string) float64 {
	var remainedQuantity float64 // Init needed quantity

	// Check if account has remaining balance for pair
	for _, balance := range assets {
		if balance.Asset == assetSymbol {
			floatQuantity, _ := strconv.ParseFloat(balance.Free, 64)
			remainedQuantity = floatQuantity
		}
	}

	return remainedQuantity
}

func GetUsedQuantities(event events.Events) float64 {
	buyQty, sellQty := GetGrossQuantities(event)
	// Literal asset fees only: see sell.go for the rationale (BNB and other
	// third-asset fees do not reduce base or quote wallet balances).
	feeInBase, _ := CalculateFees(event)
	quantity := buyQty - sellQty - feeInBase

	if event.Trade.Inverse {
		sellInQuote := trades.GetQuantityInQuote(event.Trade.History, "BUY")
		buyInQuote := trades.GetQuantityInQuote(event.Trade.History, "SELL")
		quantity = sellInQuote - buyInQuote
		quantity = quantity / event.Trade.PositionPrice
		quantity = quantity - feeInBase
	}

	quantity = ToFixed(quantity, int(event.Trade.StrategyPair.TradeFilters.LotSize))
	return quantity
}

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
	filledEntries := trades.CountFilledEntries(event.Trade)
	// Same row-selection contract as Buy: entry N reads row N-1, missing rows
	// fall back to the base row 0.
	settingsIndex := trades.SettingsIndexOrBase(strategySettings, filledEntries)

	pairSymbols := strings.Split(event.Trade.Symbol, "/")
	multiplier := strategySettings[settingsIndex].Multiplier

	var assetSymbol string
	var neededQuantity float64

	if event.Trade.PositionType == "sell" || event.Trade.PositionType == "takeProfit" || event.Trade.PositionType == "sellParent" {
		sellAction = true
	}

	if sellAction {
		buyQty, sellQty := GetGrossQuantities(event)
		assetSymbol = pairSymbols[0]
		neededQuantity = buyQty - sellQty

		// Literal asset fees only — see sell.go for the rationale.
		feeInBase, feeInQuote := CalculateFees(event)
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
		neededQuantity = trades.GetLatestQuantityByHistory(event.Trade.History, quantityType) * multiplier
		if !event.Trade.Inverse {
			neededQuantity *= event.Trade.PositionPrice
		}
	}

	remainedQuantity := GetAssetBudget(assets, assetSymbol)

	if !event.Trade.Inverse {
		remainedQuantity = remainedQuantity - aggragates.FindUsedAmount(event.Params.InverseUsedAmount, assetSymbol)
		neededQuantity = ToFixed(neededQuantity, int(event.Trade.StrategyPair.TradeFilters.LotSize))
	}

	return remainedQuantity, neededQuantity, assetSymbol, nil
}

func HasFunds(event events.Events) (events.Events, error) {
	remainedQuantity, neededQuantity, assetSymbol, err := GetFundsQuantities(event)

	if err != nil {
		return events.Events{}, err
	}

	if remainedQuantity < neededQuantity {
		// enter take loss mode if this feature is activated for this strategy; takes precedence over impasse
		if event.Trade.Strategy.Params.TakeLoss && event.Trade.ParentID == 0 {
			// message must not contain "Insufficient funds" so SaveError keeps the trade active instead of blocking it
			msg := fmt.Sprintf("Take loss mode activated: budget (%f %s) is below the required %f %s for the next %s.", remainedQuantity, assetSymbol, neededQuantity, assetSymbol, event.Trade.PositionType)
			event.Trade.PositionType = "takeLoss"
			return SaveError(event, fmt.Errorf(msg))
		}

		// set trade to impasse if this feature is activated for this strategy
		if event.Trade.Strategy.Params.Impasse && event.Trade.ParentID == 0 {
			usedAmount := GetUsedQuantities(event) * event.Trade.PositionPrice
			_, hasFundsError := trades.CalculateInitialBid(usedAmount, event.Trade, 0)
			if hasFundsError == nil {
				event.Trade.PositionType = "impasse"
			}
		}

		return SaveError(event, fmt.Errorf("Insufficient funds (%f %s) for the requested action (%s). You need at least %f %s to resume this trade.", remainedQuantity, assetSymbol, event.Trade.PositionType, neededQuantity, assetSymbol))
	}

	return event, nil
}
