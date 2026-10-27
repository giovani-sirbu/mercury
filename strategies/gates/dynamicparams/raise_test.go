package dynamicparams

import "testing"

// Both bearish always raises both and the base tier nothing; the mixed tier
// raises whichever of the four increases MixedIncrease names.
func TestRaiseForEveryMixedIncrease(t *testing.T) {
	for _, mixed := range []Increase{IncreasePercentage, IncreaseDepths, IncreaseBoth, IncreaseNone} {
		cases := []struct {
			tier Tier
			want Increase
		}{
			{TierBothBearish, IncreaseBoth},
			{TierMixed, mixed},
			{TierBase, IncreaseNone},
		}
		for _, c := range cases {
			if got := raiseFor(c.tier, mixed); got != c.want {
				t.Errorf("mixed %q, tier %d: raiseFor = %q, want %q", mixed, c.tier, got, c.want)
			}
		}
	}
}

// Raise reads the constant.
func TestRaiseFollowsMixedIncrease(t *testing.T) {
	if got := Raise(TierMixed); got != MixedIncrease {
		t.Errorf("Raise(TierMixed) = %q, want MixedIncrease %q", got, MixedIncrease)
	}
	if got := Raise(TierBothBearish); got != IncreaseBoth {
		t.Errorf("Raise(TierBothBearish) = %q, want %q", got, IncreaseBoth)
	}
	if got := Raise(TierBase); got != IncreaseNone {
		t.Errorf("Raise(TierBase) = %q, want %q", got, IncreaseNone)
	}
}

// What each increase names, and that only an increase naming an amount above
// zero changes a row; an unknown increase names nothing.
func TestIncreaseNamesItsFields(t *testing.T) {
	cases := []struct {
		increase   Increase
		percentage bool
		depths     bool
	}{
		{IncreasePercentage, true, false},
		{IncreaseDepths, false, true},
		{IncreaseBoth, true, true},
		{IncreaseNone, false, false},
		{Increase(""), false, false},
	}
	for _, c := range cases {
		if got := c.increase.raisesPercentage(); got != c.percentage {
			t.Errorf("%q: raisesPercentage = %v, want %v", c.increase, got, c.percentage)
		}
		if got := c.increase.raisesDepths(); got != c.depths {
			t.Errorf("%q: raisesDepths = %v, want %v", c.increase, got, c.depths)
		}
		wantChanges := (c.percentage && BearPercentagePoints != 0) || (c.depths && BearDepths != 0)
		if got := c.increase.changesRows(); got != wantChanges {
			t.Errorf("%q: changesRows = %v, want %v", c.increase, got, wantChanges)
		}
	}
}
