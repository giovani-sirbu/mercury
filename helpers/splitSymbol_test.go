package helpers

import "testing"

func TestSplitSymbolReturnsBaseAndQuote(t *testing.T) {
	base, quote := SplitSymbol("BTC/USDT")
	if base != "BTC" || quote != "USDT" {
		t.Errorf("SplitSymbol(BTC/USDT) = (%q,%q), want (BTC,USDT)", base, quote)
	}
}

func TestSplitSymbolReturnsEmptyStringsForMalformedSymbol(t *testing.T) {
	base, quote := SplitSymbol("BTCUSDT")
	if base != "" || quote != "" {
		t.Errorf("SplitSymbol(BTCUSDT) = (%q,%q), want both empty", base, quote)
	}
}
