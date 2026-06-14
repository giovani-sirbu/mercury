package actions

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestHasNegativeHistoryAmountFindsNegativeQuantity(t *testing.T) {
	history := []aggragates.TradesHistory{
		{Quantity: 5},
		{Quantity: -1},
		{Quantity: 2},
	}
	if !hasNegativeHistoryAmount(history) {
		t.Fatal("expected true when a history entry has a negative quantity")
	}
}

func TestHasNegativeHistoryAmountReturnsFalseWhenAllPositive(t *testing.T) {
	history := []aggragates.TradesHistory{
		{Quantity: 5},
		{Quantity: 2},
	}
	if hasNegativeHistoryAmount(history) {
		t.Fatal("expected false when all history entries are non-negative")
	}
}

func TestHasNegativeHistoryAmountEmptyHistoryReturnsFalse(t *testing.T) {
	if hasNegativeHistoryAmount(nil) {
		t.Fatal("expected false for empty history")
	}
}
