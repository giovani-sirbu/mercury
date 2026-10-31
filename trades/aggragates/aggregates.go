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
	// Hold is an open trade a gate holds: active in every respect but the status the user reads.
	Hold Status = "hold"
)

// OpenStatuses are the statuses of a trade the engines tick: active and held.
func OpenStatuses() []Status {
	return []Status{Active, Hold}
}

// IsOpen is whether the status is one of OpenStatuses.
func (status Status) IsOpen() bool {
	return status == Active || status == Hold
}

// SettledStatus is the status a tick leaves on a trade found in status: held, hold over an open
// status; not held, active over hold; blocked, paused, closed and "" kept.
func SettledStatus(status Status, held bool) Status {
	if !status.IsOpen() {
		return status
	}
	if held {
		return Hold
	}
	return Active
}

const (
	Spot    TradeTypes = "spot"
	Futures TradeTypes = "futures"
)
