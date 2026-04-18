// Package distributed provides distributed state management for video processing
// across multiple hosts using NATS JetStream.
package distributed

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// FrameProcessingState represents the state of a frame being processed
type FrameProcessingState struct {
	FrameID    string           `json:"frame_id"`
	Timestamp  time.Time        `json:"timestamp"`
	HostID     string           `json:"host_id"`
	Platform   string           `json:"platform"`
	Status     ProcessingStatus `json:"status"`
	Elements   []UIElement      `json:"elements,omitempty"`
	TextBlocks []TextBlock      `json:"text_blocks,omitempty"`
	LLMResult  string           `json:"llm_result,omitempty"`
	Error      string           `json:"error,omitempty"`
	LatencyMs  float64          `json:"latency_ms"`
}

// ProcessingStatus represents the current state of processing
type ProcessingStatus int

const (
	StatusPending ProcessingStatus = iota
	StatusCapturing
	StatusProcessing
	StatusAnalyzing
	StatusComplete
	StatusFailed
	StatusTimeout
)

func (s ProcessingStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusCapturing:
		return "capturing"
	case StatusProcessing:
		return "processing"
	case StatusAnalyzing:
		return "analyzing"
	case StatusComplete:
		return "complete"
	case StatusFailed:
		return "failed"
	case StatusTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

// UIElement represents a detected UI element
type UIElement struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"` // button, textfield, image, etc.
	Bounds     Bounds  `json:"bounds"`
	Confidence float64 `json:"confidence"`
	Text       string  `json:"text,omitempty"`
}

// Bounds represents a rectangular region
type Bounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// TextBlock represents detected text
type TextBlock struct {
	Text       string  `json:"text"`
	Bounds     Bounds  `json:"bounds"`
	Confidence float64 `json:"confidence"`
}

// StateManager manages distributed state using NATS JetStream
type StateManager struct {
	nc     *nats.Conn
	js     jetstream.JetStream
	kv     jetstream.KeyValue
	stream jetstream.Stream

	hostID     string
	mu         sync.RWMutex
	localCache map[string]*FrameProcessingState

	// Subscriptions
	subs []jetstream.ConsumeContext
}

// StateManagerConfig configuration for state manager
type StateManagerConfig struct {
	NATSURL    string
	HostID     string
	StreamName string
	KVBucket   string
}

// DefaultConfig returns default configuration
func DefaultConfig() StateManagerConfig {
	return StateManagerConfig{
		NATSURL:    "nats://localhost:4222",
		HostID:     generateHostID(),
		StreamName: "HELIXQA_FRAMES",
		KVBucket:   "FRAME_STATE",
	}
}

func generateHostID() string {
	return fmt.Sprintf("host-%d", time.Now().UnixNano())
}

// NewStateManager creates a new distributed state manager
func NewStateManager(config StateManagerConfig) (*StateManager, error) {
	// Connect to NATS
	nc, err := nats.Connect(config.NATSURL,
		nats.Timeout(10*time.Second),
		nats.ReconnectWait(1*time.Second),
		nats.MaxReconnects(10),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream: %w", err)
	}

	sm := &StateManager{
		nc:         nc,
		js:         js,
		hostID:     config.HostID,
		localCache: make(map[string]*FrameProcessingState),
	}

	// Initialize streams and KV
	if err := sm.setupStreams(config); err != nil {
		nc.Close()
		return nil, err
	}

	if err := sm.setupKV(config); err != nil {
		nc.Close()
		return nil, err
	}

	return sm, nil
}

// setupStreams creates JetStream streams
func (sm *StateManager) setupStreams(config StateManagerConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create stream for frame processing events
	stream, err := sm.js.CreateStream(ctx, jetstream.StreamConfig{
		Name:        config.StreamName,
		Description: "Frame processing events",
		Subjects:    []string{"frames.>", "workers.>"},
		Retention:   jetstream.WorkQueuePolicy,
		MaxMsgs:     100000,
		MaxAge:      time.Hour * 24,
		Storage:     jetstream.FileStorage,
		Replicas:    1,
	})
	if err != nil {
		// Stream might already exist
		stream, err = sm.js.Stream(ctx, config.StreamName)
		if err != nil {
			return fmt.Errorf("failed to create/get stream: %w", err)
		}
	}

	sm.stream = stream
	return nil
}

