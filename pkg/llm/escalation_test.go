// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- controllable mock for escalation tests ---

type controllableMock struct {
	name           string
	supportsVision bool
	costPer1k      float64
	qualityScore   float64

	mu        sync.Mutex
	callCount int
	fail      bool
	resp      *Response
}

func (c *controllableMock) Name() string         { return c.name }
func (c *controllableMock) SupportsVision() bool { return c.supportsVision }

func (c *controllableMock) Chat(
	_ context.Context, _ []Message,
) (*Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callCount++
	if c.fail {
		return nil, fmt.Errorf("mock chat fail")
	}
	return c.resp, nil
}

func (c *controllableMock) Vision(
	_ context.Context, _ []byte, _ string,
) (*Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callCount++
	if c.fail {
		return nil, fmt.Errorf("mock vision fail")
	}
	return c.resp, nil
}

func (c *controllableMock) CallCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.callCount
}

func (c *controllableMock) SetFail(fail bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fail = fail
}

// --- NewEscalationProvider tests ---

func TestNewEscalationProvider_SortsByCost(t *testing.T) {
	// Register test entries in the vision registry so
	// buildTiers can sort by cost.
	origRegistry := visionRegistryByProvider
	defer func() { visionRegistryByProvider = origRegistry }()

	visionRegistryByProvider = map[string]visionModelScore{
		"cheap":     {Provider: "cheap", QualityScore: 0.80, CostPer1kTokens: 0.001},
		"expensive": {Provider: "expensive", QualityScore: 0.95, CostPer1kTokens: 0.020},
		"free":      {Provider: "free", QualityScore: 0.65, CostPer1kTokens: 0.0},
	}

	providers := []Provider{
		&mockProvider{name: "expensive", vision: false},
		&mockProvider{name: "cheap"},
		&mockProvider{name: "free"},
	}

	ep := NewEscalationProvider(providers)
	tiers := ep.Tiers()

	require.Len(t, tiers, 3)
	// Free first, then cheap, then expensive.
	assert.Equal(t, "free", tiers[0].Name)
	assert.Equal(t, "cheap", tiers[1].Name)
	assert.Equal(t, "expensive", tiers[2].Name)

	assert.Equal(t, 1, tiers[0].CostRank)
	assert.Equal(t, 2, tiers[1].CostRank)
	assert.Equal(t, 3, tiers[2].CostRank)
}

func TestNewEscalationProvider_DefaultMaxFails(t *testing.T) {
	ep := NewEscalationProvider([]Provider{
		&mockProvider{name: "a"},
	})
	assert.Equal(t, 0, ep.CurrentTier())
	assert.Equal(t, 0, ep.FailCount())
}

func TestNewEscalationProviderWithMaxFails_Custom(
	t *testing.T,
) {
	ep := NewEscalationProviderWithMaxFails(
		[]Provider{&mockProvider{name: "a"}}, 5,
	)
	assert.Equal(t, 5, ep.maxFails)
}

func TestNewEscalationProviderWithMaxFails_ZeroDefault(
	t *testing.T,
) {
	ep := NewEscalationProviderWithMaxFails(
		[]Provider{&mockProvider{name: "a"}}, 0,
	)
	assert.Equal(t, defaultMaxFails, ep.maxFails)
}

func TestEscalationProvider_Name(t *testing.T) {
	ep := NewEscalationProvider(nil)
	assert.Equal(t, "escalation", ep.Name())
}

func TestEscalationProvider_SupportsVision_True(t *testing.T) {
	ep := NewEscalationProvider([]Provider{
		&mockProvider{name: "a", vision: false},
		&mockProvider{name: "b", vision: true},
	})
	assert.True(t, ep.SupportsVision())
}

func TestEscalationProvider_SupportsVision_False(
	t *testing.T,
) {
	ep := NewEscalationProvider([]Provider{
		&mockProvider{name: "a", vision: false},
	})
	assert.False(t, ep.SupportsVision())
}

func TestEscalationProvider_SupportsVision_Empty(
	t *testing.T,
) {
	ep := NewEscalationProvider(nil)
	assert.False(t, ep.SupportsVision())
}

// --- Chat escalation tests ---

func TestEscalationProvider_Chat_Success(t *testing.T) {
	cheap := &controllableMock{
		name: "cheap",
		resp: &Response{Content: "cheap answer"},
	}
	ep := NewEscalationProviderWithMaxFails(
		[]Provider{cheap}, 3,
	)
	ctx := context.Background()

	resp, err := ep.Chat(ctx, []Message{
		{Role: RoleUser, Content: "hi"},
	})
	require.NoError(t, err)
	assert.Equal(t, "cheap answer", resp.Content)
	assert.Equal(t, 0, ep.CurrentTier())
	assert.Equal(t, 0, ep.FailCount())
}

