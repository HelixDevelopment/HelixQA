// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock provider for consensus tests ---

// consensusMock extends the base mockProvider with a
// visionDelay field for timeout testing.
type consensusMock struct {
	name           string
	supportsVision bool
	chatResp       *Response
	chatErr        error
	visionResp     *Response
	visionErr      error
	visionDelay    time.Duration
}

func (m *consensusMock) Name() string         { return m.name }
func (m *consensusMock) SupportsVision() bool { return m.supportsVision }

func (m *consensusMock) Chat(
	_ context.Context, _ []Message,
) (*Response, error) {
	if m.chatErr != nil {
		return nil, m.chatErr
	}
	return m.chatResp, nil
}

func (m *consensusMock) Vision(
	ctx context.Context, _ []byte, _ string,
) (*Response, error) {
	if m.visionDelay > 0 {
		select {
		case <-time.After(m.visionDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.visionErr != nil {
		return nil, m.visionErr
	}
	return m.visionResp, nil
}

// --- NewConsensusProvider tests ---

func TestNewConsensusProvider_Defaults(t *testing.T) {
	providers := []Provider{
		&consensusMock{name: "a", supportsVision: true},
		&consensusMock{name: "b", supportsVision: true},
		&consensusMock{name: "c", supportsVision: true},
	}
	cp := NewConsensusProvider(providers, 0)
	assert.Equal(t, defaultConsensusQuorum, cp.Quorum())
	assert.Len(t, cp.Providers(), 3)
}

func TestNewConsensusProvider_QuorumClamped(t *testing.T) {
	providers := []Provider{
		&consensusMock{name: "a"},
		&consensusMock{name: "b"},
	}
	// Quorum larger than provider count is clamped.
	cp := NewConsensusProvider(providers, 10)
	assert.Equal(t, 2, cp.Quorum())
}

func TestNewConsensusProvider_NegativeQuorum(t *testing.T) {
	providers := []Provider{
		&consensusMock{name: "a"},
		&consensusMock{name: "b"},
		&consensusMock{name: "c"},
	}
	cp := NewConsensusProvider(providers, -1)
	assert.Equal(t, defaultConsensusQuorum, cp.Quorum())
}

func TestConsensusProvider_Name(t *testing.T) {
	cp := NewConsensusProvider(nil, 2)
	assert.Equal(t, "consensus", cp.Name())
}

func TestConsensusProvider_SupportsVision_True(t *testing.T) {
	providers := []Provider{
		&consensusMock{name: "a", supportsVision: false},
		&consensusMock{name: "b", supportsVision: true},
	}
	cp := NewConsensusProvider(providers, 2)
	assert.True(t, cp.SupportsVision())
}

func TestConsensusProvider_SupportsVision_False(t *testing.T) {
	providers := []Provider{
		&consensusMock{name: "a", supportsVision: false},
		&consensusMock{name: "b", supportsVision: false},
	}
	cp := NewConsensusProvider(providers, 2)
	assert.False(t, cp.SupportsVision())
}

func TestConsensusProvider_SupportsVision_Empty(t *testing.T) {
	cp := NewConsensusProvider(nil, 2)
	assert.False(t, cp.SupportsVision())
}

// --- Chat tests ---

func TestConsensusProvider_Chat_Success(t *testing.T) {
	providers := []Provider{
		&consensusMock{
			name:     "a",
			chatResp: &Response{Content: "hello"},
		},
	}
	cp := NewConsensusProvider(providers, 1)
	ctx := context.Background()

	resp, err := cp.Chat(ctx, []Message{
		{Role: RoleUser, Content: "hi"},
	})
	require.NoError(t, err)
	assert.Equal(t, "hello", resp.Content)
}

func TestConsensusProvider_Chat_Fallthrough(t *testing.T) {
	providers := []Provider{
		&consensusMock{
			name:    "a",
			chatErr: fmt.Errorf("fail"),
		},
		&consensusMock{
			name:     "b",
			chatResp: &Response{Content: "from b"},
		},
	}
	cp := NewConsensusProvider(providers, 1)
	ctx := context.Background()

	resp, err := cp.Chat(ctx, []Message{
		{Role: RoleUser, Content: "hi"},
	})
	require.NoError(t, err)
	assert.Equal(t, "from b", resp.Content)
}

func TestConsensusProvider_Chat_AllFail(t *testing.T) {
	providers := []Provider{
		&consensusMock{name: "a", chatErr: fmt.Errorf("e1")},
		&consensusMock{name: "b", chatErr: fmt.Errorf("e2")},
	}
	cp := NewConsensusProvider(providers, 1)
	ctx := context.Background()

	_, err := cp.Chat(ctx, []Message{
		{Role: RoleUser, Content: "hi"},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all providers failed")
}

func TestConsensusProvider_Chat_NilProviders(t *testing.T) {
	cp := NewConsensusProvider(nil, 1)
	ctx := context.Background()

	_, err := cp.Chat(ctx, []Message{
		{Role: RoleUser, Content: "hi"},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no providers")
}

// --- Vision consensus tests ---

func TestConsensusProvider_Vision_AllAgree(t *testing.T) {
	action := `{"action":"tap","x":100,"y":200}`
	providers := []Provider{
		&consensusMock{
			name: "a", supportsVision: true,
			visionResp: &Response{Content: action},
		},
		&consensusMock{
			name: "b", supportsVision: true,
			visionResp: &Response{Content: action},
		},
		&consensusMock{
			name: "c", supportsVision: true,
			visionResp: &Response{Content: action},
		},
	}
	cp := NewConsensusProvider(providers, 2)
	ctx := context.Background()

	resp, err := cp.Vision(ctx, []byte("img"), "prompt")
	require.NoError(t, err)
	assert.Contains(t, resp.Content, "tap")
}

func TestConsensusProvider_Vision_MajorityWins(t *testing.T) {
	providers := []Provider{
		&consensusMock{
			name: "a", supportsVision: true,
			visionResp: &Response{
				Content: `{"action":"tap","x":10,"y":20}`,
			},
		},
		&consensusMock{
			name: "b", supportsVision: true,
			visionResp: &Response{
				Content: `{"action":"tap","x":11,"y":21}`,
			},
		},
		&consensusMock{
			name: "c", supportsVision: true,
			visionResp: &Response{
				Content: `{"action":"swipe","dir":"up"}`,
			},
		},
	}
	cp := NewConsensusProvider(providers, 2)
	ctx := context.Background()

	resp, err := cp.Vision(ctx, []byte("img"), "prompt")
	require.NoError(t, err)
	// "tap" has 2 votes, "swipe" has 1 — tap wins.
	assert.Contains(t, resp.Content, "tap")
}

func TestConsensusProvider_Vision_SplitVote_Fallback(
	t *testing.T,
) {
	providers := []Provider{
		&consensusMock{
			name: "a", supportsVision: true,
			visionResp: &Response{
				Content: `{"action":"tap"}`,
			},
		},
		&consensusMock{
			name: "b", supportsVision: true,
			visionResp: &Response{
				Content: `{"action":"swipe"}`,
			},
		},
		&consensusMock{
			name: "c", supportsVision: true,
			visionResp: &Response{
				Content: `{"action":"scroll"}`,
			},
		},
	}
	// Quorum of 2: no action gets 2 votes.
	cp := NewConsensusProvider(providers, 2)
	ctx := context.Background()

	resp, err := cp.Vision(ctx, []byte("img"), "prompt")
	require.NoError(t, err)
	// Falls back to the first successful provider.
	assert.NotEmpty(t, resp.Content)
}

func TestConsensusProvider_Vision_SingleProvider(t *testing.T) {
	providers := []Provider{
		&consensusMock{
			name: "solo", supportsVision: true,
			visionResp: &Response{Content: "result"},
		},
	}
	cp := NewConsensusProvider(providers, 1)
	ctx := context.Background()

	resp, err := cp.Vision(ctx, []byte("img"), "prompt")
	require.NoError(t, err)
	assert.Equal(t, "result", resp.Content)
}

func TestConsensusProvider_Vision_AllFail(t *testing.T) {
	providers := []Provider{
		&consensusMock{
			name: "a", supportsVision: true,
			visionErr: fmt.Errorf("e1"),
		},
		&consensusMock{
			name: "b", supportsVision: true,
			visionErr: fmt.Errorf("e2"),
		},
	}
	cp := NewConsensusProvider(providers, 2)
	ctx := context.Background()

	_, err := cp.Vision(ctx, []byte("img"), "prompt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all providers failed")
}

func TestConsensusProvider_Vision_NilProviders(t *testing.T) {
	cp := NewConsensusProvider(nil, 1)
	ctx := context.Background()

	_, err := cp.Vision(ctx, []byte("img"), "prompt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no providers")
}

func TestConsensusProvider_Vision_PartialFailure(
	t *testing.T,
) {
	// One fails, two succeed with same action.
	providers := []Provider{
		&consensusMock{
			name: "a", supportsVision: true,
			visionErr: fmt.Errorf("timeout"),
		},
		&consensusMock{
			name: "b", supportsVision: true,
			visionResp: &Response{
				Content: `{"action":"tap"}`,
			},
		},
		&consensusMock{
			name: "c", supportsVision: true,
			visionResp: &Response{
				Content: `{"action":"tap"}`,
			},
		},
	}
	cp := NewConsensusProvider(providers, 2)
	ctx := context.Background()

	resp, err := cp.Vision(ctx, []byte("img"), "prompt")
	require.NoError(t, err)
	assert.Contains(t, resp.Content, "tap")
}

func TestConsensusProvider_Vision_EmptyResponse(t *testing.T) {
	providers := []Provider{
		&consensusMock{
			name: "a", supportsVision: true,
			visionResp: &Response{Content: ""},
		},
		&consensusMock{
			name: "b", supportsVision: true,
			visionResp: &Response{
				Content: `{"action":"press"}`,
			},
		},
	}
	cp := NewConsensusProvider(providers, 1)
	ctx := context.Background()

	resp, err := cp.Vision(ctx, []byte("img"), "prompt")
	require.NoError(t, err)
	// Only provider "b" has content.
	assert.Contains(t, resp.Content, "press")
}

func TestConsensusProvider_Vision_Timeout(t *testing.T) {
	providers := []Provider{
		&consensusMock{
			name: "slow", supportsVision: true,
			visionDelay: 5 * time.Second,
			visionResp:  &Response{Content: "late"},
		},
		&consensusMock{
			name: "fast", supportsVision: true,
			visionResp: &Response{
				Content: `{"action":"tap"}`,
			},
		},
	}
	cp := NewConsensusProvider(providers, 1)
	cp.SetTimeout(100 * time.Millisecond)
	ctx := context.Background()

	resp, err := cp.Vision(ctx, []byte("img"), "prompt")
	require.NoError(t, err)
	// Fast provider should succeed even though slow times out.
	assert.NotEmpty(t, resp.Content)
}

func TestConsensusProvider_Vision_NonVisionSkipped(
	t *testing.T,
) {
	providers := []Provider{
		&consensusMock{
			name: "chat-only", supportsVision: false,
		},
		&consensusMock{
			name: "vision", supportsVision: true,
			visionResp: &Response{Content: "ok"},
		},
	}
	cp := NewConsensusProvider(providers, 1)
	ctx := context.Background()

	resp, err := cp.Vision(ctx, []byte("img"), "prompt")
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Content)
}

func TestConsensusProvider_SetTimeout(t *testing.T) {
	cp := NewConsensusProvider(nil, 1)

	// Valid timeout.
	cp.SetTimeout(5 * time.Second)
	assert.Equal(t, 5*time.Second, cp.timeout)

	// Invalid (zero) — should not change.
	cp.SetTimeout(0)
	assert.Equal(t, 5*time.Second, cp.timeout)

	// Negative — should not change.
	cp.SetTimeout(-1)
	assert.Equal(t, 5*time.Second, cp.timeout)
}

// --- extractActionType tests ---

func TestExtractActionType_JSONObject(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "action field",
			input:    `{"action":"tap","x":100}`,
			expected: "tap",
		},
		{
			name:     "type field",
			input:    `{"type":"SWIPE","dir":"up"}`,
			expected: "swipe",
		},
		{
			name:     "action preferred over type",
			input:    `{"action":"click","type":"press"}`,
			expected: "click",
		},
		{
			name:     "markdown fenced json",
			input:    "```json\n{\"action\":\"scroll\"}\n```",
			expected: "scroll",
		},
		{
			name:     "json array",
			input:    `[{"action":"tap"},{"action":"swipe"}]`,
			expected: "tap",
		},
		{
			name:     "keyword fallback - tap",
			input:    "I would tap on the login button",
			expected: "tap",
		},
		{
			name:     "keyword fallback - swipe",
			input:    "Swipe up to see more content",
			expected: "swipe",
		},
		{
			name:     "empty content",
			input:    "",
			expected: "unknown",
		},
		{
			name:     "no recognizable action",
			input:    "analyze the screen layout",
			expected: "unknown",
		},
		{
			name:     "whitespace only",
			input:    "   \n\t  ",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractActionType(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}
