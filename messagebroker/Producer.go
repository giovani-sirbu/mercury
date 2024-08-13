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

func (m MessageBroker) Producer() *Producer {
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

	w := &kafka.Writer{
		Addr:         kafka.TCP(m.Address[0]),
		BatchTimeout: m.Timeout,
		Transport: &kafka.Transport{
			TLS: &tls.Config{
				Certificates: []tls.Certificate{keypair},
				RootCAs:      caCertPool,
			},
		},
		AllowAutoTopicCreation: true,
	}

	return &Producer{Writer: w}
}

// Produce messages
func (m MessageBroker) Produce(topic string, key, value []byte, producer *Producer) (err error) {
	topicWithPrefix := fmt.Sprintf("%s%s", os.Getenv("TOPIC_PREFIX"), topic)

	// Define messages
	msg := kafka.Message{
		Topic: topicWithPrefix,
		Key:   key,
		Value: value,
		Time:  time.Now(),
	}

	commonLog.Info(fmt.Sprintf("Produced on topic: %s", topicWithPrefix), "", "Producer")

	// Return message response
	err = producer.Writer.WriteMessages(context.TODO(), msg)
	return err
}
