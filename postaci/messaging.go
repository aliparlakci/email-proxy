package main

import (
	"context"
	"crypto/tls"
	"log"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/scram"
)

type MessageBroker struct {
	kafkaAddress    string
	sharedTransport *kafka.Transport
}

type MessageProducer interface {
	Produce(ctx context.Context, topic, message string) error
}

func (mb *MessageBroker) Initialize(kafkaAddress, kafkaUsername, kafkaPassword string) {
	mb.kafkaAddress = kafkaAddress
	mechanism, err := scram.Mechanism(scram.SHA256, kafkaUsername, kafkaPassword)
	if err != nil {
		log.Fatalln(err)
	}

	mb.sharedTransport = &kafka.Transport{
		SASL: mechanism,
		TLS:  &tls.Config{},
	}
}

func (mb *MessageBroker) Produce(ctx context.Context, topic, message string) error {
	w := kafka.Writer{
		Addr:      kafka.TCP(mb.kafkaAddress),
		Topic:     topic,
		Balancer:  &kafka.Hash{},
		Transport: mb.sharedTransport,
	}

	err := w.WriteMessages(ctx, kafka.Message{
		Key:   nil,
		Value: []byte(message),
	})

	return err
}
