// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCmdRunner is a test double for CommandRunner.
type mockCmdRunner struct {
	output    []byte
	err       error
	lastName  string
	lastArgs  []string
	callCount int
}

func (m *mockCmdRunner) Run(
	_ context.Context,
	name string,
	args ...string,
) ([]byte, error) {
	m.callCount++
	m.lastName = name
	m.lastArgs = args
	return m.output, m.err
}

// TestBridgedCLIProvider_Name verifies the provider name
// is prefixed with "bridge-".
func TestBridgedCLIProvider_Name(t *testing.T) {
	tests := []struct {
		cliName string
		want    string
	}{
		{"claude", "bridge-claude"},
		{"qwen-coder", "bridge-qwen-coder"},
		{"opencode", "bridge-opencode"},
	}
	for _, tt := range tests {
		t.Run(tt.cliName, func(t *testing.T) {
			p := NewBridgedCLIProvider(
				"/usr/bin/"+tt.cliName,
				tt.cliName, "",
			)
			assert.Equal(t, tt.want, p.Name())
		})
	}
}

// TestBridgedCLIProvider_SupportsVision verifies that only
// the Claude CLI reports vision support.
func TestBridgedCLIProvider_SupportsVision(t *testing.T) {
	tests := []struct {
		cliName string
		want    bool
	}{
		{"claude", true},
		{"qwen-coder", false},
		{"opencode", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.cliName, func(t *testing.T) {
			p := NewBridgedCLIProvider("bin", tt.cliName, "")
			assert.Equal(t, tt.want, p.SupportsVision())
		})
	}
}

// TestBridgedCLIProvider_ImplementsProvider verifies
// compile-time interface satisfaction.
func TestBridgedCLIProvider_ImplementsProvider(t *testing.T) {
	var _ Provider = (*BridgedCLIProvider)(nil)
}

// TestBridgedCLIProvider_Chat_JSONResponse verifies that
// a JSON response with the "result" field is parsed.
func TestBridgedCLIProvider_Chat_JSONResponse(t *testing.T) {
	resp := cliJSONResponse{
		Result: "Hello from Claude CLI",
		Model:  "claude-sonnet-4-20250514",
		Usage: &struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		}{InputTokens: 15, OutputTokens: 42},
	}
	data, _ := json.Marshal(resp)

	runner := &mockCmdRunner{output: data}
	p := NewBridgedCLIProvider(
		"/usr/bin/claude", "claude", "sonnet",
	).WithCommandRunner(runner)

	result, err := p.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "What is 2+2?"},
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello from Claude CLI", result.Content)
	assert.Equal(t, "claude-sonnet-4-20250514", result.Model)
	assert.Equal(t, 15, result.InputTokens)
	assert.Equal(t, 42, result.OutputTokens)

	// Verify CLI was invoked correctly.
	assert.Equal(t, "/usr/bin/claude", runner.lastName)
	assert.Contains(t, runner.lastArgs, "--json")
	assert.Contains(t, runner.lastArgs, "--print")
	assert.Contains(t, runner.lastArgs, "--model")
	assert.Contains(t, runner.lastArgs, "sonnet")
}

// TestBridgedCLIProvider_Chat_ContentField verifies
// fallback to "content" field when "result" is empty.
func TestBridgedCLIProvider_Chat_ContentField(t *testing.T) {
	data := []byte(`{"content":"from content field"}`)
	runner := &mockCmdRunner{output: data}
	p := NewBridgedCLIProvider(
		"opencode", "opencode", "",
	).WithCommandRunner(runner)

	result, err := p.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "hello"},
	})
	require.NoError(t, err)
	assert.Equal(t, "from content field", result.Content)
}

