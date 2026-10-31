package ladder

import (
	"math"
	"testing"
)

// The costs stop offset depths short of the ceiling, the ceiling itself and
// the depth stay, and at 0 the view is DepthOf's.
func TestDepthOfOffsetLeavesTheLastDepthsOutOfTheCosts(t *testing.T) {
	rows := rowPerDepthRows(2, 2, 2, 2, 2)
	trade := costLadderTrade(2, 10, rows...)
	settings := tradedSettings(trade)
	whole := DepthOf(trade)

	if got := DepthOfOffset(trade, 0); got != whole {
		t.Errorf("offset 0 = %+v, want DepthOf %+v", got, whole)
	}

	for offset := 1; offset <= len(rows)-2; offset++ {
		got := DepthOfOffset(trade, offset)
		costed := len(rows) - offset

		if got.Depth != whole.Depth || got.MaxDepth != whole.MaxDepth || got.Asset != whole.Asset {
			t.Errorf("offset %d: %+v, want the depth, ceiling and asset of %+v", offset, got, whole)
		}
		want := remainingCostFrom(trade, settings, 2, costed, trade.PositionPrice)
		if math.Abs(got.RemainingCost-want) > costEpsilon {
			t.Errorf("offset %d: RemainingCost = %f, want the walk through depth %d, %f", offset, got.RemainingCost, costed, want)
		}
		wantPlanned := remainingCostFrom(trade, settings, 2, costed, lastEntryPrice(trade))
		if math.Abs(got.PlannedRemainingCost-wantPlanned) > costEpsilon {
			t.Errorf("offset %d: PlannedRemainingCost = %f, want the walk through depth %d, %f", offset, got.PlannedRemainingCost, costed, wantPlanned)
		}
		if got.RemainingCost >= whole.RemainingCost {
			t.Errorf("offset %d: RemainingCost = %f, want under the whole ladder's %f", offset, got.RemainingCost, whole.RemainingCost)
		}
	}

	// The offset that reaches the filled depth leaves nothing to cost, and a
	// larger one is no different.
	for _, offset := range []int{len(rows) - 2, len(rows), len(rows) + 3} {
		if got := DepthOfOffset(trade, offset); got.RemainingCost != 0 || got.PlannedRemainingCost != 0 || got.MaxDepth != whole.MaxDepth {
			t.Errorf("offset %d: %+v, want no cost left and the ceiling kept", offset, got)
		}
	}
}
