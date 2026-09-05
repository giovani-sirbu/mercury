package aggragates

import "encoding/json"

const (
	persistAuto byte = iota
	persistSpotShape
	persistFuturesShape
)

// SettingsForTradeType keeps the grid fields every strategy uses and, for
// futures only, the order/cool-off/range fields. Spot rows persist without
// those keys so strategy_settings matches the engine that will read them.
func SettingsForTradeType(tradeType TradeTypes, settings []StrategySettings) []StrategySettings {
	out := make([]StrategySettings, len(settings))
	for i, setting := range settings {
		out[i] = setting.ForTradeType(tradeType)
	}
	return out
}

func (s StrategySettings) ForTradeType(tradeType TradeTypes) StrategySettings {
	if tradeType == Futures {
		s.persistKind = persistFuturesShape
		return s
	}
	return StrategySettings{
		Tolerance:          s.Tolerance,
		MinDepths:          s.MinDepths,
		Depths:             s.Depths,
		ImpasseDepth:       s.ImpasseDepth,
		TrailingTakeProfit: s.TrailingTakeProfit,
		InitialBid:         s.InitialBid,
		Percentage:         s.Percentage,
		Multiplier:         s.Multiplier,
		persistKind:        persistSpotShape,
	}
}

func (s StrategySettings) hasFuturesFields() bool {
	return s.Leverage > 0 ||
		s.StopLoss > 0 ||
		s.PriceAdjustment != 0 ||
		s.CancelTimeInMinutes != 0 ||
		s.KeepAliveInterval != "" ||
		s.KlineInterval != "" ||
		s.CoolOffInMinutes != 0 ||
		s.MarginPercentage != 0 ||
		s.IncrementCoolOff != 0 ||
		s.Timeframes.MinTimeframes != 0 ||
		len(s.Timeframes.Values) > 0 ||
		len(s.Timeframes.Required) > 0
}

func (s StrategySettings) persistFutures() bool {
	switch s.persistKind {
	case persistFuturesShape:
		return true
	case persistSpotShape:
		return false
	default:
		return s.hasFuturesFields()
	}
}

type spotSettingsJSON struct {
	Tolerance          float64 `json:"tolerance"`
	MinDepths          float64 `json:"minDepths"`
	Depths             float64 `json:"depths"`
	ImpasseDepth       float64 `json:"impasseDepths"`
	TrailingTakeProfit float64 `json:"trailingTakeProfit"`
	InitialBid         float64 `json:"initialBid"`
	Percentage         float64 `json:"percentage"`
	Multiplier         float64 `json:"multiplier"`
}

type futuresSettingsJSON struct {
	spotSettingsJSON
	Leverage            uint8      `json:"leverage"`
	StopLoss            uint8      `json:"stopLoss"`
	PriceAdjustment     float64    `json:"priceAdjustment"`
	CancelTimeInMinutes int        `json:"cancelTimeInMinutes"`
	KeepAliveInterval   string     `json:"keepAliveInterval"`
	KlineInterval       string     `json:"klineInterval"`
	CoolOffInMinutes    int        `json:"coolOffInMinutes"`
	MarginPercentage    float64    `json:"marginPercentage"`
	IncrementCoolOff    int        `json:"incrementCoolOff"`
	Timeframes          Timeframes `json:"timeframes"`
}

func (s StrategySettings) MarshalJSON() ([]byte, error) {
	spot := spotSettingsJSON{
		Tolerance:          s.Tolerance,
		MinDepths:          s.MinDepths,
		Depths:             s.Depths,
		ImpasseDepth:       s.ImpasseDepth,
		TrailingTakeProfit: s.TrailingTakeProfit,
		InitialBid:         s.InitialBid,
		Percentage:         s.Percentage,
		Multiplier:         s.Multiplier,
	}
	if !s.persistFutures() {
		return json.Marshal(spot)
	}
	return json.Marshal(futuresSettingsJSON{
		spotSettingsJSON:    spot,
		Leverage:            s.Leverage,
		StopLoss:            s.StopLoss,
		PriceAdjustment:     s.PriceAdjustment,
		CancelTimeInMinutes: s.CancelTimeInMinutes,
		KeepAliveInterval:   s.KeepAliveInterval,
		KlineInterval:       s.KlineInterval,
		CoolOffInMinutes:    s.CoolOffInMinutes,
		MarginPercentage:    s.MarginPercentage,
		IncrementCoolOff:    s.IncrementCoolOff,
		Timeframes:          s.Timeframes,
	})
}

func (s *StrategySettings) UnmarshalJSON(data []byte) error {
	var parsed futuresSettingsJSON
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	*s = StrategySettings{
		Tolerance:           parsed.Tolerance,
		MinDepths:           parsed.MinDepths,
		Depths:              parsed.Depths,
		ImpasseDepth:        parsed.ImpasseDepth,
		TrailingTakeProfit:  parsed.TrailingTakeProfit,
		InitialBid:          parsed.InitialBid,
		Percentage:          parsed.Percentage,
		Multiplier:          parsed.Multiplier,
		Leverage:            parsed.Leverage,
		StopLoss:            parsed.StopLoss,
		PriceAdjustment:     parsed.PriceAdjustment,
		CancelTimeInMinutes: parsed.CancelTimeInMinutes,
		KeepAliveInterval:   parsed.KeepAliveInterval,
		KlineInterval:       parsed.KlineInterval,
		CoolOffInMinutes:    parsed.CoolOffInMinutes,
		MarginPercentage:    parsed.MarginPercentage,
		IncrementCoolOff:    parsed.IncrementCoolOff,
		Timeframes:          parsed.Timeframes,
	}
	return nil
}