func TestEscalationProvider_Chat_EscalatesAfterMaxFails(
	t *testing.T,
) {
	cheap := &controllableMock{
		name: "cheap",
		fail: true,
	}
	expensive := &controllableMock{
		name: "expensive",
		resp: &Response{Content: "expensive answer"},
	}

	// Override registry so buildTiers sorts correctly.
	origRegistry := visionRegistryByProvider
	defer func() { visionRegistryByProvider = origRegistry }()
	visionRegistryByProvider = map[string]visionModelScore{
		"cheap":     {Provider: "cheap", CostPer1kTokens: 0.001, QualityScore: 0.5},
		"expensive": {Provider: "expensive", CostPer1kTokens: 0.020, QualityScore: 0.9},
	}

	ep := NewEscalationProviderWithMaxFails(
		[]Provider{cheap, expensive}, 3,
	)
	ctx := context.Background()

	// First 3 calls fail on cheap tier.
	for i := 0; i < 3; i++ {
		_, _ = ep.Chat(ctx, []Message{
			{Role: RoleUser, Content: "hi"},
		})
	}
	assert.Equal(t, 1, ep.CurrentTier())
	assert.Equal(t, 0, ep.FailCount())

	// Next call goes to expensive tier and succeeds.
	resp, err := ep.Chat(ctx, []Message{
		{Role: RoleUser, Content: "hi"},
	})
	require.NoError(t, err)
	assert.Equal(t, "expensive answer", resp.Content)
}

