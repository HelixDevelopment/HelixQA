package distributed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: These tests require a running NATS server
// For unit tests without NATS, use mocks

func TestProcessingStatus_String(t *testing.T) {
	tests := []struct {
		status   ProcessingStatus
		expected string
	}{
		{StatusPending, "pending"},
		{StatusCapturing, "capturing"},
		{StatusProcessing, "processing"},
		{StatusAnalyzing, "analyzing"},
		{StatusComplete, "complete"},
		{StatusFailed, "failed"},
		{StatusTimeout, "timeout"},
		{ProcessingStatus(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestFrameProcessingState(t *testing.T) {
	state := &FrameProcessingState{
		FrameID:   "frame-001",
		Timestamp: time.Now(),
		HostID:    "host-test",
		Platform:  "android",
		Status:    StatusProcessing,
		Elements: []UIElement{
			{
				ID:         "btn-1",
				Type:       "button",
				Bounds:     Bounds{X: 100, Y: 200, Width: 80, Height: 40},
				Confidence: 0.95,
				Text:       "Submit",
			},
		},
		TextBlocks: []TextBlock{
			{
				Text:       "Welcome",
				Bounds:     Bounds{X: 50, Y: 100, Width: 200, Height: 30},
				Confidence: 0.98,
			},
		},
		LatencyMs: 45.5,
	}

	assert.Equal(t, "frame-001", state.FrameID)
	assert.Equal(t, "host-test", state.HostID)
	assert.Equal(t, "android", state.Platform)
	assert.Equal(t, StatusProcessing, state.Status)
	assert.Len(t, state.Elements, 1)
	assert.Len(t, state.TextBlocks, 1)
	assert.InDelta(t, 45.5, state.LatencyMs, 0.01)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, "nats://localhost:4222", config.NATSURL)
	assert.NotEmpty(t, config.HostID)
	assert.Equal(t, "HELIXQA_FRAMES", config.StreamName)
	assert.Equal(t, "FRAME_STATE", config.KVBucket)
}

func TestStats(t *testing.T) {
	stats := &Stats{
		TotalFrames:      100,
		ByStatus:         map[string]int{"complete": 80, "failed": 20},
		ByPlatform:       map[string]int{"android": 60, "desktop": 40},
		AverageLatencyMs: 50.5,
	}

	assert.Equal(t, 100, stats.TotalFrames)
	assert.Equal(t, 80, stats.ByStatus["complete"])
	assert.Equal(t, 20, stats.ByStatus["failed"])
	assert.Equal(t, 60, stats.ByPlatform["android"])
	assert.InDelta(t, 50.5, stats.AverageLatencyMs, 0.01)
}

// Mock implementations for testing without NATS

type MockKV struct {
	data map[string][]byte
	mu   sync.RWMutex
}

func NewMockKV() *MockKV {
	return &MockKV{
		data: make(map[string][]byte),
	}
}

func (m *MockKV) Put(ctx context.Context, key string, value []byte) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return 1, nil
}

func (m *MockKV) Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	value, ok := m.data[key]
	if !ok {
		return nil, errors.New("key not found")
	}
	
	return &mockEntry{
		key:       key,
		value:     value,
		revision:  1,
		created:   time.Now(),
		operation: jetstream.KeyValuePut,
	}, nil
}

func (m *MockKV) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *MockKV) Keys(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	keys := make([]string, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	return keys, nil
}

type mockEntry struct {
	key       string
	value     []byte
	revision  uint64
	created   time.Time
	operation jetstream.KeyValueOp
}

func (m *mockEntry) Bucket() string        { return "TEST_BUCKET" }
func (m *mockEntry) Key() string           { return m.key }
func (m *mockEntry) Value() []byte         { return m.value }
func (m *mockEntry) Revision() uint64      { return m.revision }
func (m *mockEntry) Created() time.Time    { return m.created }
func (m *mockEntry) Delta() uint64         { return 0 }
func (m *mockEntry) Operation() jetstream.KeyValueOp { return m.operation }

// These tests require NATS to be running
// Skip if NATS is not available

func skipIfNoNATS(t *testing.T) {
	config := DefaultConfig()
	_, err := NewStateManager(config)
	if err != nil {
		t.Skip("NATS not available, skipping integration test")
	}
}

func TestStateManager_Integration(t *testing.T) {
	skipIfNoNATS(t)

	config := DefaultConfig()
	config.HostID = "test-host-" + time.Now().Format("20060102150405")

	sm, err := NewStateManager(config)
	require.NoError(t, err)
	defer sm.Close()

	ctx := context.Background()

	// Test publishing frame state
	state := &FrameProcessingState{
		FrameID:  "test-frame-001",
		Platform: "android",
		Status:   StatusProcessing,
	}

	err = sm.PublishFrameState(ctx, state)
	require.NoError(t, err)

	// Test retrieving frame state
	retrieved, err := sm.GetFrameState(ctx, "test-frame-001")
	require.NoError(t, err)
	assert.Equal(t, state.FrameID, retrieved.FrameID)
	assert.Equal(t, state.Platform, retrieved.Platform)
	assert.Equal(t, config.HostID, retrieved.HostID) // Should be populated

	// Test updating status
	err = sm.UpdateFrameStatus(ctx, "test-frame-001", StatusComplete)
	require.NoError(t, err)

	updated, err := sm.GetFrameState(ctx, "test-frame-001")
	require.NoError(t, err)
	assert.Equal(t, StatusComplete, updated.Status)

	// Test deletion
	err = sm.DeleteFrameState(ctx, "test-frame-001")
	require.NoError(t, err)

	_, err = sm.GetFrameState(ctx, "test-frame-001")
	assert.Error(t, err) // Should not exist
}

func TestStateManager_ListFrames(t *testing.T) {
	skipIfNoNATS(t)

	config := DefaultConfig()
	config.HostID = "test-host-list"

	sm, err := NewStateManager(config)
	require.NoError(t, err)
	defer sm.Close()

	ctx := context.Background()

	// Create test frames
	platforms := []string{"android", "desktop", "web"}
	for i, platform := range platforms {
		state := &FrameProcessingState{
			FrameID:  fmt.Sprintf("list-frame-%d", i),
			Platform: platform,
			Status:   StatusComplete,
		}
		err := sm.PublishFrameState(ctx, state)
		require.NoError(t, err)
	}

	// Test listing all frames
	frames, err := sm.ListFrames(ctx, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(frames), 3)

	// Test filtering by platform
	androidFrames, err := sm.ListFrames(ctx, "android")
	require.NoError(t, err)
	for _, f := range androidFrames {
		assert.Equal(t, "android", f.Platform)
	}

	// Cleanup
	for i := 0; i < len(platforms); i++ {
		sm.DeleteFrameState(ctx, fmt.Sprintf("list-frame-%d", i))
	}
}

// Benchmarks

func BenchmarkFrameProcessingState_Marshal(b *testing.B) {
	state := &FrameProcessingState{
		FrameID:  "bench-frame",
		Platform: "android",
		Status:   StatusComplete,
		Elements: make([]UIElement, 10),
	}

	for i := 0; i < 10; i++ {
		state.Elements[i] = UIElement{
			ID:         fmt.Sprintf("elem-%d", i),
			Type:       "button",
			Bounds:     Bounds{X: i * 10, Y: i * 20, Width: 100, Height: 50},
			Confidence: 0.95,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(state)
	}
}

func BenchmarkFrameProcessingState_Unmarshal(b *testing.B) {
	state := &FrameProcessingState{
		FrameID:  "bench-frame",
		Platform: "android",
		Status:   StatusComplete,
		Elements: make([]UIElement, 10),
	}

	data, _ := json.Marshal(state)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var s FrameProcessingState
		_ = json.Unmarshal(data, &s)
	}
}
