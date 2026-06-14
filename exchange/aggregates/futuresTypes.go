package aggregates

// FuturesBalance is the per-asset balance returned by the futures API.
type FuturesBalance struct {
	AccountAlias       string `json:"accountAlias"`
	Asset              string `json:"asset"`
	Balance            string `json:"balance"`
	CrossWalletBalance string `json:"crossWalletBalance"`
	CrossUnPnl         string `json:"crossUnPnl"`
	AvailableBalance   string `json:"availableBalance"`
	MaxWithdrawAmount  string `json:"maxWithdrawAmount"`
}

// IncomeHistory is a single entry in the futures income log (funding, PnL,
// commission, etc.).
type IncomeHistory struct {
	Asset      string `json:"asset"`
	Income     string `json:"income"`
	IncomeType string `json:"incomeType"`
	Info       string `json:"info"`
	Symbol     string `json:"symbol"`
	Time       int64  `json:"time"`
	TranID     int64  `json:"tranId"`
	TradeID    string `json:"tradeId"`
}

// PositionRisk is the open-position detail returned by /positionRisk.
type PositionRisk struct {
	EntryPrice       string `json:"entryPrice"`
	BreakEvenPrice   string `json:"breakEvenPrice"`
	MarginType       string `json:"marginType"`
	IsAutoAddMargin  string `json:"isAutoAddMargin"`
	IsolatedMargin   string `json:"isolatedMargin"`
	Leverage         string `json:"leverage"`
	LiquidationPrice string `json:"liquidationPrice"`
	MarkPrice        string `json:"markPrice"`
	MaxNotionalValue string `json:"maxNotionalValue"`
	PositionAmt      string `json:"positionAmt"`
	Symbol           string `json:"symbol"`
	UnRealizedProfit string `json:"unRealizedProfit"`
	PositionSide     string `json:"positionSide"`
	Notional         string `json:"notional"`
	IsolatedWallet   string `json:"isolatedWallet"`
}

// SymbolLeverage is returned after updating leverage on a futures symbol.
type SymbolLeverage struct {
	Leverage         int    `json:"leverage"`
	MaxNotionalValue string `json:"maxNotionalValue"`
	Symbol           string `json:"symbol"`
}
