package aggragates

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSettingsForTradeTypeSpotDropsFuturesKeys(t *testing.T) {
	in := []StrategySettings{{
		Depths: 10, MinDepths: 8, Percentage: 2.25, Multiplier: 2,
		Tolerance: 0.25, TrailingTakeProfit: 0.5, ImpasseDepth: 6,
		Leverage: 5, StopLoss: 3, KlineInterval: "15m",
	}}
	got := SettingsForTradeType(Spot, in)
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, key := range []string{`"leverage"`, `"stopLoss"`, `"klineInterval"`, `"timeframes"`, `"takeLossPercentage"`} {
		if strings.Contains(text, key) {
			t.Fatalf("spot JSON must omit %s, got %s", key, text)
		}
	}
	for _, key := range []string{`"depths"`, `"minDepths"`, `"percentage"`, `"multiplier"`, `"tolerance"`, `"trailingTakeProfit"`, `"impasseDepths"`} {
		if !strings.Contains(text, key) {
			t.Fatalf("spot JSON must keep %s, got %s", key, text)
		}
	}
}

func TestSettingsForTradeTypeFuturesKeepsOrderFields(t *testing.T) {
	in := []StrategySettings{{
		Depths: 8, Percentage: 2, Leverage: 0, KlineInterval: "",
	}}
	got := SettingsForTradeType(Futures, in)
	raw, err := json.Marshal(got[0])
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, key := range []string{`"leverage"`, `"klineInterval"`, `"timeframes"`, `"coolOffInMinutes"`} {
		if !strings.Contains(text, key) {
			t.Fatalf("futures JSON must include %s even at zero, got %s", key, text)
		}
	}
}

func TestStrategySettingsUnmarshalIgnoresUnknownKeys(t *testing.T) {
	var got StrategySettings
	err := json.Unmarshal([]byte(`{"depths":10,"takeLossPercentage":4,"rangeIntervals":2}`), &got)
	if err != nil {
		t.Fatal(err)
	}
	if got.Depths != 10 {
		t.Fatalf("depths = %v, want 10", got.Depths)
	}
}