// TestBridgedCLIProvider_Chat_TextField verifies fallback
// to "text" field.
func TestBridgedCLIProvider_Chat_TextField(t *testing.T) {
	data := []byte(`{"text":"from text field"}`)
	runner := &mockCmdRunner{output: data}
	p := NewBridgedCLIProvider(
		"qwen", "qwen-coder", "",
	).WithCommandRunner(runner)

	result, err := p.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "hello"},
	})
	require.NoError(t, err)
	assert.Equal(t, "from text field", result.Content)
}

// TestBridgedCLIProvider_Chat_PlainTextFallback verifies
// that non-JSON output is returned as-is.
func TestBridgedCLIProvider_Chat_PlainTextFallback(
	t *testing.T,
) {
	runner := &mockCmdRunner{
		output: []byte("Just plain text response"),
	}
	p := NewBridgedCLIProvider(
		"mytool", "mytool", "",
	).WithCommandRunner(runner)

	result, err := p.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "hello"},
	})
	require.NoError(t, err)
	assert.Equal(t, "Just plain text response",
		result.Content)
}

// TestBridgedCLIProvider_Chat_EmptyResponse returns error
// on empty CLI output.
func TestBridgedCLIProvider_Chat_EmptyResponse(
	t *testing.T,
) {
	runner := &mockCmdRunner{output: []byte("")}
	p := NewBridgedCLIProvider(
		"cli", "cli", "",
	).WithCommandRunner(runner)

	_, err := p.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "hello"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response")
}

// TestBridgedCLIProvider_Chat_CLIError propagates the
// underlying CLI error.
func TestBridgedCLIProvider_Chat_CLIError(t *testing.T) {
	runner := &mockCmdRunner{
		err: errors.New("exit status 1"),
	}
	p := NewBridgedCLIProvider(
		"broken", "broken", "",
	).WithCommandRunner(runner)

	_, err := p.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "hello"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit status 1")
}

