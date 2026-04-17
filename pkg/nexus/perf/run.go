package perf

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// K6Runner shells out to an installed k6 binary to execute the script
// produced by GenerateScript. Keeping k6 out-of-process avoids the
// AGPL contamination risk of embedding k6's JS runtime; operators who
// want the embedded path bring the k6 package themselves.
type K6Runner struct {
	binary string
	dir    string
}

// NewK6Runner returns a runner that invokes binary (default "k6")
// under workdir. Output artefacts land under workdir/k6-out/.
func NewK6Runner(binary, workdir string) (*K6Runner, error) {
	if binary == "" {
		binary = "k6"
	}
	if workdir == "" {
		workdir = "."
	}
	return &K6Runner{binary: binary, dir: workdir}, nil
}

// RunScenario writes scenario's generated script to a temporary file,
// invokes k6 with JSON output, and returns a parsed Metrics envelope.
// The caller is responsible for asserting thresholds via Metrics.Assert.
func (r *K6Runner) RunScenario(ctx context.Context, scenario Scenario) (*Metrics, error) {
	if r.binary == "" {
		return nil, errors.New("k6 runner: binary is empty")
	}
	if _, err := exec.LookPath(r.binary); err != nil {
		return nil, fmt.Errorf("k6 runner: %s not found in PATH: %w", r.binary, err)
	}
	script, err := GenerateScript(scenario)
	if err != nil {
		return nil, err
	}
	outDir := filepath.Join(r.dir, "k6-out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("k6 runner: mkdir: %w", err)
	}
	scriptPath := filepath.Join(outDir, "scenario.js")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		return nil, fmt.Errorf("k6 runner: write script: %w", err)
	}
	resultsPath := filepath.Join(outDir, "results.json")
	cmd := exec.CommandContext(ctx, r.binary, "run", "--out", "json="+resultsPath, scriptPath)
	cmd.Env = append(os.Environ(), "K6_BROWSER_ENABLED=true", "K6_BROWSER_HEADLESS=true")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("k6 runner: exec failed: %w\n--- k6 output ---\n%s", err, string(out))
	}
	raw, err := os.ReadFile(resultsPath)
	if err != nil {
		return nil, fmt.Errorf("k6 runner: read results: %w", err)
	}
	return ParseK6JSON(raw)
}

// Available reports whether the configured k6 binary is on PATH.
// Returns a typed error describing where to install k6 when missing.
func (r *K6Runner) Available() error {
	if _, err := exec.LookPath(r.binary); err != nil {
		return fmt.Errorf("k6 runner: %s not installed. See https://grafana.com/docs/k6/latest/set-up/install-k6/", r.binary)
	}
	return nil
}
