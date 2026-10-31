package ladder

import (
	"math"
	"testing"
)

// The number handed to the user has to be a number that works: depositing it
// must make the very gate that refused them accept. CalculateMinimumQuantity
// modelled the ladder separately and applied no wallet reserve, so its answer
// was refused on arrival.
func TestMinimumStartingWalletIsAcceptedByTheGateThatRefused(t *testing.T) {
	for _, inverse := range []bool{false, true} {
		trade := sizingTrade(inverse, 0.1, ladderSettings())

		required := MinimumStartingWallet(trade)
		if required <= 0 {
			t.Fatalf("inverse=%t: no minimum computed", inverse)
		}

		if _, err := CalculateInitialBid(required, trade, 0); err != nil {
			t.Errorf("inverse=%t: the reported minimum %f is still refused: %v", inverse, required, err)
		}
	}
}

// And it must be the real boundary, not a safe over-estimate: a hair under it
// is still refused, or the figure would send users to deposit more than the
// ladder needs.
func TestMinimumStartingWalletIsTheBoundary(t *testing.T) {
	for _, inverse := range []bool{false, true} {
		trade := sizingTrade(inverse, 0.1, ladderSettings())

		required := MinimumStartingWallet(trade)

		if _, err := CalculateInitialBid(required*0.999, trade, 0); err == nil {
			t.Errorf("inverse=%t: %f is below the reported minimum and was admitted", inverse, required*0.999)
		}
	}
}

// The reserve is part of the answer: holding more of the wallet out of the
// ladder means a bigger wallet is needed to reach the same first bid.
func TestMinimumStartingWalletCarriesTheWalletReserve(t *testing.T) {
	trade := sizingTrade(false, 0.1, ladderSettings())

	required := MinimumStartingWallet(trade)

	// The bid at the shallowest ladder is a fixed share of the budget, and the
	// budget is the wallet less the reserve, so the minimum scales with it.
	bare := MinimumStartingWallet(trade) * (1 - InitialBidReservePercent/100)
	if math.Abs(required-bare) < 1e-9 {
		t.Fatal("the reserve is missing from the reported minimum")
	}
}
