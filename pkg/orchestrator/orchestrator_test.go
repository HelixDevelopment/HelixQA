// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.challenges/pkg/bank"
	"digital.vasic.challenges/pkg/challenge"

	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/detector"
	"digital.vasic.helixqa/pkg/reporter"
	"digital.vasic.helixqa/pkg/validator"
)

// mockRunner for detector tests.
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

// mockChallengeRunner implements runner.Runner for tests.
type mockChallengeRunner struct {
	results map[challenge.ID]*challenge.Result
	err     error
}

func (m *mockChallengeRunner) Run(
	ctx context.Context,
	id challenge.ID,
	cfg *challenge.Config,
) (*challenge.Result, error) {
	if m.err != nil {
		return nil, m.err
	}
	if r, ok := m.results[id]; ok {
		return r, nil
	}
	return &challenge.Result{
		ChallengeID:   id,
		ChallengeName: string(id),
		Status:        challenge.StatusPassed,
		StartTime:     time.Now(),
		EndTime:       time.Now(),
	}, nil
}

func (m *mockChallengeRunner) RunAll(
	ctx context.Context,
	cfg *challenge.Config,
) ([]*challenge.Result, error) {
	return nil, nil
}

func (m *mockChallengeRunner) RunSequence(
	ctx context.Context,
	ids []challenge.ID,
	cfg *challenge.Config,
) ([]*challenge.Result, error) {
	return nil, nil
}

func (m *mockChallengeRunner) RunParallel(
	ctx context.Context,
	ids []challenge.ID,
	cfg *challenge.Config,
	maxConcurrency int,
) ([]*challenge.Result, error) {
	return nil, nil
}

// createTestBank creates a temporary test bank file.
func createTestBank(t *testing.T, dir string) string {
	t.Helper()
	bankFile := bank.BankFile{
		Version: "1.0",
		Name:    "test-bank",
		Challenges: []challenge.Definition{
			{
				ID:       "test-1",
				Name:     "Test Challenge 1",
				Category: "unit",
			},
			{
				ID:       "test-2",
				Name:     "Test Challenge 2",
				Category: "unit",
			},
		},
	}
	data, err := json.Marshal(bankFile)
	require.NoError(t, err)

	path := filepath.Join(dir, "test-bank.json")
	err = os.WriteFile(path, data, 0644)
	require.NoError(t, err)
	return path
}

// --- Constructor tests ---

func TestNew(t *testing.T) {
	cfg := config.DefaultConfig()
	o := New(cfg)
	assert.NotNil(t, o)
	assert.Equal(t, cfg, o.config)
}

func TestNew_WithOptions(t *testing.T) {
	cfg := config.DefaultConfig()
	b := bank.New()
	rep := reporter.New()

	o := New(
		cfg,
		WithBank(b),
		WithReporter(rep),
	)
	assert.NotNil(t, o)
	assert.Equal(t, b, o.bank)
	assert.Equal(t, rep, o.reporter)
}

func TestNew_WithDetector(t *testing.T) {
	cfg := config.DefaultConfig()
	mock := newMockRunner()
	det := detector.New(
		config.PlatformDesktop,
		detector.WithCommandRunner(mock),
	)

	o := New(cfg, WithDetector(det))
	assert.Equal(t, det, o.detector)
}

func TestNew_WithValidator(t *testing.T) {
	cfg := config.DefaultConfig()
	mock := newMockRunner()
	det := detector.New(
		config.PlatformDesktop,
		detector.WithCommandRunner(mock),
	)
	val := validator.New(det)

	o := New(cfg, WithValidator(val))
	assert.Equal(t, val, o.val)
}

func TestNew_WithRunner(t *testing.T) {
	cfg := config.DefaultConfig()
	r := &mockChallengeRunner{}

	o := New(cfg, WithRunner(r))
	assert.Equal(t, r, o.runner)
}

// --- Run tests ---

func TestRun_NoBanks(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Banks = nil

	o := New(cfg)
	_, err := o.Run(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "load banks")
}

func TestRun_BankNotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Banks = []string{"/nonexistent/path"}

	o := New(cfg)
	_, err := o.Run(context.Background())
	assert.Error(t, err)
}

