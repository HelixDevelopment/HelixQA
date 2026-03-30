// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// defaultBridgeTimeout is the maximum wall-clock time
// allowed for a single CLI invocation.
const defaultBridgeTimeout = 120 * time.Second

// BridgedCLIProvider wraps an external CLI tool (e.g.
// "claude", "qwen-coder", "opencode") as an LLM provider.
// It shells out to the CLI with --json --print flags and
// parses the JSON response.
//
// This enables using CLI-only LLM tools that have no HTTP
// API in the HelixQA pipeline — the same pattern used by
// HelixAgent's ClaudeCLIProvider.
type BridgedCLIProvider struct {
	// cliPath is the absolute or $PATH-relative path to the
	// CLI binary (e.g. "/usr/local/bin/claude").
	cliPath string

	// cliName identifies the tool: "claude", "qwen-coder",
	// "opencode", etc.
	cliName string

	// model is the model identifier passed to the CLI
	// (optional — some CLIs infer it).
	model string

	// timeout caps a single CLI invocation.
	timeout time.Duration

	// cmdRunner is an abstraction for exec.CommandContext
	// to allow testing without real binaries. When nil,
	// the real os/exec is used.
	cmdRunner CommandRunner
}

// CommandRunner abstracts CLI execution for testability.
type CommandRunner interface {
	// Run executes the command described by name and args,
	// returning combined stdout/stderr and any error.
	Run(ctx context.Context, name string,
		args ...string) ([]byte, error)
}

// execRunner is the default CommandRunner using os/exec.
type execRunner struct{}

func (r *execRunner) Run(
	ctx context.Context,
	name string,
	args ...string,
) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Include stderr in the error for diagnostics.
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return nil, fmt.Errorf(
				"bridge-cli %s: %w: %s",
				filepath.Base(name), err, errMsg,
			)
		}
		return nil, fmt.Errorf(
			"bridge-cli %s: %w",
			filepath.Base(name), err,
		)
	}
	return stdout.Bytes(), nil
}

// NewBridgedCLIProvider creates a provider that wraps the
// given CLI tool. cliPath may be an absolute path or a
// $PATH-relative name. cliName identifies the tool for
// logging and capability detection.
func NewBridgedCLIProvider(
	cliPath, cliName, model string,
) *BridgedCLIProvider {
	return &BridgedCLIProvider{
		cliPath:   cliPath,
		cliName:   cliName,
		model:     model,
		timeout:   defaultBridgeTimeout,
		cmdRunner: &execRunner{},
	}
}

// WithTimeout overrides the default per-invocation timeout.
func (p *BridgedCLIProvider) WithTimeout(
	d time.Duration,
) *BridgedCLIProvider {
	if d > 0 {
		p.timeout = d
	}
	return p
}

// WithCommandRunner injects a custom CommandRunner for
// testing.
func (p *BridgedCLIProvider) WithCommandRunner(
	cr CommandRunner,
) *BridgedCLIProvider {
	p.cmdRunner = cr
	return p
}

// Name returns the canonical provider identifier, prefixed
// with "bridge-" to distinguish from native API providers.
func (p *BridgedCLIProvider) Name() string {
	return "bridge-" + p.cliName
}

// SupportsVision reports whether the CLI tool supports
// image inputs. Currently only the Claude CLI supports
// the --image flag.
func (p *BridgedCLIProvider) SupportsVision() bool {
	return p.cliName == "claude"
}

