package service

import (
	"encoding/json"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestBackgroundService_Constructor_Variants(t *testing.T) {
	tests := []struct {
		name         string
		workers      int
		pollInterval time.Duration
	}{
		{"single worker", 1, time.Second},
		{"multiple workers", 4, 5 * time.Second},
		{"zero workers", 0, time.Millisecond},
		{"large worker count", 100, 100 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewBackgroundService(nil, zap.NewNop(), tt.workers, tt.pollInterval)
			if svc == nil {
				t.Fatal("expected non-nil BackgroundService")
			}
			if svc.workers != tt.workers {
				t.Errorf("workers = %d, want %d", svc.workers, tt.workers)
			}
			if svc.pollInt != tt.pollInterval {
				t.Errorf("pollInt = %v, want %v", svc.pollInt, tt.pollInterval)
			}
		})
	}
}

func TestBackgroundService_Constructor_NilDB(t *testing.T) {
	svc := NewBackgroundService(nil, zap.NewNop(), 4, time.Second)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.db != nil {
		t.Error("db should be nil")
	}
}

func TestBackgroundService_Constructor_NilLogger(t *testing.T) {
	svc := NewBackgroundService(nil, nil, 4, time.Second)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.logger != nil {
		t.Error("logger should be nil")
	}
}

func TestBackgroundService_Enqueue_JSONMarshalError(t *testing.T) {
	svc := NewBackgroundService(nil, zap.NewNop(), 1, time.Second)

	// Channels cannot be marshaled to JSON - this tests the error path before DB access
	ch := make(chan int)
	_, err := svc.Enqueue(nil, "test", ch, 1)
	if err == nil {
		t.Error("expected error for non-marshalable payload")
	}
}

func TestBackgroundService_Enqueue_JSONMarshal(t *testing.T) {
	payload := map[string]interface{}{
		"type":   "reconciliation",
		"amount": 5000,
		"items":  []string{"a", "b", "c"},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded["type"] != "reconciliation" {
		t.Errorf("type = %v, want reconciliation", decoded["type"])
	}
}

func TestBackgroundService_Enqueue_JSONNilPayload(t *testing.T) {
	data, err := json.Marshal(nil)
	if err != nil {
		t.Fatalf("marshal nil error: %v", err)
	}
	if string(data) != "null" {
		t.Errorf("nil JSON = %q, want null", string(data))
	}
}

func TestBackgroundTaskRow_FieldValues(t *testing.T) {
	task := &backgroundTaskRow{
		Type:     "reconciliation",
		Priority: 5,
		Attempts: 3,
	}

	if task.Type != "reconciliation" {
		t.Errorf("Type = %q, want reconciliation", task.Type)
	}
	if task.Priority != 5 {
		t.Errorf("Priority = %d, want 5", task.Priority)
	}
	if task.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3", task.Attempts)
	}
}

func TestBackgroundTaskRow_JSON(t *testing.T) {
	task := &backgroundTaskRow{
		Type:     "webhook_delivery",
		Priority: 10,
		Attempts: 1,
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded backgroundTaskRow
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Type != "webhook_delivery" {
		t.Errorf("Type = %q, want webhook_delivery", decoded.Type)
	}
	if decoded.Priority != 10 {
		t.Errorf("Priority = %d, want 10", decoded.Priority)
	}
}

func TestBackgroundService_WorkerID(t *testing.T) {
	svc := NewBackgroundService(nil, zap.NewNop(), 4, time.Second)

	if svc.workers != 4 {
		t.Errorf("workers = %d, want 4", svc.workers)
	}
}

func TestBackgroundService_PollInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
	}{
		{"1 second", time.Second},
		{"5 seconds", 5 * time.Second},
		{"100ms", 100 * time.Millisecond},
		{"5 minutes", 5 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewBackgroundService(nil, zap.NewNop(), 1, tt.interval)
			if svc.pollInt != tt.interval {
				t.Errorf("pollInt = %v, want %v", svc.pollInt, tt.interval)
			}
		})
	}
}