func TestRun_WithTestBank(t *testing.T) {
	dir := t.TempDir()
	bankPath := createTestBank(t, dir)

	cfg := config.DefaultConfig()
	cfg.Banks = []string{bankPath}
	cfg.OutputDir = filepath.Join(dir, "output")
	cfg.Platforms = []Platform{config.PlatformDesktop}
	cfg.ValidateSteps = false // Skip validation for this test.
	cfg.Speed = config.SpeedFast

	mockRunner := &mockChallengeRunner{
		results: map[challenge.ID]*challenge.Result{
			"test-1": {
				ChallengeID:   "test-1",
				ChallengeName: "Test 1",
				Status:        challenge.StatusPassed,
				StartTime:     time.Now(),
				EndTime:       time.Now(),
			},
			"test-2": {
				ChallengeID:   "test-2",
				ChallengeName: "Test 2",
				Status:        challenge.StatusPassed,
				StartTime:     time.Now(),
				EndTime:       time.Now(),
			},
		},
	}

	o := New(cfg, WithRunner(mockRunner))

	result, err := o.Run(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.NotEmpty(t, result.ReportPath)
	assert.FileExists(t, result.ReportPath)
}

func TestRun_WithFailedChallenge(t *testing.T) {
	dir := t.TempDir()
	bankPath := createTestBank(t, dir)

	cfg := config.DefaultConfig()
	cfg.Banks = []string{bankPath}
	cfg.OutputDir = filepath.Join(dir, "output")
	cfg.Platforms = []Platform{config.PlatformDesktop}
	cfg.ValidateSteps = false
	cfg.Speed = config.SpeedFast

	mockRunner := &mockChallengeRunner{
		results: map[challenge.ID]*challenge.Result{
			"test-1": {
				ChallengeID:   "test-1",
				ChallengeName: "Test 1",
				Status:        challenge.StatusPassed,
			},
			"test-2": {
				ChallengeID:   "test-2",
				ChallengeName: "Test 2",
				Status:        challenge.StatusFailed,
				Error:         "assertion failed",
			},
		},
	}

	o := New(cfg, WithRunner(mockRunner))

	result, err := o.Run(context.Background())
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, 1, result.Report.FailedChallenges)
}

func TestRun_WithRunnerError(t *testing.T) {
	dir := t.TempDir()
	bankPath := createTestBank(t, dir)

	cfg := config.DefaultConfig()
	cfg.Banks = []string{bankPath}
	cfg.OutputDir = filepath.Join(dir, "output")
	cfg.Platforms = []Platform{config.PlatformDesktop}
	cfg.ValidateSteps = false
	cfg.Speed = config.SpeedFast

	mockRunner := &mockChallengeRunner{
		err: fmt.Errorf("runner failed"),
	}

	o := New(cfg, WithRunner(mockRunner))

	result, err := o.Run(context.Background())
	require.NoError(t, err)
	// Runner errors are logged but run continues.
	assert.False(t, result.Success)
}

func TestRun_CancelledContext(t *testing.T) {
	dir := t.TempDir()
	bankPath := createTestBank(t, dir)

	cfg := config.DefaultConfig()
	cfg.Banks = []string{bankPath}
	cfg.OutputDir = filepath.Join(dir, "output")
	cfg.Platforms = []Platform{config.PlatformDesktop}
	cfg.ValidateSteps = false

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	o := New(cfg)

	_, err := o.Run(ctx)
	assert.Error(t, err)
}

func TestRun_WithPreloadedBank(t *testing.T) {
	dir := t.TempDir()
	bankPath := createTestBank(t, dir)

	b := bank.New()
	err := b.LoadFile(bankPath)
	require.NoError(t, err)

	cfg := config.DefaultConfig()
	cfg.Banks = []string{bankPath} // Won't be used.
	cfg.OutputDir = filepath.Join(dir, "output")
	cfg.Platforms = []Platform{config.PlatformDesktop}
	cfg.ValidateSteps = false
	cfg.Speed = config.SpeedFast

	o := New(cfg, WithBank(b))

	result, err := o.Run(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result.Report)
	assert.Equal(t, 2, len(b.All()))
}

func TestRun_BankDir(t *testing.T) {
	dir := t.TempDir()
	bankDir := filepath.Join(dir, "banks")
	err := os.MkdirAll(bankDir, 0755)
	require.NoError(t, err)

	// Create bank file in directory.
	bankFile := bank.BankFile{
		Version: "1.0",
		Name:    "dir-bank",
		Challenges: []challenge.Definition{
			{ID: "dir-test-1", Name: "Dir Test 1"},
		},
	}
	data, err := json.Marshal(bankFile)
	require.NoError(t, err)
	err = os.WriteFile(
		filepath.Join(bankDir, "bank.json"), data, 0644,
	)
	require.NoError(t, err)

	cfg := config.DefaultConfig()
	cfg.Banks = []string{bankDir}
	cfg.OutputDir = filepath.Join(dir, "output")
	cfg.Platforms = []Platform{config.PlatformDesktop}
	cfg.ValidateSteps = false
	cfg.Speed = config.SpeedFast

	o := New(cfg)

	result, err := o.Run(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result.Report)
}

