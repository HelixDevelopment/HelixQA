package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"go.uber.org/zap"
)

func startTestNATSServer(t *testing.T) *server.Server {
	t.Helper()
	opts := &server.Options{
		Host:       "127.0.0.1",
		Port:       -1,
		NoLog:      true,
		NoSigs:     true,
		JetStream:  true,
		StoreDir:   t.TempDir(),
		ServerName: "test-server",
	}
	s, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("Failed to create NATS server: %v", err)
	}
	go s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}
	t.Cleanup(func() {
		s.Shutdown()
		s.WaitForShutdown()
	})
	return s
}

func TestEvent_Struct(t *testing.T) {
	event := &Event{
		Type:      "payment.created",
		Source:    "payment-service",
		Data:      map[string]string{"id": "123"},
		Timestamp: time.Now().UnixMilli(),
	}

	if event.Type != "payment.created" {
		t.Errorf("Type = %q, want payment.created", event.Type)
	}
	if event.Source != "payment-service" {
		t.Errorf("Source = %q, want payment-service", event.Source)
	}
	if event.Timestamp == 0 {
		t.Error("Timestamp should not be zero")
	}
}

func TestEvent_JSON(t *testing.T) {
	event := &Event{
		Type:      "payment.created",
		Source:    "payment-service",
		Data:      map[string]string{"id": "123"},
		Timestamp: time.Now().UnixMilli(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if decoded.Type != "payment.created" {
		t.Errorf("decoded Type = %q, want payment.created", decoded.Type)
	}
	if decoded.Source != "payment-service" {
		t.Errorf("decoded Source = %q, want payment-service", decoded.Source)
	}
}

func TestEvent_NewEvent(t *testing.T) {
	event := &Event{
		Type:   "test.event",
		Source: "test-source",
		Data:   "test-data",
	}

	if event.Type != "test.event" {
		t.Errorf("Type = %q", event.Type)
	}
}

func TestEvent_NilData(t *testing.T) {
	event := &Event{
		Type:   "test.event",
		Source: "test-source",
		Data:   nil,
	}

	if event.Data != nil {
		t.Errorf("Data should be nil, got %v", event.Data)
	}
}

func TestEvent_JSONNilData(t *testing.T) {
	event := &Event{
		Type:      "test.event",
		Source:    "test-source",
		Data:      nil,
		Timestamp: 1234567890,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if decoded.Data != nil {
		t.Errorf("decoded Data should be nil, got %v", decoded.Data)
	}
}

func TestEvent_JSONRoundTrip(t *testing.T) {
	event := &Event{
		Type:      "order.completed",
		Source:    "order-service",
		Data:      []string{"item1", "item2"},
		Timestamp: time.Now().UnixMilli(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if decoded.Type != event.Type {
		t.Errorf("Type mismatch: got %q, want %q", decoded.Type, event.Type)
	}
	if decoded.Source != event.Source {
		t.Errorf("Source mismatch: got %q, want %q", decoded.Source, event.Source)
	}
	if decoded.Timestamp != event.Timestamp {
		t.Errorf("Timestamp mismatch: got %d, want %d", decoded.Timestamp, event.Timestamp)
	}
}

func TestEvent_JSONEmptyData(t *testing.T) {
	event := &Event{
		Type:      "test.event",
		Source:    "test-source",
		Data:      "",
		Timestamp: 100,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if decoded.Data != "" {
		t.Errorf("decoded Data should be empty string, got %v", decoded.Data)
	}
}

func TestEvent_JSONMapData(t *testing.T) {
	event := &Event{
		Type:   "test.event",
		Source: "test-source",
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": float64(42),
			"key3": true,
		},
		Timestamp: 200,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	decodedMap, ok := decoded.Data.(map[string]interface{})
	if !ok {
		t.Fatal("decoded Data is not a map")
	}
	if decodedMap["key1"] != "value1" {
		t.Errorf("key1 = %v, want value1", decodedMap["key1"])
	}
}

func TestEvent_JSONInvalidJSON(t *testing.T) {
	var event Event
	if err := json.Unmarshal([]byte("not json"), &event); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestNewNatsEventBus_ConnectSuccess(t *testing.T) {
	natsServer := startTestNATSServer(t)
	logger := zap.NewNop()

	bus, err := NewNatsEventBus(natsServer.ClientURL(), logger)
	if err != nil {
		t.Fatalf("NewNatsEventBus failed: %v", err)
	}
	defer bus.Close()

	if bus.conn == nil {
		t.Error("conn should not be nil")
	}
	if bus.js == nil {
		t.Error("js should not be nil")
	}
	if bus.logger == nil {
		t.Error("logger should not be nil")
	}
}

func TestNatsEventBus_Close_NilConn(t *testing.T) {
	bus := &NatsEventBus{conn: nil}
	if err := bus.Close(); err != nil {
		t.Errorf("Close() on nil conn should return nil, got %v", err)
	}
}

func TestNatsEventBus_Close_WithConn(t *testing.T) {
	natsServer := startTestNATSServer(t)
	logger := zap.NewNop()

	bus, err := NewNatsEventBus(natsServer.ClientURL(), logger)
	if err != nil {
		t.Fatalf("NewNatsEventBus failed: %v", err)
	}

	if err := bus.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}
}

func TestNatsEventBus_PublishSubscribe(t *testing.T) {
	natsServer := startTestNATSServer(t)
	logger := zap.NewNop()

	bus, err := NewNatsEventBus(natsServer.ClientURL(), logger)
	if err != nil {
		t.Fatalf("NewNatsEventBus failed: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()
	received := make(chan *Event, 1)

	err = bus.Subscribe(ctx, "events.test.>", func(event *Event) error {
		received <- event
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	event := &Event{
		Type:   "test.event",
		Source: "test-source",
		Data:   map[string]string{"key": "value"},
	}

	if err := bus.Publish(ctx, "events.test.publish", event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	select {
	case got := <-received:
		if got.Type != "test.event" {
			t.Errorf("received Type = %q, want test.event", got.Type)
		}
		if got.Source != "test-source" {
			t.Errorf("received Source = %q, want test-source", got.Source)
		}
		if got.Timestamp == 0 {
			t.Error("received Timestamp should not be zero")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestNatsEventBus_SubscribeMultiple(t *testing.T) {
	natsServer := startTestNATSServer(t)
	logger := zap.NewNop()

	bus, err := NewNatsEventBus(natsServer.ClientURL(), logger)
	if err != nil {
		t.Fatalf("NewNatsEventBus failed: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()
	var mu sync.Mutex
	received := make(map[string]bool)

	err = bus.Subscribe(ctx, "events.multi.>", func(event *Event) error {
		mu.Lock()
		received[event.Type] = true
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	events := []*Event{
		{Type: "event.a", Source: "src"},
		{Type: "event.b", Source: "src"},
		{Type: "event.c", Source: "src"},
	}

	for _, e := range events {
		if err := bus.Publish(ctx, "events.multi."+e.Type, e); err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
	}

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 3 {
		t.Errorf("received %d events, want 3", len(received))
	}
	for _, e := range events {
		if !received[e.Type] {
			t.Errorf("event %q not received", e.Type)
		}
	}
}

func TestNatsEventBus_PublishTimestamp(t *testing.T) {
	natsServer := startTestNATSServer(t)
	logger := zap.NewNop()

	bus, err := NewNatsEventBus(natsServer.ClientURL(), logger)
	if err != nil {
		t.Fatalf("NewNatsEventBus failed: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()
	received := make(chan *Event, 1)

	err = bus.Subscribe(ctx, "events.ts.>", func(event *Event) error {
		received <- event
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	before := time.Now().UnixMilli()
	event := &Event{Type: "ts.test", Source: "src"}
	if err := bus.Publish(ctx, "events.ts.test", event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	after := time.Now().UnixMilli()

	select {
	case got := <-received:
		if got.Timestamp < before || got.Timestamp > after {
			t.Errorf("Timestamp %d not in range [%d, %d]", got.Timestamp, before, after)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestNatsEventBus_PublishNilData(t *testing.T) {
	natsServer := startTestNATSServer(t)
	logger := zap.NewNop()

	bus, err := NewNatsEventBus(natsServer.ClientURL(), logger)
	if err != nil {
		t.Fatalf("NewNatsEventBus failed: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()
	received := make(chan *Event, 1)

	err = bus.Subscribe(ctx, "events.nil.>", func(event *Event) error {
		received <- event
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	event := &Event{Type: "nil.test", Source: "src", Data: nil}
	if err := bus.Publish(ctx, "events.nil.test", event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	select {
	case got := <-received:
		if got.Data != nil {
			t.Errorf("received Data should be nil, got %v", got.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestNatsEventBus_PublishComplexData(t *testing.T) {
	natsServer := startTestNATSServer(t)
	logger := zap.NewNop()

	bus, err := NewNatsEventBus(natsServer.ClientURL(), logger)
	if err != nil {
		t.Fatalf("NewNatsEventBus failed: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()
	received := make(chan *Event, 1)

	err = bus.Subscribe(ctx, "events.complex.>", func(event *Event) error {
		received <- event
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	data := map[string]interface{}{
		"id":     12345,
		"amount": 99.99,
		"items":  []string{"a", "b", "c"},
		"nested": map[string]string{"key": "val"},
	}
	event := &Event{Type: "complex.test", Source: "src", Data: data}
	if err := bus.Publish(ctx, "events.complex.test", event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	select {
	case got := <-received:
		gotData, ok := got.Data.(map[string]interface{})
		if !ok {
			t.Fatal("received Data is not a map")
		}
		if gotData["id"] != float64(12345) {
			t.Errorf("id = %v, want 12345", gotData["id"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestNatsEventBus_ConcurrentPublishSubscribe(t *testing.T) {
	natsServer := startTestNATSServer(t)
	logger := zap.NewNop()

	bus, err := NewNatsEventBus(natsServer.ClientURL(), logger)
	if err != nil {
		t.Fatalf("NewNatsEventBus failed: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()
	var mu sync.Mutex
	received := make(map[string]bool)

	err = bus.Subscribe(ctx, "events.concurrent.>", func(event *Event) error {
		mu.Lock()
		received[event.Type] = true
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	var wg sync.WaitGroup
	numEvents := 10
	for i := 0; i < numEvents; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			event := &Event{
				Type:   "concurrent.event",
				Source: "src",
				Data:   map[string]int{"idx": idx},
			}
			if err := bus.Publish(ctx, "events.concurrent.test", event); err != nil {
				t.Errorf("Publish failed: %v", err)
			}
		}(i)
	}
	wg.Wait()

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Errorf("received %d distinct event types, want 1", len(received))
	}
}

func TestNatsEventBus_SubscribeHandlerError(t *testing.T) {
	natsServer := startTestNATSServer(t)
	logger := zap.NewNop()

	bus, err := NewNatsEventBus(natsServer.ClientURL(), logger)
	if err != nil {
		t.Fatalf("NewNatsEventBus failed: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()
	handlerErr := errors.New("handler error")

	err = bus.Subscribe(ctx, "events.handler.>", func(event *Event) error {
		return handlerErr
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	event := &Event{Type: "handler.test", Source: "src"}
	if err := bus.Publish(ctx, "events.handler.test", event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
}

func TestNatsEventBus_PublishAfterClose(t *testing.T) {
	natsServer := startTestNATSServer(t)
	logger := zap.NewNop()

	bus, err := NewNatsEventBus(natsServer.ClientURL(), logger)
	if err != nil {
		t.Fatalf("NewNatsEventBus failed: %v", err)
	}

	bus.Close()

	ctx := context.Background()
	event := &Event{Type: "closed.test", Source: "src"}
	if err := bus.Publish(ctx, "events.closed.test", event); err == nil {
		t.Error("expected error publishing after close")
	}
}

func TestNatsEventBus_SubscribeAfterClose(t *testing.T) {
	natsServer := startTestNATSServer(t)
	logger := zap.NewNop()

	bus, err := NewNatsEventBus(natsServer.ClientURL(), logger)
	if err != nil {
		t.Fatalf("NewNatsEventBus failed: %v", err)
	}

	bus.Close()

	ctx := context.Background()
	if err := bus.Subscribe(ctx, "events.closed.>", func(event *Event) error {
		return nil
	}); err == nil {
		t.Error("expected error subscribing after close")
	}
}

func TestNatsEventBus_SubscribeInvalidJSON(t *testing.T) {
	natsServer := startTestNATSServer(t)
	logger := zap.NewNop()

	bus, err := NewNatsEventBus(natsServer.ClientURL(), logger)
	if err != nil {
		t.Fatalf("NewNatsEventBus failed: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()
	received := make(chan *Event, 1)

	err = bus.Subscribe(ctx, "events.invalidjson.>", func(event *Event) error {
		received <- event
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish invalid JSON via raw NATS connection
	if bus.conn != nil {
		bus.conn.Publish("events.invalidjson.test", []byte("not valid json"))
	}

	// Should not receive any event since unmarshal fails
	select {
	case <-received:
		t.Error("should not receive event from invalid JSON")
	case <-time.After(200 * time.Millisecond):
		// Expected: no event received
	}
}

type unmarshalableData struct {
	ch chan int
}

func TestNatsEventBus_PublishMarshalError(t *testing.T) {
	natsServer := startTestNATSServer(t)
	logger := zap.NewNop()

	bus, err := NewNatsEventBus(natsServer.ClientURL(), logger)
	if err != nil {
		t.Fatalf("NewNatsEventBus failed: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()

	event := &Event{
		Type:   "marshal.error",
		Source: "src",
		Data:   map[string]interface{}{"val": math.NaN()},
	}

	err = bus.Publish(ctx, "events.marshal.error", event)
	if err == nil {
		t.Error("expected error marshaling event with NaN data")
	}
}

func TestNatsEventBus_PublishInfinityError(t *testing.T) {
	natsServer := startTestNATSServer(t)
	logger := zap.NewNop()

	bus, err := NewNatsEventBus(natsServer.ClientURL(), logger)
	if err != nil {
		t.Fatalf("NewNatsEventBus failed: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()

	event := &Event{
		Type:   "marshal.error",
		Source: "src",
		Data:   map[string]interface{}{"val": math.Inf(1)},
	}

	err = bus.Publish(ctx, "events.marshal.error", event)
	if err == nil {
		t.Error("expected error marshaling event with Inf data")
	}
}

func TestNatsEventBus_MarshalJSON(t *testing.T) {
	event := &Event{
		Type:      "test.type",
		Source:    "test-source",
		Data:      "test-data",
		Timestamp: 1234567890,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Type != event.Type {
		t.Errorf("Type = %q, want %q", decoded.Type, event.Type)
	}
	if decoded.Source != event.Source {
		t.Errorf("Source = %q, want %q", decoded.Source, event.Source)
	}
	if decoded.Timestamp != event.Timestamp {
		t.Errorf("Timestamp = %d, want %d", decoded.Timestamp, event.Timestamp)
	}
}

func TestEvent_EmptyEvent(t *testing.T) {
	event := &Event{}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Type != "" {
		t.Errorf("Type should be empty, got %q", decoded.Type)
	}
	if decoded.Source != "" {
		t.Errorf("Source should be empty, got %q", decoded.Source)
	}
	if decoded.Timestamp != 0 {
		t.Errorf("Timestamp should be 0, got %d", decoded.Timestamp)
	}
}

func TestEvent_JSONTags(t *testing.T) {
	event := &Event{
		Type:      "tag.test",
		Source:    "src",
		Data:      "data",
		Timestamp: 42,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal raw failed: %v", err)
	}

	if raw["type"] != "tag.test" {
		t.Errorf("JSON key 'type' = %v, want 'tag.test'", raw["type"])
	}
	if raw["source"] != "src" {
		t.Errorf("JSON key 'source' = %v, want 'src'", raw["source"])
	}
	if raw["data"] != "data" {
		t.Errorf("JSON key 'data' = %v, want 'data'", raw["data"])
	}
	if raw["timestamp"] != float64(42) {
		t.Errorf("JSON key 'timestamp' = %v, want 42", raw["timestamp"])
	}
}

func TestEvent_ImplementsEventBusInterface(t *testing.T) {
	var _ EventBus = (*NatsEventBus)(nil)
}


