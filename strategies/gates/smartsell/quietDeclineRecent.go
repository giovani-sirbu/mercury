package smartsell

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/quietdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// quietDeclinePendsRecently is quietdecline.PendsRecently on the first and newest fills' stamps; a
// ladder with no fill hands over zero stamps, which the leaf fails closed on.
func quietDeclinePendsRecently(st state, block aggragates.SmartSellIndicators) bool {
	var first entryFill
	if len(st.fills) > 0 {
		first = st.fills[0]
	}
	return quietdecline.PendsRecently(first.At, st.lastFill().At, block)
}
