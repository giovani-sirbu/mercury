package dynamicparams_test

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
)

// Blocks for each tier on the timeframe the tests read.
var (
	bothBearish = aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: bearish, BMSB: bearish, Valid: true}
	mixedRead   = aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: bullish, BMSB: bearish, Valid: true}
	notBearish  = aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: bullish, BMSB: neutral, Valid: true}
	notRead     = aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: bearish, BMSB: bearish}
)

// threeRowLadder is a row-per-depth ladder whose rows differ in every field
// Adjust could touch, with a timeframe list to prove the rows' own slices are
// left alone too.
func threeRowLadder() []aggragates.StrategySettings {
	return []aggragates.StrategySettings{
		{
			Tolerance:          0.25,
			MinDepths:          6,
			Depths:             8,
			Percentage:         2,
			Multiplier:         2,
			TrailingTakeProfit: 0.1,
			ImpasseDepth:       4,
			Timeframes:         aggragates.Timeframes{Values: []string{"1h"}},
		},
		{
			Tolerance:          0.3,
			MinDepths:          6,
			Depths:             8,
			Percentage:         2.25,
			Multiplier:         1.8,
			TrailingTakeProfit: 0.1,
			ImpasseDepth:       4,
		},
		{
			Tolerance:          0.35,
			MinDepths:          6,
			Depths:             8,
			Percentage:         3,
			Multiplier:         1.5,
			TrailingTakeProfit: 0.1,
			ImpasseDepth:       4,
			InitialBid:         2,
		},
	}
}

// cloneRows copies the rows down to their own slices, so a write through
// any alias of the original shows up against the clone.
func cloneRows(rows []aggragates.StrategySettings) []aggragates.StrategySettings {
	clone := make([]aggragates.StrategySettings, len(rows))
	for index, row := range rows {
		row.Timeframes.Values = append([]string(nil), row.Timeframes.Values...)
		row.Timeframes.Required = append([]string(nil), row.Timeframes.Required...)
		clone[index] = row
	}
	return clone
}

// mixedRaises is what the mixed tier raises under the shipped MixedIncrease.
func mixedRaises() (percentage bool, depths bool) {
	switch dynamicparams.MixedIncrease {
	case dynamicparams.IncreasePercentage:
		return true, false
	case dynamicparams.IncreaseDepths:
		return false, true
	case dynamicparams.IncreaseBoth:
		return true, true
	default:
		return false, false
	}
}

// Reads that raise nothing hand back the very slice: same length, same
// backing array, nothing allocated for the engines to confuse with a raise.
func TestAdjustReturnsTheInputWhenNothingIsRaised(t *testing.T) {
	for name, reads := range map[string]aggragates.DynamicParamsIndicators{
		"not bearish":   notBearish,
		"not read":      notRead,
		"the zero read": {},
	} {
		in := threeRowLadder()
		got := dynamicparams.Adjust(in, reads)
		if len(got) != len(in) || &got[0] != &in[0] {
			t.Errorf("%s: Adjust must return the input slice itself", name)
		}
	}

	if got := dynamicparams.Adjust(nil, bothBearish); got != nil {
		t.Errorf("no rows: Adjust = %v, want the nil input back", got)
	}
	empty := []aggragates.StrategySettings{}
	if got := dynamicparams.Adjust(empty, bothBearish); got == nil || len(got) != 0 {
		t.Errorf("empty rows: Adjust = %v, want the empty input back", got)
	}
}

// A raise is a fresh array: the stored rows, which the backtest shares with
// its run's snapshot, read exactly as before — every field, every row — and
// a write into the raised copy never reaches them.
func TestAdjustNeverWritesTheInput(t *testing.T) {
	for name, reads := range map[string]aggragates.DynamicParamsIndicators{
		"both bearish": bothBearish,
		"mixed":        mixedRead,
	} {
		in := threeRowLadder()
		before := cloneRows(in)

		got := dynamicparams.Adjust(in, reads)
		if !reflect.DeepEqual(in, before) {
			t.Fatalf("%s: Adjust wrote its input: got %+v, want %+v", name, in, before)
		}
		if len(got) != len(in) {
			t.Fatalf("%s: Adjust returned %d rows for %d", name, len(got), len(in))
		}

		raisesSomething := name == "both bearish" || dynamicparams.MixedIncrease != dynamicparams.IncreaseNone
		if !raisesSomething {
			continue
		}
		if &got[0] == &in[0] {
			t.Fatalf("%s: a raise must be a copy, not the input", name)
		}
		got[0].Percentage = 99
		if in[0].Percentage != before[0].Percentage {
			t.Fatalf("%s: a write into the raised copy reached the input", name)
		}
	}
}

