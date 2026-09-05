package regime

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The reason printed on a long add veto must be the timeframe that actually
// carries the block, never a bullish label from the first-match scan.
func TestRegimeDetailNamesTheRealBlocker(t *testing.T) {
	cases := []struct {
		name    string
		regimes map[string]string
		head    string
		want    string
	}{
		{
			name:    "2h blocks while 4h is a rising shock",
			regimes: map[string]string{"4h": ShockUp, "2h": DownPersist, "1h": UpPersist, "15m": UpPersist},
			want:    "2h downtrend-persist",
		},
		{
			name:    "both legs block",
			regimes: map[string]string{"4h": DownPersist, "2h": DownPersist, "1h": DownPersist, "15m": ShockDown},
			want:    "4h+2h downtrend-persist",
		},
		{
			name:    "4h blocks, 15m shock is irrelevant",
			regimes: map[string]string{"4h": DownPersist, "2h": UpPersist, "1h": UpPersist, "15m": ShockDown},
			want:    "4h downtrend-persist",
		},
		{
			name:    "1h is no longer a long add-veto leg: the headline, never 1h",
			regimes: map[string]string{"4h": ShockUp, "2h": UpPersist, "1h": DownPersist, "15m": UpPersist},
			head:    "mixed",
			want:    "mixed",
		},
		{
			name:    "no leg carries the block: the headline, not an invented label",
			regimes: map[string]string{"4h": ShockUp, "2h": UpPersist, "1h": UpPersist, "15m": ShockUp},
			head:    "mixed",
			want:    "mixed",
		},
	}
	for _, tc := range cases {
		got := regimeDetail(aggragates.AIIndicators{Regimes: tc.regimes, Regime: tc.head})
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
