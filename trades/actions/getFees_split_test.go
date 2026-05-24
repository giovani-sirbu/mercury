package actions

import "testing"

func TestSplitSymbolReturnsBaseAndQuote(t *testing.T) {
	base, quote := splitSymbol("BTC/USDT")
	if base != "BTC" || quote != "USDT" {
		t.Errorf("splitSymbol(BTC/USDT) = (%q,%q), want (BTC,USDT)", base, quote)
	}
}

func TestSplitSymbolReturnsEmptyStringsForMalformedSymbol(t *testing.T) {
	base, quote := splitSymbol("BTCUSDT")
	if base != "" || quote != "" {
		t.Errorf("splitSymbol(BTCUSDT) = (%q,%q), want both empty", base, quote)
	}
}
