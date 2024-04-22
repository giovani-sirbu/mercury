package aggragates

type Status string

const (
	Active  Status = "active"
	Blocked Status = "blocked"
	Paused  Status = "paused"
	Closed  Status = "closed"
	Impasse Status = "impasse"
)
