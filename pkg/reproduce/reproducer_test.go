// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package reproduce

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/llm"
)

// --- mock executor ---

// mockExecutor records all actions and returns canned
// screenshots. It implements navigator.ActionExecutor.
type mockExecutor struct {
	actions       []string
	screenshotErr error
	screenshotImg []byte
	clickErr      error
	typeErr       error
	scrollErr     error
	swipeErr      error
	keyPressErr   error
	longPressErr  error
	clearErr      error
	backErr       error
	homeErr       error
}

func (m *mockExecutor) Click(
	_ context.Context, x, y int,
) error {
	m.actions = append(m.actions,
		fmt.Sprintf("click:%d,%d", x, y))
	return m.clickErr
}

func (m *mockExecutor) Type(
	_ context.Context, text string,
) error {
	m.actions = append(m.actions, "type:"+text)
	return m.typeErr
}

func (m *mockExecutor) Clear(_ context.Context) error {
	m.actions = append(m.actions, "clear")
	return m.clearErr
}

func (m *mockExecutor) Scroll(
	_ context.Context, direction string, amount int,
) error {
	m.actions = append(m.actions,
		fmt.Sprintf("scroll:%s,%d", direction, amount))
	return m.scrollErr
}

func (m *mockExecutor) LongPress(
	_ context.Context, x, y int,
) error {
	m.actions = append(m.actions,
		fmt.Sprintf("longpress:%d,%d", x, y))
	return m.longPressErr
}

func (m *mockExecutor) Swipe(
	_ context.Context, fromX, fromY, toX, toY int,
) error {
	m.actions = append(m.actions,
		fmt.Sprintf("swipe:%d,%d,%d,%d",
			fromX, fromY, toX, toY))
	return m.swipeErr
}

func (m *mockExecutor) KeyPress(
	_ context.Context, key string,
) error {
	m.actions = append(m.actions, "key:"+key)
	return m.keyPressErr
}

func (m *mockExecutor) Back(_ context.Context) error {
	m.actions = append(m.actions, "back")
	return m.backErr
}

func (m *mockExecutor) Home(_ context.Context) error {
	m.actions = append(m.actions, "home")
	return m.homeErr
}

func (m *mockExecutor) Screenshot(
	_ context.Context,
) ([]byte, error) {
	return m.screenshotImg, m.screenshotErr
}

// --- mock LLM ---

// mockVisionProvider returns canned responses for Vision
// calls. It satisfies llm.Provider.
type mockVisionProvider struct {
	response      string
	supportsVis   bool
	visionErr     error
	visionCalls   int
	lastPrompt    string
	lastImageSize int
}

func (m *mockVisionProvider) Chat(
	_ context.Context, _ []llm.Message,
) (*llm.Response, error) {
	return &llm.Response{Content: "ok"}, nil
}

func (m *mockVisionProvider) Vision(
	_ context.Context, image []byte, prompt string,
) (*llm.Response, error) {
	m.visionCalls++
	m.lastPrompt = prompt
	m.lastImageSize = len(image)
	if m.visionErr != nil {
		return nil, m.visionErr
	}
	return &llm.Response{Content: m.response}, nil
}

func (m *mockVisionProvider) Name() string { return "mock" }

func (m *mockVisionProvider) SupportsVision() bool {
	return m.supportsVis
}

// --- tests ---

func TestNewBugReproducer_Defaults(t *testing.T) {
	exec := &mockExecutor{screenshotImg: []byte("png")}
	prov := &mockVisionProvider{supportsVis: true}

	br := NewBugReproducer(exec, prov)

	assert.Equal(t, defaultMaxRetries, br.maxRetries)
	assert.Equal(t, defaultActionDelay, br.actionDelay)
	assert.Equal(t,
		defaultScreenshotDelay, br.screenshotDelay,
	)
}