// Every row is raised, and only the fields the increase names move: both
// bearish raises the percentage and the depths of all three rows; mixed
// raises what MixedIncrease names.
func TestAdjustRaisesEveryRowAndOnlyTheNamedFields(t *testing.T) {
	mixedPercentage, mixedDepths := mixedRaises()

	cases := []struct {
		name       string
		reads      aggragates.DynamicParamsIndicators
		percentage bool
		depths     bool
	}{
		{"both bearish", bothBearish, true, true},
		{"mixed", mixedRead, mixedPercentage, mixedDepths},
	}

	for _, c := range cases {
		in := threeRowLadder()
		got := dynamicparams.Adjust(in, c.reads)

		for index := range in {
			wantPercentage := in[index].Percentage
			if c.percentage {
				wantPercentage += dynamicparams.BearPercentagePoints
			}
			wantDepths := in[index].Depths
			if c.depths {
				wantDepths += float64(dynamicparams.BearDepths)
			}

			if got[index].Percentage != wantPercentage {
				t.Errorf("%s row %d: percentage %v, want %v", c.name, index, got[index].Percentage, wantPercentage)
			}
			if got[index].Depths != wantDepths {
				t.Errorf("%s row %d: depths %v, want %v", c.name, index, got[index].Depths, wantDepths)
			}

			rest := got[index]
			rest.Percentage = in[index].Percentage
			rest.Depths = in[index].Depths
			if !reflect.DeepEqual(rest, in[index]) {
				t.Errorf("%s row %d: a field other than the percentage and the depths moved: %+v, want %+v", c.name, index, got[index], in[index])
			}
		}
	}
}

// marshalledKeys is the sorted list of keys a row marshals to.
func marshalledKeys(t *testing.T, row aggragates.StrategySettings) []string {
	t.Helper()

	raw, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	keys := make([]string, 0, len(decoded))
	for key := range decoded {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return keys
}

// A raised row keeps the shape its row persists in: a spot row — whether
// shaped for spot or never shaped at all — still marshals to the spot keys,
// and a futures row to the futures ones. The copy carries the row's own
// shape rather than guessing it again.
func TestAdjustKeepsEachRowsMarshalShape(t *testing.T) {
	spotKeys := []string{"depths", "impasseDepths", "initialBid", "minDepths", "multiplier", "percentage", "tolerance", "trailingTakeProfit"}

	plain := []aggragates.StrategySettings{{Tolerance: 0.25, MinDepths: 6, Depths: 8, Percentage: 2, Multiplier: 2}}
	spot := aggragates.SettingsForTradeType(aggragates.Spot, plain)
	futures := aggragates.SettingsForTradeType(aggragates.Futures, plain)

	for name, rows := range map[string][]aggragates.StrategySettings{
		"an unshaped spot row": plain,
		"a spot-shaped row":    spot,
	} {
		raised := dynamicparams.Adjust(rows, bothBearish)
		if got := marshalledKeys(t, raised[0]); !reflect.DeepEqual(got, spotKeys) {
			t.Errorf("%s raised marshals to %v, want the spot keys %v", name, got, spotKeys)
		}
	}

	raisedFutures := dynamicparams.Adjust(futures, bothBearish)
	if got, want := marshalledKeys(t, raisedFutures[0]), marshalledKeys(t, futures[0]); !reflect.DeepEqual(got, want) {
		t.Errorf("a raised futures row marshals to %v, want its own futures keys %v", got, want)
	}
}
