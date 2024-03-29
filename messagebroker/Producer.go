package messagebroker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	commonLog "github.com/giovani-sirbu/mercury/log"
	"github.com/segmentio/kafka-go"
	"os"
	"time"
)

// Produce messages
func (m MessageBroker) Producer(topic string, parent context.Context, key, value []byte) (err error) {
	// intialize the writer with the broker addresses, and the topic
	serviceCert := fmt.Sprintf("%s/service.cert", m.CertsPath)
	serviceKey := fmt.Sprintf("%s/service.key", m.CertsPath)
	caCerts := fmt.Sprintf("%s/ca.pem", m.CertsPath)

	keypair, err := tls.LoadX509KeyPair(serviceCert, serviceKey)

	if err != nil {
		commonLog.Error(fmt.Sprintf("Failed to load Access Key and/or Access Certificate: %s", err), "", "Producer")
	}

	caCert, err := os.ReadFile(caCerts)
	if err != nil {
		commonLog.Error(fmt.Sprintf("Failed to read CA Certificate file: %s", err), "", "Producer")
	}

	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(caCert)

	if !ok {
		commonLog.Error(fmt.Sprintf("Failed to parse CA Certificate file: %s", err), "", "Producer")
	}

	topicWithPrefix := fmt.Sprintf("%s%s", os.Getenv("TOPIC_PREFIX"), topic)

	w := &kafka.Writer{
		Addr:         kafka.TCP(m.Address[0]),
		Topic:        topicWithPrefix,
		BatchTimeout: m.Timeout,
		Transport: &kafka.Transport{
			TLS: &tls.Config{
				Certificates: []tls.Certificate{keypair},
				RootCAs:      caCertPool,
			},
		},
		AllowAutoTopicCreation: true,
	}

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
