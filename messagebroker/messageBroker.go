package messagebroker

import "time"

type MessageBroker struct {
	CertsPath string
	Address   []string
	Timeout   time.Duration
}
