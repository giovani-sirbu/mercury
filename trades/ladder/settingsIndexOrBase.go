package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// SettingsIndexOrBase resolves which StrategySettings row governs a given
// ladder depth. The contract, per configuration semantics:
//   - a single configured row applies to every depth;
//   - a depth whose row exists uses exactly that row;
//   - a depth whose row does NOT exist falls back to row 0 (the base row) —
//     never to the last row.
func SettingsIndexOrBase(settings []aggragates.StrategySettings, index int) int {
	if index < 0 || index >= len(settings) {
		return 0
	}
	return index
}