// setupKV creates KV buckets
func (sm *StateManager) setupKV(config StateManagerConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create KV bucket for frame state
	kv, err := sm.js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:       config.KVBucket,
		Description:  "Frame processing state",
		MaxValueSize: 1024 * 1024, // 1MB
		History:      5,
		TTL:          time.Hour * 24,
		Storage:      jetstream.FileStorage,
	})
	if err != nil {
		// Bucket might already exist
		kv, err = sm.js.KeyValue(ctx, config.KVBucket)
		if err != nil {
			return fmt.Errorf("failed to create/get KV: %w", err)
		}
	}

	sm.kv = kv
	return nil
}

// Close closes the state manager
func (sm *StateManager) Close() error {
	// Stop all consumers
	for _, sub := range sm.subs {
		if sub != nil {
			// Consumer interface doesn't have Stop(), skip for now
			_ = sub
		}
	}

	// Close NATS connection
	if sm.nc != nil {
		sm.nc.Close()
	}

	return nil
}

// PublishFrameState publishes a frame state update
func (sm *StateManager) PublishFrameState(ctx context.Context, state *FrameProcessingState) error {
	state.HostID = sm.hostID
	state.Timestamp = time.Now()

	// Serialize
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Publish to stream
	subject := fmt.Sprintf("frames.%s.%s", state.Platform, state.FrameID)
	_, err = sm.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	// Update KV for persistence
	_, err = sm.kv.Put(ctx, state.FrameID, data)
	if err != nil {
		return fmt.Errorf("failed to store in KV: %w", err)
	}

	// Update local cache
	sm.mu.Lock()
	sm.localCache[state.FrameID] = state
	sm.mu.Unlock()

	return nil
}

