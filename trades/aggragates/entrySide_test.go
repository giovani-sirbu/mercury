package aggragates

import "testing"

func futuresTrade() Trades {
	trade := Trades{}
	trade.Strategy.TradeType = Futures
	return trade
}

// On futures the ML verdict IS the direction. Reading Inverse there — which is
// never set for a futures trade — made every SHORT entry look like a long into
// a bearish market, which is what vetoed it.
func TestEntrySideReadsTheVerdictOnFutures(t *testing.T) {
	for _, c := range []struct {
		action string
		want   string
	}{
		{ActionLong, SideLong},
		{ActionShort, SideShort},
		{ActionHold, ""},
		{"", ""},
	} {
		got := EntrySide(futuresTrade(), AIIndicators{AIAction: c.action})
		if got != c.want {
			t.Errorf("futures %q resolved to %q, want %q", c.action, got, c.want)
		}
	}
}

// On spot the ladder's own direction decides, and the verdict is irrelevant:
// an inverse ladder sells the base asset first whatever the ML says.
func TestEntrySideReadsTheLadderOnSpot(t *testing.T) {
	long := Trades{}
	long.Strategy.TradeType = Spot
	if got := EntrySide(long, AIIndicators{AIAction: ActionShort}); got != SideLong {
		t.Errorf("a spot ladder resolved to %q, want %q", got, SideLong)
	}

	inverse := Trades{Inverse: true}
	inverse.Strategy.TradeType = Spot
	if got := EntrySide(inverse, AIIndicators{AIAction: ActionLong}); got != SideShort {
		t.Errorf("an inverse ladder resolved to %q, want %q", got, SideShort)
	}
}

// An unset TradeType is spot: the column defaults to it, and a trade loaded
// before the futures work existed carries the zero value.
func TestEntrySideTreatsAnUnsetTradeTypeAsSpot(t *testing.T) {
	if got := EntrySide(Trades{}, AIIndicators{}); got != SideLong {
		t.Errorf("an unset trade type resolved to %q, want %q", got, SideLong)
	}
}
