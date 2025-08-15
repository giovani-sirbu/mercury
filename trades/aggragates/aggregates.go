package aggragates

type Status string
type TradeTypes string

const (
	Active     Status = "active"
	Blocked    Status = "blocked"
	Paused     Status = "paused"
	Closed     Status = "closed"
	Impasse    Status = "impasse"
	InPosition Status = "inPosition"
)

const (
	Spot    TradeTypes = "spot"
	Futures TradeTypes = "futures"
)
