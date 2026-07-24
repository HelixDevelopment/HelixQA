package eventbus

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type Event struct {
	Type      string      `json:"type"`
	Source    string      `json:"source"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

type EventBus interface {
	Publish(ctx context.Context, subject string, event *Event) error
	Subscribe(ctx context.Context, subject string, handler func(*Event) error) error
	Close() error
}

type NatsEventBus struct {
	conn   *nats.Conn
	js     nats.JetStreamContext
	logger *zap.Logger
}

func NewNatsEventBus(natsURL string, logger *zap.Logger) (*NatsEventBus, error) {
	conn, err := nats.Connect(natsURL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, err
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Ensure stream exists
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "EVENTS",
		Subjects: []string{"events.>"},
	})
	if err != nil {
		logger.Warn("stream may already exist", zap.Error(err))
	}

	return &NatsEventBus{conn: conn, js: js, logger: logger}, nil
}

func (b *NatsEventBus) Publish(ctx context.Context, subject string, event *Event) error {
	event.Timestamp = time.Now().UnixMilli()
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = b.js.Publish(subject, data)
	return err
}

func (b *NatsEventBus) Subscribe(ctx context.Context, subject string, handler func(*Event) error) error {
	_, err := b.js.Subscribe(subject, func(m *nats.Msg) {
		var event Event
		if err := json.Unmarshal(m.Data, &event); err != nil {
			b.logger.Error("failed to unmarshal event", zap.Error(err))
			return
		}
		if err := handler(&event); err != nil {
			b.logger.Error("handler failed", zap.Error(err))
		}
		m.Ack()
	}, nats.Durable("helix-seller"), nats.ManualAck())
	return err
}

func (b *NatsEventBus) Close() error {
	if b.conn != nil {
		b.conn.Close()
	}
	return nil
}

type NoopEventBus struct{}

func (b *NoopEventBus) Publish(ctx context.Context, subject string, event *Event) error {
	return nil
}

func (b *NoopEventBus) Subscribe(ctx context.Context, subject string, handler func(*Event) error) error {
	return nil
}

func (b *NoopEventBus) Close() error {
	return nil
}