func TestRun_MultiplePlatforms(t *testing.T) {
	dir := t.TempDir()
	bankPath := createTestBank(t, dir)

	cfg := config.DefaultConfig()
	cfg.Banks = []string{bankPath}
	cfg.OutputDir = filepath.Join(dir, "output")
	cfg.Platforms = []Platform{
		config.PlatformDesktop, config.PlatformWeb,
	}
	cfg.ValidateSteps = false
	cfg.Speed = config.SpeedFast

	o := New(cfg)

	result, err := o.Run(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result.Report)
	assert.Len(t, result.Report.PlatformResults, 2)
}

// --- loadBanks tests ---

func TestLoadBanks_AlreadyLoaded(t *testing.T) {
	b := bank.New()
	o := &Orchestrator{
		bank:   b,
		config: config.DefaultConfig(),
	}
	err := o.loadBanks()
	assert.NoError(t, err)
	assert.Equal(t, b, o.bank)
}

func TestLoadBanks_FileNotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Banks = []string{"/nonexistent/file.json"}
	o := &Orchestrator{config: cfg}
	err := o.loadBanks()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stat bank")
}

// --- getDetector tests ---

func TestGetDetector_Android(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Device = "emulator-5554"
	cfg.PackageName = "com.test.app"
	o := &Orchestrator{config: cfg}

	det := o.getDetector(config.PlatformAndroid, "/tmp/ev")
	assert.NotNil(t, det)
	assert.Equal(t, config.PlatformAndroid, det.Platform())
}

func TestGetDetector_Web(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.BrowserURL = "http://localhost:3000"
	o := &Orchestrator{config: cfg}

	det := o.getDetector(config.PlatformWeb, "/tmp/ev")
	assert.NotNil(t, det)
	assert.Equal(t, config.PlatformWeb, det.Platform())
}

func TestGetDetector_Desktop(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DesktopProcess = "java"
	cfg.DesktopPID = 12345
	o := &Orchestrator{config: cfg}

	det := o.getDetector(config.PlatformDesktop, "/tmp/ev")
	assert.NotNil(t, det)
	assert.Equal(t, config.PlatformDesktop, det.Platform())
}

func TestGetDetector_UsesExisting(t *testing.T) {
	cfg := config.DefaultConfig()
	mock := newMockRunner()
	det := detector.New(
		config.PlatformDesktop,
		detector.WithCommandRunner(mock),
	)
	o := &Orchestrator{config: cfg, detector: det}

	got := o.getDetector(config.PlatformAndroid, "/tmp/ev")
	assert.Equal(t, det, got)
}

// --- getReporter tests ---

func TestGetReporter_Default(t *testing.T) {
	cfg := config.DefaultConfig()
	o := &Orchestrator{config: cfg}

	rep := o.getReporter()
	assert.NotNil(t, rep)
}

func TestGetReporter_UsesExisting(t *testing.T) {
	cfg := config.DefaultConfig()
	rep := reporter.New()
	o := &Orchestrator{config: cfg, reporter: rep}

	got := o.getReporter()
	assert.Equal(t, rep, got)
}

// --- Result tests ---

func TestResult_Fields(t *testing.T) {
	r := &Result{
		ReportPath: "/tmp/report.md",
		Success:    true,
		StartTime:  time.Now().Add(-10 * time.Second),
		EndTime:    time.Now(),
		Duration:   10 * time.Second,
	}
	assert.True(t, r.Success)
	assert.Equal(t, "/tmp/report.md", r.ReportPath)
	assert.Equal(t, 10*time.Second, r.Duration)
}

// --- JSON report format test ---

func TestRun_JSONReport(t *testing.T) {
	dir := t.TempDir()
	bankPath := createTestBank(t, dir)

	cfg := config.DefaultConfig()
	cfg.Banks = []string{bankPath}
	cfg.OutputDir = filepath.Join(dir, "output")
	cfg.Platforms = []Platform{config.PlatformDesktop}
	cfg.ValidateSteps = false
	cfg.Speed = config.SpeedFast
	cfg.ReportFormat = config.ReportJSON

	o := New(cfg)

	result, err := o.Run(context.Background())
	require.NoError(t, err)
	assert.Contains(t, result.ReportPath, ".json")
	assert.FileExists(t, result.ReportPath)
}

// --- HTML report format test ---

func TestRun_HTMLReport(t *testing.T) {
	dir := t.TempDir()
	bankPath := createTestBank(t, dir)

	cfg := config.DefaultConfig()
	cfg.Banks = []string{bankPath}
	cfg.OutputDir = filepath.Join(dir, "output")
	cfg.Platforms = []Platform{config.PlatformDesktop}
	cfg.ValidateSteps = false
	cfg.Speed = config.SpeedFast
	cfg.ReportFormat = config.ReportHTML

	o := New(cfg)

	result, err := o.Run(context.Background())
	require.NoError(t, err)
	assert.Contains(t, result.ReportPath, ".html")
	assert.FileExists(t, result.ReportPath)
}

type Platform = config.Platform