func TestEscalationProvider_Chat_NoProviders(t *testing.T) {
	ep := NewEscalationProvider(nil)
	ctx := context.Background()

	_, err := ep.Chat(ctx, []Message{
		{Role: RoleUser, Content: "hi"},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no providers")
}

// --- Vision escalation tests ---

func TestEscalationProvider_Vision_Success(t *testing.T) {
	cheap := &controllableMock{
		name:           "cheap",
		supportsVision: true,
		resp:           &Response{Content: "vision result"},
	}
	ep := NewEscalationProviderWithMaxFails(
		[]Provider{cheap}, 3,
	)
	ctx := context.Background()

	resp, err := ep.Vision(ctx, []byte("img"), "prompt")
	require.NoError(t, err)
	assert.Equal(t, "vision result", resp.Content)
	assert.Equal(t, 0, ep.FailCount())
}

func TestEscalationProvider_Vision_Escalates(t *testing.T) {
	cheap := &controllableMock{
		name:           "cheap",
		supportsVision: true,
		fail:           true,
	}
	expensive := &controllableMock{
		name:           "expensive",
		supportsVision: true,
		resp: &Response{
			Content: "expensive vision",
		},
	}

	origRegistry := visionRegistryByProvider
	defer func() { visionRegistryByProvider = origRegistry }()
	visionRegistryByProvider = map[string]visionModelScore{
		"cheap":     {Provider: "cheap", CostPer1kTokens: 0.0, QualityScore: 0.5},
		"expensive": {Provider: "expensive", CostPer1kTokens: 0.02, QualityScore: 0.9},
	}

	ep := NewEscalationProviderWithMaxFails(
		[]Provider{cheap, expensive}, 2,
	)
	ctx := context.Background()

	// Two failures trigger escalation.
	_, _ = ep.Vision(ctx, []byte("img"), "p")
	_, _ = ep.Vision(ctx, []byte("img"), "p")
	assert.Equal(t, 1, ep.CurrentTier())

	resp, err := ep.Vision(ctx, []byte("img"), "p")
	require.NoError(t, err)
	assert.Equal(t, "expensive vision", resp.Content)
}

func TestEscalationProvider_Vision_NoProviders(t *testing.T) {
	ep := NewEscalationProvider(nil)
	ctx := context.Background()

	_, err := ep.Vision(ctx, []byte("img"), "p")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no providers")
}

// --- State management tests ---

func TestEscalationProvider_SuccessResetsFailCount(
	t *testing.T,
) {
	mock := &controllableMock{
		name: "a",
		resp: &Response{Content: "ok"},
	}
	ep := NewEscalationProviderWithMaxFails(
		[]Provider{mock}, 3,
	)
	ctx := context.Background()

	// Two failures.
	mock.SetFail(true)
	_, _ = ep.Chat(ctx, []Message{
		{Role: RoleUser, Content: "hi"},
	})
	_, _ = ep.Chat(ctx, []Message{
		{Role: RoleUser, Content: "hi"},
	})
	assert.Equal(t, 2, ep.FailCount())

	// One success resets the counter.
	mock.SetFail(false)
	_, _ = ep.Chat(ctx, []Message{
		{Role: RoleUser, Content: "hi"},
	})
	assert.Equal(t, 0, ep.FailCount())
	assert.Equal(t, 0, ep.CurrentTier())
}

func TestEscalationProvider_StaysAtMaxTier(t *testing.T) {
	mock := &controllableMock{
		name: "only",
		fail: true,
	}
	ep := NewEscalationProviderWithMaxFails(
		[]Provider{mock}, 2,
	)
	ctx := context.Background()

	// Exhaust failures — should stay at tier 0 since there
	// is only one tier.
	for i := 0; i < 10; i++ {
		_, _ = ep.Chat(ctx, []Message{
			{Role: RoleUser, Content: "hi"},
		})
	}
	assert.Equal(t, 0, ep.CurrentTier())
}

func TestEscalationProvider_Reset(t *testing.T) {
	cheap := &controllableMock{name: "cheap", fail: true}
	expensive := &controllableMock{
		name: "expensive",
		resp: &Response{Content: "ok"},
	}

	origRegistry := visionRegistryByProvider
	defer func() { visionRegistryByProvider = origRegistry }()
	visionRegistryByProvider = map[string]visionModelScore{
		"cheap":     {Provider: "cheap", CostPer1kTokens: 0.0, QualityScore: 0.5},
		"expensive": {Provider: "expensive", CostPer1kTokens: 0.02, QualityScore: 0.9},
	}

	ep := NewEscalationProviderWithMaxFails(
		[]Provider{cheap, expensive}, 2,
	)
	ctx := context.Background()

	// Escalate to tier 1.
	_, _ = ep.Chat(ctx, []Message{
		{Role: RoleUser, Content: "hi"},
	})
	_, _ = ep.Chat(ctx, []Message{
		{Role: RoleUser, Content: "hi"},
	})
	assert.Equal(t, 1, ep.CurrentTier())

	// Reset brings it back to tier 0.
	ep.Reset()
	assert.Equal(t, 0, ep.CurrentTier())
	assert.Equal(t, 0, ep.FailCount())
}

func TestEscalationProvider_Status(t *testing.T) {
	ep := NewEscalationProviderWithMaxFails(
		[]Provider{
			&mockProvider{name: "cheap"},
			&mockProvider{name: "mid"},
			&mockProvider{name: "expensive", vision: false},
		}, 3,
	)

	status := ep.Status()
	assert.Contains(t, status, "escalation:")
	assert.Contains(t, status, "tier=")
	assert.Contains(t, status, "fails=")
}

func TestEscalationProvider_Status_Empty(t *testing.T) {
	ep := NewEscalationProvider(nil)
	status := ep.Status()
	assert.Contains(t, status, "no tiers")
}

// --- EmptyResponse triggers failure ---

func TestEscalationProvider_EmptyResponseCountsAsFailure(
	t *testing.T,
) {
	mock := &controllableMock{
		name: "a",
		resp: &Response{Content: ""},
	}
	ep := NewEscalationProviderWithMaxFails(
		[]Provider{
			mock,
			&controllableMock{
				name: "b",
				resp: &Response{Content: "ok"},
			},
		}, 2,
	)
	ctx := context.Background()

	// Empty responses count as failures.
	_, _ = ep.Chat(ctx, []Message{
		{Role: RoleUser, Content: "hi"},
	})
	assert.Equal(t, 1, ep.FailCount())

	_, _ = ep.Chat(ctx, []Message{
		{Role: RoleUser, Content: "hi"},
	})
	// Should have escalated.
	assert.Equal(t, 1, ep.CurrentTier())
}

// --- NilResponse triggers failure ---

func TestEscalationProvider_NilResponseCountsAsFailure(
	t *testing.T,
) {
	mock := &controllableMock{
		name: "a",
		resp: nil,
	}
	ep := NewEscalationProviderWithMaxFails(
		[]Provider{
			mock,
			&controllableMock{
				name: "b",
				resp: &Response{Content: "ok"},
			},
		}, 1,
	)
	ctx := context.Background()

	_, _ = ep.Chat(ctx, []Message{
		{Role: RoleUser, Content: "hi"},
	})
	assert.Equal(t, 1, ep.CurrentTier())
}

// --- Concurrent access ---

func TestEscalationProvider_ConcurrentAccess(t *testing.T) {
	mock := &controllableMock{
		name: "a",
		resp: &Response{Content: "ok"},
	}
	ep := NewEscalationProviderWithMaxFails(
		[]Provider{mock}, 3,
	)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = ep.Chat(ctx, []Message{
				{Role: RoleUser, Content: "hi"},
			})
			_ = ep.CurrentTier()
			_ = ep.FailCount()
			_ = ep.Status()
		}()
	}
	wg.Wait()

	assert.Equal(t, 0, ep.CurrentTier())
}