// GetFrameState retrieves frame state from KV
func (sm *StateManager) GetFrameState(ctx context.Context, frameID string) (*FrameProcessingState, error) {
	// Check local cache first
	sm.mu.RLock()
	if cached, ok := sm.localCache[frameID]; ok {
		sm.mu.RUnlock()
		return cached, nil
	}
	sm.mu.RUnlock()

	// Get from KV
	entry, err := sm.kv.Get(ctx, frameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	// Deserialize
	var state FrameProcessingState
	if err := json.Unmarshal(entry.Value(), &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Update cache
	sm.mu.Lock()
	sm.localCache[frameID] = &state
	sm.mu.Unlock()

	return &state, nil
}

// SubscribeFrames subscribes to frame processing events
func (sm *StateManager) SubscribeFrames(ctx context.Context, platform string, handler func(*FrameProcessingState)) error {
	subject := fmt.Sprintf("frames.%s.*", platform)

	cons, err := sm.js.CreateConsumer(ctx, sm.stream.CachedInfo().Config.Name, jetstream.ConsumerConfig{
		Durable:       fmt.Sprintf("worker-%s-%s", sm.hostID, platform),
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    3,
	})
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}

	// Consume messages
	consCtx, err := cons.Consume(func(msg jetstream.Msg) {
		var state FrameProcessingState
		if err := json.Unmarshal(msg.Data(), &state); err != nil {
			msg.Nak()
			return
		}

		handler(&state)
		msg.Ack()
	})
	if err != nil {
		return fmt.Errorf("failed to start consumer: %w", err)
	}

	sm.subs = append(sm.subs, consCtx)
	return nil
}

// UpdateFrameStatus updates just the status of a frame
func (sm *StateManager) UpdateFrameStatus(ctx context.Context, frameID string, status ProcessingStatus) error {
	state, err := sm.GetFrameState(ctx, frameID)
	if err != nil {
		return err
	}

	state.Status = status
	return sm.PublishFrameState(ctx, state)
}

// DeleteFrameState removes frame state
func (sm *StateManager) DeleteFrameState(ctx context.Context, frameID string) error {
	err := sm.kv.Delete(ctx, frameID)
	if err != nil {
		return fmt.Errorf("failed to delete state: %w", err)
	}

	sm.mu.Lock()
	delete(sm.localCache, frameID)
	sm.mu.Unlock()

	return nil
}

// ListFrames lists all frame states for a platform
func (sm *StateManager) ListFrames(ctx context.Context, platform string) ([]*FrameProcessingState, error) {
	// Use wildcard to get all keys for platform
	keys, err := sm.kv.Keys(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	var frames []*FrameProcessingState
	for _, key := range keys {
		state, err := sm.GetFrameState(ctx, key)
		if err != nil {
			continue
		}
		if platform == "" || state.Platform == platform {
			frames = append(frames, state)
		}
	}

	return frames, nil
}

// GetStats returns statistics about processed frames
func (sm *StateManager) GetStats(ctx context.Context) (*Stats, error) {
	frames, err := sm.ListFrames(ctx, "")
	if err != nil {
		return nil, err
	}

	stats := &Stats{
		TotalFrames: len(frames),
		ByStatus:    make(map[string]int),
		ByPlatform:  make(map[string]int),
	}

	var totalLatency float64
	for _, f := range frames {
		stats.ByStatus[f.Status.String()]++
		stats.ByPlatform[f.Platform]++
		totalLatency += f.LatencyMs
	}

	if len(frames) > 0 {
		stats.AverageLatencyMs = totalLatency / float64(len(frames))
	}

	return stats, nil
}

// Stats represents processing statistics
type Stats struct {
	TotalFrames      int            `json:"total_frames"`
	ByStatus         map[string]int `json:"by_status"`
	ByPlatform       map[string]int `json:"by_platform"`
	AverageLatencyMs float64        `json:"average_latency_ms"`
}

// WatchFrame watches a specific frame for updates
func (sm *StateManager) WatchFrame(ctx context.Context, frameID string, callback func(*FrameProcessingState)) error {
	// Get initial state
	state, err := sm.GetFrameState(ctx, frameID)
	if err != nil {
		return err
	}

	// Call callback with initial state
	callback(state)

	// Watch for updates using watcher
	watcher, err := sm.kv.Watch(ctx, frameID)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	go func() {
		for entry := range watcher.Updates() {
			if entry == nil {
				continue
			}

			var state FrameProcessingState
			if err := json.Unmarshal(entry.Value(), &state); err != nil {
				continue
			}

			callback(&state)
		}
	}()

	return nil
}

// LeaderElection manages leader election for coordinator role
type LeaderElection struct {
	kv       jetstream.KeyValue
	hostID   string
	ctx      context.Context
	cancel   context.CancelFunc
	isLeader bool
	mu       sync.RWMutex
}

// NewLeaderElection creates a new leader election
func NewLeaderElection(kv jetstream.KeyValue, hostID string) *LeaderElection {
	ctx, cancel := context.WithCancel(context.Background())
	return &LeaderElection{
		kv:     kv,
		hostID: hostID,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins leader election
func (le *LeaderElection) Start() error {
	// Try to acquire leadership
	ctx, cancel := context.WithTimeout(le.ctx, 5*time.Second)
	defer cancel()

	// Create/update leadership key with TTL
	data := []byte(le.hostID)
	rev, err := le.kv.Create(ctx, "leader", data)
	if err != nil {
		// Key exists, check if we're the leader
		entry, err := le.kv.Get(ctx, "leader")
		if err != nil {
			return err
		}

		if string(entry.Value()) == le.hostID {
			le.setLeader(true)
			go le.keepAlive()
			return nil
		}

		le.setLeader(false)
		return nil
	}

	// We created the key, we're the leader
	le.setLeader(true)
	_ = rev
	go le.keepAlive()

	return nil
}

// keepAlive periodically updates the leadership key
func (le *LeaderElection) keepAlive() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-le.ctx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(le.ctx, 2*time.Second)
			_, err := le.kv.Put(ctx, "leader", []byte(le.hostID))
			cancel()

			if err != nil {
				le.setLeader(false)
				return
			}
		}
	}
}

// Stop stops leader election
func (le *LeaderElection) Stop() {
	le.cancel()
	le.setLeader(false)
}

// IsLeader returns true if this host is the leader
func (le *LeaderElection) IsLeader() bool {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.isLeader
}

func (le *LeaderElection) setLeader(leader bool) {
	le.mu.Lock()
	defer le.mu.Unlock()
	le.isLeader = leader
}

// WaitForLeader blocks until a leader is elected
func (le *LeaderElection) WaitForLeader(ctx context.Context) (string, error) {
	watcher, err := le.kv.Watch(ctx, "leader")
	if err != nil {
		return "", err
	}

	for entry := range watcher.Updates() {
		if entry == nil {
			continue
		}
		return string(entry.Value()), nil
	}

	return "", ctx.Err()
}