func TestNewBugReproducer_WithOptions(t *testing.T) {
	exec := &mockExecutor{screenshotImg: []byte("png")}
	prov := &mockVisionProvider{supportsVis: true}

	br := NewBugReproducer(exec, prov,
		WithMaxRetries(5),
		WithActionDelay(2*time.Second),
		WithScreenshotDelay(100*time.Millisecond),
	)

	assert.Equal(t, 5, br.maxRetries)
	assert.Equal(t, 2*time.Second, br.actionDelay)
	assert.Equal(t,
		100*time.Millisecond, br.screenshotDelay,
	)
}

func TestNewBugReproducer_InvalidOptions(t *testing.T) {
	exec := &mockExecutor{screenshotImg: []byte("png")}
	prov := &mockVisionProvider{supportsVis: true}

	br := NewBugReproducer(exec, prov,
		WithMaxRetries(-1),
		WithActionDelay(-1*time.Second),
	)

	// Invalid values should be ignored.
	assert.Equal(t, defaultMaxRetries, br.maxRetries)
	assert.Equal(t, defaultActionDelay, br.actionDelay)
}

func TestBugReproducer_Reproduce_Confirmed(t *testing.T) {
	exec := &mockExecutor{screenshotImg: []byte("fake-png")}
	prov := &mockVisionProvider{
		supportsVis: true,
		response:    "YES, the button is truncated as described.",
	}

	br := NewBugReproducer(exec, prov,
		WithActionDelay(0),
		WithScreenshotDelay(0),
	)

	bug := Bug{
		ID:          "BUG-001",
		Description: "Submit button text is truncated",
		ActionSequence: []Action{
			{Type: "click", Value: "100,200"},
			{Type: "type", Value: "hello"},
		},
		Severity: "high",
	}

	result, err := br.Reproduce(context.Background(), bug)
	require.NoError(t, err)

	assert.True(t, result.Reproduced)
	assert.Equal(t, "BUG-001", result.BugID)
	assert.Equal(t, 1, result.Attempts)
	assert.NotEmpty(t, result.Evidence)
	assert.Contains(t, result.Evidence, "truncated")
	assert.NotEmpty(t, result.Screenshots)
	assert.Greater(t,
		result.Duration, time.Duration(0),
	)

	// Verify actions were replayed.
	assert.Contains(t, exec.actions, "click:100,200")
	assert.Contains(t, exec.actions, "type:hello")
}

func TestBugReproducer_Reproduce_NotReproduced(t *testing.T) {
	exec := &mockExecutor{screenshotImg: []byte("fake-png")}
	prov := &mockVisionProvider{
		supportsVis: true,
		response:    "NO, the button looks normal.",
	}

	br := NewBugReproducer(exec, prov,
		WithMaxRetries(2),
		WithActionDelay(0),
		WithScreenshotDelay(0),
	)

	bug := Bug{
		ID:          "BUG-002",
		Description: "Text overlap on settings screen",
		ActionSequence: []Action{
			{Type: "click", Value: "50,60"},
		},
		Severity: "medium",
	}

	result, err := br.Reproduce(context.Background(), bug)
	require.NoError(t, err)

	assert.False(t, result.Reproduced)
	assert.Equal(t, 2, result.Attempts)
	assert.Empty(t, result.Evidence)
}

func TestBugReproducer_Reproduce_EmptyActions(t *testing.T) {
	exec := &mockExecutor{screenshotImg: []byte("fake-png")}
	prov := &mockVisionProvider{
		supportsVis: true,
		response:    "YES, the screen is blank.",
	}

	br := NewBugReproducer(exec, prov,
		WithActionDelay(0),
		WithScreenshotDelay(0),
	)

	bug := Bug{
		ID:             "BUG-003",
		Description:    "Blank screen on startup",
		ActionSequence: nil,
		Severity:       "critical",
	}

	result, err := br.Reproduce(context.Background(), bug)
	require.NoError(t, err)
	assert.True(t, result.Reproduced)
	assert.Equal(t, 1, result.Attempts)
}

