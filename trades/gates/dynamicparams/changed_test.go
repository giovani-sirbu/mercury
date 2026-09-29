package dynamicparams

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// An edge is a change of what the reads raise under the shipped constants.
func TestChangedFollowsTheIncrease(t *testing.T) {
	notRead := aggragates.DynamicParamsIndicators{Timeframe: "1D"}
	bothBullish := aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: 1, BMSB: 1, Valid: true}
	mixedRaises := MixedIncrease != IncreaseNone

	cases := []struct {
		name     string
		previous aggragates.DynamicParamsIndicators
		current  aggragates.DynamicParamsIndicators
		want     bool
	}{
		{"nothing read on either tick", aggragates.DynamicParamsIndicators{}, aggragates.DynamicParamsIndicators{}, false},
		{"the base tier on both ticks", messageNotBearish, bothBullish, false},
		{"the base tier turning both bearish", messageNotBearish, messageBothBearish, true},
		{"both bearish turning to the base tier", messageBothBearish, messageNotBearish, true},
		{"both bearish on both ticks", messageBothBearish, messageBothBearish, false},
		{"the bearish read passing from one row to the other", messageMixedGuppy, messageMixedBMSB, false},
		{"the base tier turning mixed", messageNotBearish, messageMixedBMSB, mixedRaises},
		{"mixed turning to the base tier", messageMixedGuppy, messageNotBearish, mixedRaises},
		{"mixed turning both bearish", messageMixedBMSB, messageBothBearish, MixedIncrease != IncreaseBoth},
		{"both bearish no longer read", messageBothBearish, notRead, true},
		{"the base tier no longer read", messageNotBearish, notRead, false},
		{"read again, both bearish", notRead, messageBothBearish, true},
	}

	for _, c := range cases {
		if got := Changed(c.previous, c.current); got != c.want {
			t.Errorf("%s: Changed = %v, want %v", c.name, got, c.want)
		}
	}
}

// The edges are the increase's, not the tier's: while the mixed tier raises
// nothing, moving between it and the base tier writes no row, and while it
// raises both, moving between it and both bearish writes none either.
func TestChangedComparesIncreasesNotTiers(t *testing.T) {
	cases := []struct {
		name     string
		mixed    Increase
		previous aggragates.DynamicParamsIndicators
		current  aggragates.DynamicParamsIndicators
		want     bool
	}{
		{"base to mixed, mixed raising nothing", IncreaseNone, messageNotBearish, messageMixedBMSB, false},
		{"mixed to base, mixed raising nothing", IncreaseNone, messageMixedGuppy, messageNotBearish, false},
		{"mixed to both bearish, mixed raising nothing", IncreaseNone, messageMixedGuppy, messageBothBearish, true},
		{"mixed to both bearish, mixed raising both", IncreaseBoth, messageMixedBMSB, messageBothBearish, false},
		{"base to mixed, mixed raising both", IncreaseBoth, messageNotBearish, messageMixedBMSB, true},
		{"base to mixed, mixed raising the depths", IncreaseDepths, messageNotBearish, messageMixedGuppy, true},
		{"mixed to both bearish, mixed raising the depths", IncreaseDepths, messageMixedGuppy, messageBothBearish, true},
	}

	for _, c := range cases {
		if got := changedFor(c.previous, c.current, c.mixed); got != c.want {
			t.Errorf("%s: changedFor = %v, want %v", c.name, got, c.want)
		}
	}
}