// cliJSONResponse is the expected shape of a --json CLI
// response. Different CLIs may use slightly different
// field names, so we handle the common variants.
type cliJSONResponse struct {
	// Result is the primary content field (Claude CLI).
	Result string `json:"result"`

	// Content is an alternative field name used by some
	// CLIs.
	Content string `json:"content"`

	// Text is another alternative.
	Text string `json:"text"`

	// Model is the model used, when reported.
	Model string `json:"model"`

	// Usage holds optional token counts.
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Chat sends a multi-turn conversation to the CLI tool by
// concatenating messages into a single prompt string. The
// CLI is invoked with --json --print flags.
func (p *BridgedCLIProvider) Chat(
	ctx context.Context,
	messages []Message,
) (*Response, error) {
	prompt := p.buildPrompt(messages)
	if prompt == "" {
		return nil, fmt.Errorf(
			"bridge-cli %s: empty prompt", p.cliName,
		)
	}

	args := p.buildArgs(prompt, "")
	return p.invoke(ctx, args)
}

// Vision sends a screenshot with a text prompt to the CLI
// tool. The image is written to a temporary file and
// passed via --image. Only supported when
// SupportsVision() is true.
func (p *BridgedCLIProvider) Vision(
	ctx context.Context,
	image []byte,
	prompt string,
) (*Response, error) {
	if !p.SupportsVision() {
		return nil, fmt.Errorf(
			"bridge-cli %s: vision not supported",
			p.cliName,
		)
	}
	if len(image) == 0 {
		return nil, fmt.Errorf(
			"bridge-cli %s: empty image data",
			p.cliName,
		)
	}

	// Write image to a temp file.
	tmpFile, err := os.CreateTemp("",
		"helixqa-vision-*.png")
	if err != nil {
		return nil, fmt.Errorf(
			"bridge-cli %s: create temp: %w",
			p.cliName, err,
		)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(image); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf(
			"bridge-cli %s: write temp: %w",
			p.cliName, err,
		)
	}
	tmpFile.Close()

	args := p.buildArgs(prompt, tmpPath)
	return p.invoke(ctx, args)
}

// buildPrompt concatenates messages into a single prompt
// string suitable for CLI --print mode. System messages
// are prefixed, assistant messages are labeled.
func (p *BridgedCLIProvider) buildPrompt(
	messages []Message,
) string {
	var parts []string
	for _, m := range messages {
		switch m.Role {
		case RoleSystem:
			parts = append(parts,
				"[System] "+m.Content)
		case RoleAssistant:
			parts = append(parts,
				"[Assistant] "+m.Content)
		default:
			parts = append(parts, m.Content)
		}
	}
	return strings.Join(parts, "\n\n")
}

// buildArgs constructs the CLI argument list. imagePath
// is empty for chat-only calls.
func (p *BridgedCLIProvider) buildArgs(
	prompt, imagePath string,
) []string {
	args := []string{"--json", "--print", prompt}
	if p.model != "" {
		args = append(args, "--model", p.model)
	}
	if imagePath != "" {
		args = append(args, "--image", imagePath)
	}
	return args
}

// invoke runs the CLI and parses the JSON response.
func (p *BridgedCLIProvider) invoke(
	ctx context.Context,
	args []string,
) (*Response, error) {
	callCtx, cancel := context.WithTimeout(
		ctx, p.timeout,
	)
	defer cancel()

	output, err := p.cmdRunner.Run(
		callCtx, p.cliPath, args...,
	)
	if err != nil {
		return nil, err
	}

	return p.parseResponse(output)
}

// parseResponse extracts the content from CLI JSON output.
// Falls back to treating the entire output as plain text
// when JSON parsing fails.
func (p *BridgedCLIProvider) parseResponse(
	output []byte,
) (*Response, error) {
	trimmed := bytes.TrimSpace(output)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf(
			"bridge-cli %s: empty response",
			p.cliName,
		)
	}

	// Try JSON parse first.
	var parsed cliJSONResponse
	if err := json.Unmarshal(trimmed, &parsed); err == nil {
		content := parsed.Result
		if content == "" {
			content = parsed.Content
		}
		if content == "" {
			content = parsed.Text
		}
		if content == "" {
			// JSON but no recognized content field — use
			// the raw output as-is.
			content = string(trimmed)
		}

		resp := &Response{
			Content: content,
			Model:   parsed.Model,
		}
		if parsed.Usage != nil {
			resp.InputTokens = parsed.Usage.InputTokens
			resp.OutputTokens = parsed.Usage.OutputTokens
		}
		if resp.Model == "" {
			resp.Model = p.model
		}
		return resp, nil
	}

	// Not JSON — treat as plain text response.
	return &Response{
		Content: string(trimmed),
		Model:   p.model,
	}, nil
}
