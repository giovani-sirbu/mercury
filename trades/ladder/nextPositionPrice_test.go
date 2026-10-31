package ladder

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// Trade 167090: depth 10 filled at 2267.75, the take profit reverted to buy at 2286.96.
// Trade 211288: the depth 6 add rested at 0.0732 under a last entry at 0.21482, the print ran to 0.08134.
func TestNextPositionPriceCapsAReanchorToBuyAtTheLastEntry(t *testing.T) {
	long := aggragates.Trades{History: []aggragates.TradesHistory{
		{Type: "BUY", Quantity: 4.736, Price: 2338.14, OrderId: 9},
		{Type: "BUY", Quantity: 9.472, Price: 2267.75, OrderId: 10},
		{Type: "BUY", Quantity: 1, Price: 1e-13, OrderId: 11},
	}}
	inverse := aggragates.Trades{Inverse: true, History: []aggragates.TradesHistory{
		{Type: "SELL", Quantity: 1, Price: 100, OrderId: 1},
		{Type: "SELL", Quantity: 2, Price: 102.6, OrderId: 2},
	}}

	// The resting add's own row carries no quantity until it fills.
	resting := aggragates.Trades{
		PositionType:  "buy",
		PositionPrice: 0.0732,
		PendingOrder:  6,
		History: []aggragates.TradesHistory{
			{Type: "BUY", Quantity: 100, Price: 0.2367, OrderId: 4},
			{Type: "BUY", Quantity: 200, Price: 0.21482, OrderId: 5},
			{Type: "BUY", Quantity: 0, Price: 0.0732, OrderId: 6},
		},
	}
	settled := resting
	settled.PendingOrder = 0
	selling := resting
	selling.PositionType = "sell"
	inverseResting := inverse
	inverseResting.PositionType = "buy"
	inverseResting.PendingOrder = 3

	cases := []struct {
		name     string
		trade    aggragates.Trades
		position string
		price    float64
		want     float64
	}{
		{"a long re-anchor over the last entry stays at it", long, "update_buy", 2286.96, 2267.75},
		{"a long re-anchor under the last entry takes the print", long, "update_buy", 2250, 2250},
		{"an inverse re-anchor under the last entry stays at it", inverse, "update_buy", 101, 102.6},
		{"an inverse re-anchor over the last entry takes the print", inverse, "update_buy", 104, 104},
		{"a ladder with no entry takes the print", aggragates.Trades{}, "update_buy", 2286.96, 2286.96},
		{"an entry takes the print", long, "buy", 2286.96, 2286.96},
		{"an arming takes the print", long, "trailingBuy", 2286.96, 2286.96},
		{"a take profit takes the print", long, "update_takeProfit", 2346.46, 2346.46},
		{"a runaway resting add re-anchors at the last entry", resting, "update_buy", 0.08134, 0.21482},
		{"a resting add with a print over the last entry re-anchors at it", resting, "update_buy", 0.25, 0.21482},
		{"a take profit revert with a pending order keeps the cap", selling, "update_buy", 0.2, 0.2},
		{"a buy with no pending order keeps the cap", settled, "update_buy", 0.08134, 0.08134},
		{"an inverse resting add re-anchors at its last entry", inverseResting, "update_buy", 104, 102.6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NextPositionPrice(tc.trade, tc.position, tc.price); got != tc.want {
				t.Fatalf("NextPositionPrice(%s, %v) = %v, want %v", tc.position, tc.price, got, tc.want)
			}
		})
	}
}
