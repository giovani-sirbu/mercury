package aggregates

// CommissionRates is the maker/taker/buyer/seller commission breakdown that
// Binance returns on /account.
type CommissionRates struct {
	Maker  string `json:"maker"`
	Taker  string `json:"taker"`
	Buyer  string `json:"buyer"`
	Seller string `json:"seller"`
}

// Account is the profile returned by GetProfile.
type Account struct {
	MakerCommission  int64             `json:"makerCommission"`
	TakerCommission  int64             `json:"takerCommission"`
	BuyerCommission  int64             `json:"buyerCommission"`
	SellerCommission int64             `json:"sellerCommission"`
	CommissionRates  CommissionRates   `json:"commissionRates"`
	CanTrade         bool              `json:"canTrade"`
	CanWithdraw      bool              `json:"canWithdraw"`
	CanDeposit       bool              `json:"canDeposit"`
	UpdateTime       uint64            `json:"updateTime"`
	AccountType      string            `json:"accountType"`
	Permissions      []string          `json:"permissions"`
	Balances         []UserAssetRecord `json:"balances"`
}

// UserAssetRecord is a single balance row.
type UserAssetRecord struct {
	Asset        string `json:"asset"`
	Free         string `json:"free"`
	Locked       string `json:"locked"`
	Freeze       string `json:"freeze"`
	Withdrawing  string `json:"withdrawing"`
	Ipoable      string `json:"ipoable"`
	BtcValuation string `json:"btcValuation"`
}

// APIKeyPermission is the permission set attached to an API key. Handlers
// should check EnableSpotAndMarginTrading before placing orders.
type APIKeyPermission struct {
	IPRestrict                     bool   `json:"ipRestrict"`
	CreateTime                     uint64 `json:"createTime"`
	EnableWithdrawals              bool   `json:"enableWithdrawals"`
	EnableInternalTransfer         bool   `json:"enableInternalTransfer"`
	PermitsUniversalTransfer       bool   `json:"permitsUniversalTransfer"`
	EnableVanillaOptions           bool   `json:"enableVanillaOptions"`
	EnableReading                  bool   `json:"enableReading"`
	EnableFutures                  bool   `json:"enableFutures"`
	EnableMargin                   bool   `json:"enableMargin"`
	EnableSpotAndMarginTrading     bool   `json:"enableSpotAndMarginTrading"`
	TradingAuthorityExpirationTime uint64 `json:"tradingAuthorityExpirationTime"`
}