func TestBugReproducer_Reproduce_ValidationError(t *testing.T) {
	exec := &mockExecutor{screenshotImg: []byte("png")}
	prov := &mockVisionProvider{supportsVis: true}

	br := NewBugReproducer(exec, prov)

	// Missing ID.
	_, err := br.Reproduce(context.Background(), Bug{
		Description: "some bug",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bug ID is required")

	// Missing description.
	_, err = br.Reproduce(context.Background(), Bug{
		ID: "BUG-X",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "description is required")
}

func TestBugReproducer_Reproduce_ContextCanceled(t *testing.T) {
	exec := &mockExecutor{screenshotImg: []byte("png")}
	prov := &mockVisionProvider{
		supportsVis: true,
		response:    "NO",
	}

	br := NewBugReproducer(exec, prov,
		WithMaxRetries(10),
		WithActionDelay(0),
		WithScreenshotDelay(0),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	bug := Bug{
		ID:          "BUG-004",
		Description: "Crash on scroll",
		ActionSequence: []Action{
			{Type: "scroll", Value: "down 300"},
		},
	}

	result, err := br.Reproduce(ctx, bug)
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Reproduced)
	assert.NotEmpty(t, result.Error)
}

func TestBugReproducer_Reproduce_ScreenshotFailure(t *testing.T) {
	exec := &mockExecutor{
		screenshotErr: fmt.Errorf("device disconnected"),
	}
	prov := &mockVisionProvider{
		supportsVis: true,
		response:    "YES",
	}

	br := NewBugReproducer(exec, prov,
		WithMaxRetries(2),
		WithActionDelay(0),
		WithScreenshotDelay(0),
	)

	bug := Bug{
		ID:          "BUG-005",
		Description: "Missing icon",
		Severity:    "low",
	}

	result, err := br.Reproduce(context.Background(), bug)
	require.NoError(t, err)

	// Cannot confirm without a screenshot.
	assert.False(t, result.Reproduced)
	assert.Equal(t, 2, result.Attempts)
}

func TestBugReproducer_Reproduce_VisionNotSupported(
	t *testing.T,
) {
	exec := &mockExecutor{screenshotImg: []byte("png")}
	prov := &mockVisionProvider{
		supportsVis: false,
		response:    "YES",
	}

	br := NewBugReproducer(exec, prov,
		WithMaxRetries(1),
		WithActionDelay(0),
		WithScreenshotDelay(0),
	)

	bug := Bug{
		ID:          "BUG-006",
		Description: "Layout shift",
		Severity:    "medium",
	}

	result, err := br.Reproduce(context.Background(), bug)
	require.NoError(t, err)

	// Cannot confirm without vision support.
	assert.False(t, result.Reproduced)
}

func TestBugReproducer_Reproduce_VisionError(t *testing.T) {
	exec := &mockExecutor{screenshotImg: []byte("png")}
	prov := &mockVisionProvider{
		supportsVis: true,
		visionErr:   fmt.Errorf("rate limited"),
	}

	br := NewBugReproducer(exec, prov,
		WithMaxRetries(2),
		WithActionDelay(0),
		WithScreenshotDelay(0),
	)

	bug := Bug{
		ID:          "BUG-007",
		Description: "Wrong color",
		Severity:    "low",
	}

	result, err := br.Reproduce(context.Background(), bug)
	require.NoError(t, err)

	assert.False(t, result.Reproduced)
	assert.Equal(t, 2, result.Attempts)
}

func TestBugReproducer_Reproduce_ActionFailure(t *testing.T) {
	exec := &mockExecutor{
		screenshotImg: []byte("png"),
		clickErr:      fmt.Errorf("touch failed"),
	}
	prov := &mockVisionProvider{
		supportsVis: true,
		response:    "YES",
	}

	br := NewBugReproducer(exec, prov,
		WithMaxRetries(2),
		WithActionDelay(0),
		WithScreenshotDelay(0),
	)

	bug := Bug{
		ID:          "BUG-008",
		Description: "Wrong data displayed",
		ActionSequence: []Action{
			{Type: "click", Value: "100,200"},
		},
		Severity: "high",
	}

	result, err := br.Reproduce(context.Background(), bug)
	require.NoError(t, err)

	// Action failure means no screenshot taken on that
	// attempt, so it retries. Both attempts fail.
	assert.False(t, result.Reproduced)
	assert.Equal(t, 2, result.Attempts)
}

func TestBugReproducer_Reproduce_ConfirmedOnSecondAttempt(
	t *testing.T,
) {
	callCount := 0
	exec := &mockExecutor{screenshotImg: []byte("png")}
	prov := &mockVisionProvider{supportsVis: true}

	// Return NO first, then YES.
	origVision := prov.Vision
	_ = origVision
	prov.response = "NO"

	br := NewBugReproducer(exec, prov,
		WithMaxRetries(3),
		WithActionDelay(0),
		WithScreenshotDelay(0),
	)

	// Override the provider to vary response per call.
	varyingProv := &varyingVisionProvider{
		responses: []string{
			"NO, bug not visible.",
			"YES, the bug is now visible.",
		},
	}
	br.provider = varyingProv

	bug := Bug{
		ID:          "BUG-009",
		Description: "Intermittent layout glitch",
		Severity:    "medium",
	}

	result, err := br.Reproduce(context.Background(), bug)
	require.NoError(t, err)
	_ = callCount

	assert.True(t, result.Reproduced)
	assert.Equal(t, 2, result.Attempts)
	assert.Contains(t, result.Evidence, "now visible")
}

func TestBugReproducer_ReproduceBatch(t *testing.T) {
	exec := &mockExecutor{screenshotImg: []byte("png")}
	prov := &mockVisionProvider{
		supportsVis: true,
		response:    "YES, confirmed.",
	}

	br := NewBugReproducer(exec, prov,
		WithActionDelay(0),
		WithScreenshotDelay(0),
	)

	bugs := []Bug{
		{
			ID:          "BUG-010",
			Description: "Missing icon",
			Severity:    "high",
		},
		{
			ID:          "BUG-011",
			Description: "Wrong color scheme",
			Severity:    "low",
		},
	}

	results, err := br.ReproduceBatch(
		context.Background(), bugs,
	)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	assert.True(t, results[0].Reproduced)
	assert.Equal(t, "BUG-010", results[0].BugID)

	assert.True(t, results[1].Reproduced)
	assert.Equal(t, "BUG-011", results[1].BugID)
}

func TestBugReproducer_ReproduceBatch_WithInvalid(
	t *testing.T,
) {
	exec := &mockExecutor{screenshotImg: []byte("png")}
	prov := &mockVisionProvider{
		supportsVis: true,
		response:    "YES",
	}

	br := NewBugReproducer(exec, prov,
		WithActionDelay(0),
		WithScreenshotDelay(0),
	)

	bugs := []Bug{
		{ID: "", Description: "no id"},
		{
			ID:          "BUG-012",
			Description: "Valid bug",
			Severity:    "high",
		},
	}

	results, err := br.ReproduceBatch(
		context.Background(), bugs,
	)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// First has a validation error.
	assert.NotEmpty(t, results[0].Error)
	assert.False(t, results[0].Reproduced)

	// Second is valid and reproduced.
	assert.True(t, results[1].Reproduced)
}

func TestBugReproducer_ReproduceBatch_ContextCanceled(
	t *testing.T,
) {
	exec := &mockExecutor{screenshotImg: []byte("png")}
	prov := &mockVisionProvider{
		supportsVis: true,
		response:    "NO",
	}

	br := NewBugReproducer(exec, prov,
		WithMaxRetries(100),
		WithActionDelay(0),
		WithScreenshotDelay(0),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	bugs := []Bug{
		{ID: "BUG-A", Description: "Bug A"},
		{ID: "BUG-B", Description: "Bug B"},
	}

	results, err := br.ReproduceBatch(ctx, bugs)
	assert.Error(t, err)
	// Some results may be partial.
	assert.NotNil(t, results)
}

func TestHighSeverityBugs(t *testing.T) {
	bugs := []Bug{
		{ID: "B1", Description: "x", Severity: "critical"},
		{ID: "B2", Description: "x", Severity: "low"},
		{ID: "B3", Description: "x", Severity: "high"},
		{ID: "B4", Description: "x", Severity: "medium"},
		{ID: "B5", Description: "x", Severity: "HIGH"},
		{ID: "B6", Description: "x", Severity: "CRITICAL"},
	}

	result := HighSeverityBugs(bugs)
	assert.Len(t, result, 4)

	ids := make([]string, len(result))
	for i, b := range result {
		ids[i] = b.ID
	}
	assert.Contains(t, ids, "B1")
	assert.Contains(t, ids, "B3")
	assert.Contains(t, ids, "B5")
	assert.Contains(t, ids, "B6")
}

func TestHighSeverityBugs_Empty(t *testing.T) {
	result := HighSeverityBugs(nil)
	assert.Nil(t, result)
}

func TestBug_Validate(t *testing.T) {
	tests := []struct {
		name    string
		bug     Bug
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid",
			bug: Bug{
				ID:          "BUG-001",
				Description: "some bug",
			},
			wantErr: false,
		},
		{
			name:    "missing ID",
			bug:     Bug{Description: "desc"},
			wantErr: true,
			errMsg:  "bug ID is required",
		},
		{
			name:    "missing description",
			bug:     Bug{ID: "BUG-X"},
			wantErr: true,
			errMsg:  "description is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.bug.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExecuteAction_AllTypes(t *testing.T) {
	exec := &mockExecutor{screenshotImg: []byte("png")}
	ctx := context.Background()

	tests := []struct {
		name       string
		action     Action
		wantAction string
		wantErr    bool
	}{
		{
			name:       "click",
			action:     Action{Type: "click", Value: "10,20"},
			wantAction: "click:10,20",
		},
		{
			name:       "tap alias",
			action:     Action{Type: "tap", Value: "30,40"},
			wantAction: "click:30,40",
		},
		{
			name:       "type",
			action:     Action{Type: "type", Value: "hello"},
			wantAction: "type:hello",
		},
		{
			name:       "text alias",
			action:     Action{Type: "text", Value: "world"},
			wantAction: "type:world",
		},
		{
			name:       "clear",
			action:     Action{Type: "clear"},
			wantAction: "clear",
		},
		{
			name:       "key_press",
			action:     Action{Type: "key_press", Value: "ENTER"},
			wantAction: "key:ENTER",
		},
		{
			name:       "back",
			action:     Action{Type: "back"},
			wantAction: "back",
		},
		{
			name:       "home",
			action:     Action{Type: "home"},
			wantAction: "home",
		},
		{
			name:    "unknown",
			action:  Action{Type: "fly"},
			wantErr: true,
		},
		{
			name:    "bad click coords",
			action:  Action{Type: "click", Value: "bad"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec.actions = nil
			err := executeAction(ctx, exec, tt.action)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, exec.actions,
					tt.wantAction)
			}
		})
	}
}

func TestExecuteAction_Wait(t *testing.T) {
	exec := &mockExecutor{}
	ctx := context.Background()

	start := time.Now()
	err := executeAction(ctx, exec, Action{
		Type:  "wait",
		Value: "10ms",
	})
	assert.NoError(t, err)
	assert.GreaterOrEqual(t,
		time.Since(start), 10*time.Millisecond,
	)
}

func TestExecuteAction_WaitInvalidDuration(t *testing.T) {
	exec := &mockExecutor{}
	ctx := context.Background()

	// Invalid duration falls back to 1s; use a short
	// context timeout to avoid waiting.
	ctx, cancel := context.WithTimeout(
		ctx, 50*time.Millisecond,
	)
	defer cancel()

	err := executeAction(ctx, exec, Action{
		Type:  "wait",
		Value: "not-a-duration",
	})
	// Context times out before the 1s default wait.
	assert.Error(t, err)
}

func TestExecuteAction_LongPress(t *testing.T) {
	exec := &mockExecutor{}
	ctx := context.Background()

	err := executeAction(ctx, exec, Action{
		Type:  "long_press",
		Value: "100,200",
	})
	assert.NoError(t, err)
	assert.Contains(t, exec.actions, "longpress:100,200")
}

func TestExecuteAction_Swipe(t *testing.T) {
	exec := &mockExecutor{}
	ctx := context.Background()

	err := executeAction(ctx, exec, Action{
		Type:  "swipe",
		Value: "10,20,30,40",
	})
	assert.NoError(t, err)
	assert.Contains(t, exec.actions, "swipe:10,20,30,40")
}

func TestExecuteAction_SwipeBadCoords(t *testing.T) {
	exec := &mockExecutor{}
	ctx := context.Background()

	err := executeAction(ctx, exec, Action{
		Type:  "swipe",
		Value: "bad",
	})
	assert.Error(t, err)
}

func TestExecuteAction_LongPressBadCoords(t *testing.T) {
	exec := &mockExecutor{}
	ctx := context.Background()

	err := executeAction(ctx, exec, Action{
		Type:  "long_press",
		Value: "bad",
	})
	assert.Error(t, err)
}

func TestReproductionResult_Fields(t *testing.T) {
	r := &ReproductionResult{
		BugID:      "BUG-100",
		Reproduced: true,
		Attempts:   2,
		ActionSequence: []Action{
			{Type: "click", Value: "10,20"},
		},
		Screenshots: []string{"shot-1", "shot-2"},
		Evidence:    "Bug confirmed",
		Duration:    5 * time.Second,
	}

	assert.Equal(t, "BUG-100", r.BugID)
	assert.True(t, r.Reproduced)
	assert.Equal(t, 2, r.Attempts)
	assert.Len(t, r.ActionSequence, 1)
	assert.Len(t, r.Screenshots, 2)
	assert.Equal(t, "Bug confirmed", r.Evidence)
	assert.Equal(t, 5*time.Second, r.Duration)
}

// --- varying provider helper ---

// varyingVisionProvider returns a different response on
// each Vision call, cycling through the responses slice.
type varyingVisionProvider struct {
	responses []string
	callIndex int
}

func (v *varyingVisionProvider) Chat(
	_ context.Context, _ []llm.Message,
) (*llm.Response, error) {
	return &llm.Response{Content: "ok"}, nil
}

func (v *varyingVisionProvider) Vision(
	_ context.Context, _ []byte, _ string,
) (*llm.Response, error) {
	idx := v.callIndex
	if idx >= len(v.responses) {
		idx = len(v.responses) - 1
	}
	v.callIndex++
	resp := v.responses[idx]
	return &llm.Response{Content: resp}, nil
}

func (v *varyingVisionProvider) Name() string {
	return "varying"
}

func (v *varyingVisionProvider) SupportsVision() bool {
	return true
}

// --- confirmBug prompt tests ---

func TestConfirmBug_PromptContainsBugDescription(
	t *testing.T,
) {
	exec := &mockExecutor{screenshotImg: []byte("png")}
	prov := &mockVisionProvider{
		supportsVis: true,
		response:    "NO",
	}

	br := NewBugReproducer(exec, prov,
		WithMaxRetries(1),
		WithActionDelay(0),
		WithScreenshotDelay(0),
	)

	bug := Bug{
		ID:          "BUG-PROMPT",
		Description: "Button text is clipped at edges",
		Severity:    "medium",
	}

	_, _ = br.Reproduce(context.Background(), bug)

	assert.Contains(t, prov.lastPrompt,
		"Button text is clipped at edges")
	assert.True(t,
		strings.Contains(prov.lastPrompt, "YES or NO") ||
			strings.Contains(prov.lastPrompt, "YES") &&
				strings.Contains(prov.lastPrompt, "NO"),
	)
}
