package aggragates

import (
	"encoding/json"
	"testing"
)

// A stored params document may carry a key that is no longer a field — the
// jsonb column keeps whatever the strategy was last saved with. Decoding
// ignores it and reads the flags that remain, so retiring a flag needs no
// data migration.
func TestStrategyParamsIgnoresAKeyThatIsNotAField(t *testing.T) {
	raw := []byte(`{"legacyFlag":true,"cooldown":true}`)

	var params StrategyParams
	if err := json.Unmarshal(raw, &params); err != nil {
		t.Fatalf("an unknown key must decode without error, got %v", err)
	}
	if !params.Cooldown {
		t.Fatalf("the real flag must decode beside the unknown key, got %+v", params)
	}
	if params != (StrategyParams{Cooldown: true}) {
		t.Fatalf("the unknown key must set nothing, got %+v", params)
	}
}
