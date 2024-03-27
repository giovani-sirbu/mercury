package messagebroker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	commonLog "github.com/giovani-sirbu/mercury/log"
	"github.com/segmentio/kafka-go"
	"os"
	"strings"
	"time"
)

// Handler response func type
type fn func([]byte)

// Consumer messages
func Consumer(topic string, handler fn) {
	brokerAddress := strings.Split(os.Getenv("BROKERS"), ",")

	// intialize the writer with the broker addresses, and the topic

	keypair, err := tls.LoadX509KeyPair("/app/common/service.cert", "/app/common/service.key")

	if err != nil {
		commonLog.Error(fmt.Sprintf("Failed to load Access Key and/or Access Certificate: %s", err), "", "Producer")
	}

	caCert, err := os.ReadFile("/app/common/ca.pem")
	if err != nil {
		commonLog.Error(fmt.Sprintf("Failed to read CA Certificate file: %s", err), "", "Producer")
	}

	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(caCert)

	if !ok {
		commonLog.Error(fmt.Sprintf("Failed to parse CA Certificate file: %s", err), "", "Producer")
	}

	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
		TLS: &tls.Config{
			Certificates: []tls.Certificate{keypair},
			RootCAs:      caCertPool,
		},
	}

	topicWithPrefix := fmt.Sprintf("%s-%s", os.Getenv("TOPIC_PREFIX"), topic)
	commonLog.Info(fmt.Sprintf("Consumer started on topic: %s, on brokers: %s", topicWithPrefix, os.Getenv("BROKERS")), "", "Consumer")

	// initialize a new reader with the brokers and topic
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokerAddress,   // Broker list
		Topic:    topicWithPrefix, // Topic to consume
		MinBytes: 1,
		MaxBytes: 57671680,
		Dialer:   dialer,
	})

	for {
		// read message
		ctx := context.Background()
		m, err := reader.ReadMessage(ctx)
		defer ctx.Done()

		if err != nil {
			commonLog.Error(err.Error(), "", "Consumer")
			continue
		}

		// define value
		value := m.Value

		commonLog.Info(fmt.Sprintf("Consumed on topic: %s", topic), "", "Consumer")

		// Handle response callback
		go handler(value)
	}
}
