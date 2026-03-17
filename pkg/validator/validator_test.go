// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/detector"
)

// mockRunner implements detector.CommandRunner for testing.
type mockRunner struct {
	responses map[string]mockResponse
}

type mockResponse struct {
	output []byte
	err    error
}

func newMockRunner() *mockRunner {
	return &mockRunner{
		responses: make(map[string]mockResponse),
	}
}

func (m *mockRunner) On(
	key string, output []byte, err error,
) {
	m.responses[key] = mockResponse{output: output, err: err}
}

func (m *mockRunner) Run(
	ctx context.Context, name string, args ...string,
) ([]byte, error) {
	key := name
	if len(args) > 0 {
		key = name + " " + strings.Join(args, " ")
	}
	if resp, ok := m.responses[key]; ok {
		return resp.output, resp.err
	}
	for k, resp := range m.responses {
		if strings.HasPrefix(key, k) {
			return resp.output, resp.err
		}
	}
	if resp, ok := m.responses[name]; ok {
		return resp.output, resp.err
	}
	return nil, fmt.Errorf("no mock: %s", key)
}

// --- Constructor tests ---

func TestNew_Defaults(t *testing.T) {
	mock := newMockRunner()
	det := detector.New(
		config.PlatformDesktop,
		detector.WithCommandRunner(mock),
	)
	v := New(det)
	assert.NotNil(t, v)
	assert.Equal(t, "evidence", v.EvidenceDir())
	assert.Equal(t, 0, v.TotalCount())
}

func TestNew_WithOptions(t *testing.T) {
	mock := newMockRunner()
	det := detector.New(
		config.PlatformDesktop,
		detector.WithCommandRunner(mock),
	)
	v := New(
		det,
		WithEvidenceDir("/tmp/ev"),
	)
	assert.Equal(t, "/tmp/ev", v.EvidenceDir())
}

func TestNew_WithScreenshotFunc(t *testing.T) {
	mock := newMockRunner()
	det := detector.New(
		config.PlatformDesktop,
		detector.WithCommandRunner(mock),
	)
	called := false
	v := New(
		det,
		WithScreenshotFunc(func(
			ctx context.Context, name string,
		) (string, error) {
			called = true
			return "/tmp/" + name + ".png", nil
		}),
	)
	assert.NotNil(t, v)
	_ = called // Will be tested in ValidateStep tests.
}

// --- ValidateStep tests ---

func TestValidateStep_Passed(t *testing.T) {
	mock := newMockRunner()
	mock.On("pgrep", []byte("12345"), nil)

	det := detector.New(
		config.PlatformDesktop,
		detector.WithCommandRunner(mock),
	)
	v := New(det)

	result, err := v.ValidateStep(
		context.Background(),
		"launch-app",
		config.PlatformDesktop,
	)
	require.NoError(t, err)
	assert.Equal(t, StepPassed, result.Status)
	assert.Equal(t, "launch-app", result.StepName)
	assert.Equal(t, config.PlatformDesktop, result.Platform)
	assert.False(t, result.StartTime.IsZero())
	assert.False(t, result.EndTime.IsZero())
}

func TestValidateStep_Failed_Crash(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"pgrep",
		[]byte(""),
		fmt.Errorf("not found"),
	)

	det := detector.New(
		config.PlatformDesktop,
		detector.WithCommandRunner(mock),
	)
	v := New(det)

	result, err := v.ValidateStep(
		context.Background(),
		"click-button",
		config.PlatformDesktop,
	)
	require.NoError(t, err)
	assert.Equal(t, StepFailed, result.Status)
	assert.Contains(t, result.Error, "crash detected")
}

func TestValidateStep_WithScreenshots(t *testing.T) {
	mock := newMockRunner()
	mock.On("pgrep", []byte("12345"), nil)

	det := detector.New(
		config.PlatformDesktop,
		detector.WithCommandRunner(mock),
	)
	screenshots := []string{}
	v := New(
		det,
		WithScreenshotFunc(func(
			ctx context.Context, name string,
		) (string, error) {
			path := "/tmp/" + name + ".png"
			screenshots = append(screenshots, path)
			return path, nil
		}),
	)

	result, err := v.ValidateStep(
		context.Background(),
		"step1",
		config.PlatformDesktop,
	)
	require.NoError(t, err)
	assert.Equal(t, StepPassed, result.Status)
	assert.NotEmpty(t, result.PreScreenshot)
	assert.NotEmpty(t, result.PostScreenshot)
	assert.Len(t, screenshots, 2) // pre + post
}

