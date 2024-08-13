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

type BrokerMethods struct {
	Producer *Producer
	Produce  func(topic string, key, value []byte, producer *Producer) error
	Consumer func(topic string, handler fn)
}

func (broker MessageBroker) Init() BrokerMethods {
	producer := broker.Producer()
	return BrokerMethods{
		Producer: producer,
		Produce:  broker.Produce,
		Consumer: broker.Consumer,
	}
}