// TestBridgedCLIProvider_Chat_EmptyMessages returns error
// for empty prompt.
func TestBridgedCLIProvider_Chat_EmptyMessages(
	t *testing.T,
) {
	runner := &mockCmdRunner{}
	p := NewBridgedCLIProvider(
		"cli", "cli", "",
	).WithCommandRunner(runner)

	_, err := p.Chat(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty prompt")
}

// TestBridgedCLIProvider_Vision_NotSupported verifies that
// non-Claude CLIs reject vision calls.
func TestBridgedCLIProvider_Vision_NotSupported(
	t *testing.T,
) {
	p := NewBridgedCLIProvider("opencode", "opencode", "")
	_, err := p.Vision(
		context.Background(),
		[]byte{0x89, 0x50, 0x4E, 0x47},
		"describe this",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vision not supported")
}

// TestBridgedCLIProvider_Vision_EmptyImage returns error.
func TestBridgedCLIProvider_Vision_EmptyImage(
	t *testing.T,
) {
	p := NewBridgedCLIProvider(
		"claude", "claude", "",
	)
	_, err := p.Vision(
		context.Background(), nil, "describe this",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty image")
}

// TestBridgedCLIProvider_Vision_Success verifies that
// vision calls pass --image to the CLI.
func TestBridgedCLIProvider_Vision_Success(t *testing.T) {
	data := []byte(`{"result":"I see a login screen"}`)
	runner := &mockCmdRunner{output: data}
	p := NewBridgedCLIProvider(
		"/usr/bin/claude", "claude", "",
	).WithCommandRunner(runner)

	result, err := p.Vision(
		context.Background(),
		[]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A},
		"describe what you see",
	)
	require.NoError(t, err)
	assert.Equal(t, "I see a login screen", result.Content)

	// Verify --image flag was passed.
	assert.Contains(t, runner.lastArgs, "--image")
	// The image path should be a temp file.
	imageIdx := -1
	for i, a := range runner.lastArgs {
		if a == "--image" && i+1 < len(runner.lastArgs) {
			imageIdx = i + 1
			break
		}
	}
	require.NotEqual(t, -1, imageIdx,
		"--image flag should have a path argument")
	assert.Contains(t, runner.lastArgs[imageIdx],
		"helixqa-vision")
}

// TestBridgedCLIProvider_WithTimeout verifies custom
// timeout is stored.
func TestBridgedCLIProvider_WithTimeout(t *testing.T) {
	p := NewBridgedCLIProvider("cli", "cli", "").
		WithTimeout(5 * time.Minute)
	assert.Equal(t, 5*time.Minute, p.timeout)
}

// TestBridgedCLIProvider_WithTimeout_IgnoresZero verifies
// that zero timeout is ignored.
func TestBridgedCLIProvider_WithTimeout_IgnoresZero(
	t *testing.T,
) {
	p := NewBridgedCLIProvider("cli", "cli", "").
		WithTimeout(0)
	assert.Equal(t, defaultBridgeTimeout, p.timeout)
}

// TestBridgedCLIProvider_BuildPrompt_MultiTurn verifies
// that multi-turn messages are concatenated with role
// labels.
func TestBridgedCLIProvider_BuildPrompt_MultiTurn(
	t *testing.T,
) {
	p := NewBridgedCLIProvider("cli", "cli", "")
	prompt := p.buildPrompt([]Message{
		{Role: RoleSystem, Content: "You are a QA agent"},
		{Role: RoleUser, Content: "Analyze this screen"},
		{Role: RoleAssistant, Content: "I see a form"},
		{Role: RoleUser, Content: "What fields?"},
	})
	assert.Contains(t, prompt, "[System] You are a QA agent")
	assert.Contains(t, prompt, "Analyze this screen")
	assert.Contains(t, prompt, "[Assistant] I see a form")
	assert.Contains(t, prompt, "What fields?")
}

// TestBridgedCLIProvider_ModelFallback verifies that the
// model from the provider config is used when the JSON
// response does not include a model field.
func TestBridgedCLIProvider_ModelFallback(t *testing.T) {
	data := []byte(`{"result":"ok"}`)
	runner := &mockCmdRunner{output: data}
	p := NewBridgedCLIProvider(
		"cli", "cli", "my-model-v2",
	).WithCommandRunner(runner)

	result, err := p.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "hello"},
	})
	require.NoError(t, err)
	assert.Equal(t, "my-model-v2", result.Model)
}

// TestBridgedCLIProvider_BuildArgs_NoModel verifies that
// --model is omitted when model is empty.
func TestBridgedCLIProvider_BuildArgs_NoModel(
	t *testing.T,
) {
	p := NewBridgedCLIProvider("cli", "cli", "")
	args := p.buildArgs("hello", "")
	for _, a := range args {
		if a == "--model" {
			t.Fatal("--model should not be present " +
				"when model is empty")
		}
	}
	assert.Contains(t, args, "--json")
	assert.Contains(t, args, "--print")
	assert.Contains(t, args, "hello")
}

// TestBridgedCLIProvider_WhitespaceOnlyOutput returns
// error.
func TestBridgedCLIProvider_WhitespaceOnlyOutput(
	t *testing.T,
) {
	runner := &mockCmdRunner{output: []byte("   \n\t  ")}
	p := NewBridgedCLIProvider(
		"cli", "cli", "",
	).WithCommandRunner(runner)

	_, err := p.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "hello"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response")
}

// TestBridgedCLIProvider_JSONNoContentFields verifies
// that JSON with no recognized content fields falls back
// to the raw JSON string.
func TestBridgedCLIProvider_JSONNoContentFields(
	t *testing.T,
) {
	data := []byte(`{"status":"ok","data":123}`)
	runner := &mockCmdRunner{output: data}
	p := NewBridgedCLIProvider(
		"cli", "cli", "",
	).WithCommandRunner(runner)

	result, err := p.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "hello"},
	})
	require.NoError(t, err)
	// Falls back to raw JSON string.
	assert.True(t, strings.Contains(
		result.Content, "status"))
}
