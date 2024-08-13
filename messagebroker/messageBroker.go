package messagebroker

import (
	"github.com/segmentio/kafka-go"
	"time"
)

type MessageBroker struct {
	CertsPath string
	Address   []string
	Timeout   time.Duration
}

type Producer struct {
	Writer *kafka.Writer
}
