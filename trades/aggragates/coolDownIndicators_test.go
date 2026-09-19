package aggragates

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The sophos /markers payload is exactly three booleans, and this struct is
// the only place mercury reads them: trades/gates/cooldown decides the first
// fill on HasFirstFillVerdict plus the flag for the side the entry takes.
// Nothing else in either repo pins these json tags, so a rename or a dropped
// field would surface only as silently-degraded live trading.
func TestCoolDownIndicatorsDecodesTheMarkersPayload(t *testing.T) {
	var got CoolDownIndicators
	raw := []byte(`{"hasFirstFillVerdict":true,"allowLongEntry":false,"allowShortEntry":true}`)
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode markers payload: %v", err)
	}
	if want := (CoolDownIndicators{HasFirstFillVerdict: true, AllowShortEntry: true}); got != want {
		t.Fatalf("decoded %+v, want %+v", got, want)
	}

	typ := reflect.TypeOf(got)
	tags := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		tags = append(tags, typ.Field(i).Tag.Get("json"))
	}
	want := []string{"hasFirstFillVerdict", "allowLongEntry", "allowShortEntry"}
	if !reflect.DeepEqual(tags, want) {
		t.Errorf("json tags = %v, want exactly %v — the retired markers are off the wire", tags, want)
	}
}

// Deploy order is free in both directions. A sophos still serving the retired
// markers decodes to the same verdict, since an unknown key is ignored; and a
// payload that omits a key decodes to false, which for hasFirstFillVerdict is
// "no verdict" — the fail-open FirstFillHold already has, not a hold.
func TestCoolDownIndicatorsIgnoresRetiredAndMissingKeys(t *testing.T) {
	var legacy CoolDownIndicators
	raw := []byte(`{"volatilityScore":0.42,"marketBullish":true,"marketBearish":true,` +
		`"hasFirstFillVerdict":true,"allowLongEntry":true,"allowShortEntry":false}`)
	if err := json.Unmarshal(raw, &legacy); err != nil {
		t.Fatalf("decode a payload still carrying the retired markers: %v", err)
	}
	if want := (CoolDownIndicators{HasFirstFillVerdict: true, AllowLongEntry: true}); legacy != want {
		t.Fatalf("decoded %+v, want %+v", legacy, want)
	}

	var absent CoolDownIndicators
	if err := json.Unmarshal([]byte(`{}`), &absent); err != nil {
		t.Fatalf("decode an empty payload: %v", err)
	}
	if absent != (CoolDownIndicators{}) {
		t.Fatalf("an absent verdict decoded to %+v, want the zero verdict", absent)
	}
}