func TestValidateStep_ScreenshotError(t *testing.T) {
	mock := newMockRunner()
	mock.On("pgrep", []byte("12345"), nil)

	det := detector.New(
		config.PlatformDesktop,
		detector.WithCommandRunner(mock),
	)
	v := New(
		det,
		WithScreenshotFunc(func(
			ctx context.Context, name string,
		) (string, error) {
			return "", fmt.Errorf("screenshot error")
		}),
	)

	result, err := v.ValidateStep(
		context.Background(),
		"step1",
		config.PlatformDesktop,
	)
	require.NoError(t, err)
	// Should still pass — screenshot failure is non-fatal.
	assert.Equal(t, StepPassed, result.Status)
	assert.Empty(t, result.PreScreenshot)
	assert.Empty(t, result.PostScreenshot)
}

// --- Results tracking tests ---

func TestResults_Empty(t *testing.T) {
	mock := newMockRunner()
	det := detector.New(
		config.PlatformDesktop,
		detector.WithCommandRunner(mock),
	)
	v := New(det)
	assert.Empty(t, v.Results())
	assert.Equal(t, 0, v.TotalCount())
	assert.Equal(t, 0, v.PassedCount())
	assert.Equal(t, 0, v.FailedCount())
}

func TestResults_AfterValidation(t *testing.T) {
	mock := newMockRunner()
	mock.On("pgrep", []byte("12345"), nil)

	det := detector.New(
		config.PlatformDesktop,
		detector.WithCommandRunner(mock),
	)
	v := New(det)

	_, _ = v.ValidateStep(
		context.Background(), "step1",
		config.PlatformDesktop,
	)
	_, _ = v.ValidateStep(
		context.Background(), "step2",
		config.PlatformDesktop,
	)

	assert.Equal(t, 2, v.TotalCount())
	assert.Equal(t, 2, v.PassedCount())
	assert.Equal(t, 0, v.FailedCount())
	assert.Len(t, v.Results(), 2)
}

func TestResults_MixedPassFail(t *testing.T) {
	mock := newMockRunner()
	det := detector.New(
		config.PlatformDesktop,
		detector.WithCommandRunner(mock),
	)
	v := New(det)

	// Step 1: pass (process alive).
	mock.On("pgrep", []byte("12345"), nil)
	_, _ = v.ValidateStep(
		context.Background(), "step1",
		config.PlatformDesktop,
	)

	// Step 2: fail (process dead).
	mock.responses = map[string]mockResponse{}
	mock.On(
		"pgrep",
		[]byte(""),
		fmt.Errorf("not found"),
	)
	_, _ = v.ValidateStep(
		context.Background(), "step2",
		config.PlatformDesktop,
	)

	assert.Equal(t, 2, v.TotalCount())
	assert.Equal(t, 1, v.PassedCount())
	assert.Equal(t, 1, v.FailedCount())
}

func TestReset(t *testing.T) {
	mock := newMockRunner()
	mock.On("pgrep", []byte("12345"), nil)

	det := detector.New(
		config.PlatformDesktop,
		detector.WithCommandRunner(mock),
	)
	v := New(det)

	_, _ = v.ValidateStep(
		context.Background(), "step1",
		config.PlatformDesktop,
	)
	assert.Equal(t, 1, v.TotalCount())

	v.Reset()
	assert.Equal(t, 0, v.TotalCount())
	assert.Empty(t, v.Results())
}

func TestEvidencePath(t *testing.T) {
	mock := newMockRunner()
	det := detector.New(
		config.PlatformDesktop,
		detector.WithCommandRunner(mock),
	)
	v := New(det, WithEvidenceDir("/data/evidence"))
	assert.Equal(t,
		"/data/evidence/screenshot.png",
		v.EvidencePath("screenshot.png"),
	)
}

// --- StepStatus constants ---

func TestStepStatusConstants(t *testing.T) {
	assert.Equal(t, StepStatus("passed"), StepPassed)
	assert.Equal(t, StepStatus("failed"), StepFailed)
	assert.Equal(t, StepStatus("skipped"), StepSkipped)
	assert.Equal(t, StepStatus("error"), StepError)
}

// --- StepResult fields ---

func TestStepResult_Fields(t *testing.T) {
	mock := newMockRunner()
	mock.On("pgrep", []byte("12345"), nil)

	det := detector.New(
		config.PlatformDesktop,
		detector.WithCommandRunner(mock),
	)
	v := New(det)

	result, err := v.ValidateStep(
		context.Background(), "test-step",
		config.PlatformDesktop,
	)
	require.NoError(t, err)
	assert.Equal(t, "test-step", result.StepName)
	assert.Equal(t, config.PlatformDesktop, result.Platform)
	assert.NotNil(t, result.Detection)
	assert.True(t, result.Duration >= 0)
}

// --- Concurrent access ---

func TestValidator_ConcurrentAccess(t *testing.T) {
	mock := newMockRunner()
	mock.On("pgrep", []byte("12345"), nil)

	det := detector.New(
		config.PlatformDesktop,
		detector.WithCommandRunner(mock),
	)
	v := New(det)

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			_, _ = v.ValidateStep(
				context.Background(),
				fmt.Sprintf("step-%d", n),
				config.PlatformDesktop,
			)
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 10, v.TotalCount())
}
