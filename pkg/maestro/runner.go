// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package maestro provides a FlowRunner that executes Maestro YAML
// mobile flow files via the maestro CLI subprocess and parses the
// results into structured FlowResult values.
package maestro

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// flowResultPattern matches the summary line emitted by maestro, e.g.
// "1 Passed, 0 Failed" or "0 Passed, 2 Failed".
var flowResultPattern = regexp.MustCompile(
	`(\d+)\s+Passed,\s+(\d+)\s+Failed`,
)

// FlowResult captures the outcome of a single Maestro flow execution.
type FlowResult struct {
	// FlowFile is the path to the YAML flow file that was executed.
	FlowFile string `json:"flow_file"`

	// Output is the combined stdout/stderr of the maestro process.
	Output string `json:"output"`

	// Error holds any error message from the execution.
	Error string `json:"error,omitempty"`

	// Success is true when all steps passed and no failure marker
	// was detected.
	Success bool `json:"success"`

	// Passed is the number of steps that passed.
	Passed int `json:"passed"`

	// Failed is the number of steps that failed.
	Failed int `json:"failed"`
}

// FlowRunner executes Maestro YAML flow files via the maestro CLI.
type FlowRunner struct {
	maestroPath string
}

// NewFlowRunner returns a FlowRunner that uses "maestro" (resolved
// via PATH) as the CLI executable.
func NewFlowRunner() *FlowRunner {
	return &FlowRunner{maestroPath: "maestro"}
}

// NewFlowRunnerWithPath returns a FlowRunner that uses the given
// path as the maestro CLI executable.
func NewFlowRunnerWithPath(path string) *FlowRunner {
	return &FlowRunner{maestroPath: path}
}

// RunFlow executes the given Maestro flow file, optionally targeting
// a specific device. It always returns a non-nil *FlowResult even
// when the subprocess fails; the caller should inspect result.Success
// and result.Error rather than only checking the returned error.
func (r *FlowRunner) RunFlow(
	ctx context.Context,
	flowFile string,
	device string,
) (*FlowResult, error) {
	args := r.buildArgs(flowFile, device)
	cmd := exec.CommandContext(ctx, r.maestroPath, args...)
	raw, err := cmd.CombinedOutput()
	output := string(raw)

	result, parseErr := r.parseFlowResult(output)
	if result == nil {
		result = &FlowResult{}
	}
	result.FlowFile = flowFile
	result.Output = output

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		if parseErr != nil {
			return result, fmt.Errorf(
				"maestro: run failed and parse failed: %w",
				parseErr,
			)
		}
		return result, nil
	}

	if parseErr != nil {
		result.Success = false
		result.Error = parseErr.Error()
		return result, fmt.Errorf(
			"maestro: parse output failed: %w", parseErr,
		)
	}

	return result, nil
}

// buildArgs constructs the argument slice for the maestro CLI.
// It always includes "test" and flowFile; --device is added only
// when device is non-empty.
func (r *FlowRunner) buildArgs(flowFile, device string) []string {
	args := []string{"test", flowFile}
	if device != "" {
		args = append(args, "--device", device)
	}
	return args
}

// parseFlowResult extracts pass/fail counts from maestro output and
// determines overall success. Success requires that the failed count
// is zero and that the output contains no "❌" failure marker.
func (r *FlowRunner) parseFlowResult(
	output string,
) (*FlowResult, error) {
	matches := flowResultPattern.FindStringSubmatch(output)
	if matches == nil {
		return nil, fmt.Errorf(
			"maestro: could not parse result from output",
		)
	}

	passed, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, fmt.Errorf(
			"maestro: invalid passed count %q: %w",
			matches[1], err,
		)
	}

	failed, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, fmt.Errorf(
			"maestro: invalid failed count %q: %w",
			matches[2], err,
		)
	}

	success := failed == 0 && !strings.Contains(output, "❌")

	return &FlowResult{
		Passed:  passed,
		Failed:  failed,
		Success: success,
	}, nil
}
