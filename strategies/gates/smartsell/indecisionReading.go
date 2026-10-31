package smartsell

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// indecisionReads is whether sophos' indecision stands on the last closed bar (Indecision):
// the vote one short of the need, the near miss, the SMC trend tolerance or the whole table bearish.
func indecisionReads(block aggragates.SmartSellIndicators) bool {
	return block.Indecision
}

// indecisionReadsRecently is whether sophos served a bar among its last IndecisionRecentBars closed
// bars on which the indecision stood (IndecisionRecentAt above zero); it asks nothing of
// the ladder's fills and fails closed on a block without the key.
func indecisionReadsRecently(block aggragates.SmartSellIndicators) bool {
	return block.IndecisionRecentAt > 0
}
