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

// Produce messages
func Producer(topic string, parent context.Context, key, value []byte) (err error) {
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

	w := kafka.NewWriter(kafka.WriterConfig{
		Brokers: brokerAddress,
		Topic:   topicWithPrefix,
		Dialer:  dialer,
	})

	// Define messages
	msg := kafka.Message{
		Key:   key,
		Value: value,
		Time:  time.Now(),
	}

	commonLog.Info(fmt.Sprintf("Produced on topic: %s", topicWithPrefix), "", "Producer")

	defer w.Close()
	defer parent.Done()

	// Return message response
	return w.WriteMessages(parent, msg)

}
