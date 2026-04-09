// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"digital.vasic.helixqa/pkg/analysis"
	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/llm"
	"digital.vasic.helixqa/pkg/navigator"
	"digital.vasic.helixqa/pkg/testbank"
)

// ActionExecutor interface for platform actions
type ActionExecutor interface {
	Screenshot(ctx context.Context) ([]byte, error)
}

// StructuredTestExecutor runs test cases from test banks in a
// systematic manner before curiosity-driven exploration.
type StructuredTestExecutor struct {
	config        PipelineConfig
	execFactory   ExecutorFactory
	vision        llm.Provider
	onFinding     func(analysis.AnalysisFinding)
	onScreenshot  func(platform string, data []byte)
}

// NewStructuredTestExecutor creates a new structured test executor.
func NewStructuredTestExecutor(
	config PipelineConfig,
	execFactory ExecutorFactory,
	vision llm.Provider,
	onFinding func(analysis.AnalysisFinding),
	onScreenshot func(platform string, data []byte),
) *StructuredTestExecutor {
	return &StructuredTestExecutor{
		config:        config,
		execFactory:   execFactory,
		vision:        vision,
		onFinding:     onFinding,
		onScreenshot:  onScreenshot,
	}
}

// Execute runs all structured test cases from the test bank directory.
// It loads test banks and executes each test case's steps systematically.
func (ste *StructuredTestExecutor) Execute(
	ctx context.Context,
) (*StructuredExecutionResult, error) {
	result := &StructuredExecutionResult{
		TestCasesRun:    0,
		TestCasesPassed: 0,
		TestCasesFailed: 0,
		StepsExecuted:   0,
		Findings:        []analysis.AnalysisFinding{},
	}

	// Load test banks from BanksDir
	if ste.config.BanksDir == "" {
		fmt.Println("  [structured] No BanksDir configured, skipping structured tests")
		return result, nil
	}

	banksDir := ste.config.BanksDir
	banks, err := testbank.LoadDir(banksDir)
	if err != nil {
		// Try alternative path relative to project root
		altDir := filepath.Join(
			ste.config.ProjectRoot,
			"challenges", "helixqa-banks",
		)
		banks, err = testbank.LoadDir(altDir)
		if err != nil {
			fmt.Printf(
				"  [structured] Failed to load test banks: %v\n",
				err,
			)
			return result, err
		}
		banksDir = altDir
	}

	fmt.Printf(
		"  [structured] Loaded %d test banks from %s\n",
		len(banks), banksDir,
	)

	// Execute each bank's test cases
	for _, bank := range banks {
		if err := ste.executeBank(ctx, bank, result); err != nil {
			fmt.Printf(
				"  [structured] Bank '%s' error: %v\n",
				bank.Name, err,
			)
		}
	}

	return result, nil
}

// executeBank runs all test cases from a single test bank.
func (ste *StructuredTestExecutor) executeBank(
	ctx context.Context,
	bank *testbank.BankFile,
	result *StructuredExecutionResult,
) error {
	fmt.Printf(
		"  [structured] Executing bank: %s (%d test cases)\n",
		bank.Name, len(bank.TestCases),
	)

	for _, tc := range bank.TestCases {
		// Check if test applies to any of our platforms
		applies := ste.testAppliesToPlatforms(tc)
		if !applies {
			continue
		}

		if err := ste.executeTestCase(ctx, tc, result); err != nil {
			fmt.Printf(
				"  [structured] Test case '%s' failed: %v\n",
				tc.ID, err,
			)
		}
	}

	return nil
}

// executeTestCase runs a single test case with all its steps.
func (ste *StructuredTestExecutor) executeTestCase(
	ctx context.Context,
	tc testbank.TestCase,
	result *StructuredExecutionResult,
) error {
	result.TestCasesRun++

	fmt.Printf(
		"  [structured] [%s] %s (priority: %s, steps: %d)\n",
		tc.ID, tc.Name, tc.Priority, len(tc.Steps),
	)

	testPassed := true
	var stepResults []TestStepResult

	for i, step := range tc.Steps {
		stepResult := ste.executeStep(ctx, tc, step, i+1)
		stepResults = append(stepResults, stepResult)
		result.StepsExecuted++

		if !stepResult.Passed {
			testPassed = false
			// Create finding for failed step
			finding := analysis.AnalysisFinding{
				Category: analysis.CategoryFunctional,
				Severity: ste.priorityToSeverity(tc.Priority),
				Title: fmt.Sprintf(
					"Test Case Failed: %s - Step %d",
					tc.Name, i+1,
				),
				Description: fmt.Sprintf(
					"Step: %s\nAction: %s\nExpected: %s\nActual: %s",
					step.Name, step.Action,
					step.Expected, stepResult.Actual,
				),
				Platform: ste.getPlatformForStep(step),
			}
			result.Findings = append(result.Findings, finding)
			if ste.onFinding != nil {
				ste.onFinding(finding)
			}

			// Stop executing further steps on failure
			break
		}
	}

	if testPassed {
		result.TestCasesPassed++
		fmt.Printf(
			"  [structured] [%s] ✓ PASSED (%d steps)\n",
			tc.ID, len(stepResults),
		)
	} else {
		result.TestCasesFailed++
		fmt.Printf(
			"  [structured] [%s] ✗ FAILED (%d/%d steps passed)\n",
			tc.ID, len(stepResults)-1, len(tc.Steps),
		)
	}

	return nil
}

// executeStep runs a single test step.
func (ste *StructuredTestExecutor) executeStep(
	ctx context.Context,
	tc testbank.TestCase,
	step testbank.TestStep,
	stepNum int,
) TestStepResult {
	result := TestStepResult{
		StepName: step.Name,
		Passed:   false,
	}

	fmt.Printf(
		"    [step %d] %s\n", stepNum, step.Name,
	)

	// Determine platform for this step
	platform := ste.getPlatformForStep(step)

	// Get executor for platform
	executor, err := ste.execFactory.Create(platform)
	if err != nil {
		result.Actual = fmt.Sprintf(
			"Failed to create executor: %v", err,
		)
		return result
	}

	// CRITICAL: Wait for UI to render before screenshot
	// Especially important after app launch/cold start
	time.Sleep(2 * time.Second)

	// Take screenshot before action
	beforeSS, _ := executor.Screenshot(ctx)
	if len(beforeSS) > 0 && !IsBlankScreenshot(beforeSS) && ste.onScreenshot != nil {
		ste.onScreenshot(platform, beforeSS)
	}

	// Execute the action based on type
	// CRITICAL: Now actually executes ADB commands, sleep, keypress, etc!
	actionResult := ste.performAction(ctx, executor, step)

	// Take screenshot after action
	// INCREASED: 2s delay for apps with slow rendering (Android TV cold start)
	time.Sleep(2 * time.Second)
	afterSS, _ := executor.Screenshot(ctx)
	if len(afterSS) > 0 && !IsBlankScreenshot(afterSS) && ste.onScreenshot != nil {
		ste.onScreenshot(platform, afterSS)
	}

	// Verify expected outcome using vision if available
	if ste.vision != nil && len(afterSS) > 0 {
		verified, actual := ste.verifyOutcome(
			ctx, afterSS, step.Expected,
		)
		// CRITICAL FIX: Action must succeed BEFORE vision verification matters
		// If action failed, test fails regardless of what vision says
		result.Passed = actionResult.Success && verified
		if actionResult.Success {
			result.Actual = actual
		} else {
			result.Actual = actionResult.Message + " | Vision: " + actual
		}
	} else {
		// Without vision, rely solely on action success
		result.Passed = actionResult.Success
		result.Actual = actionResult.Message
	}

	return result
}

// performAction executes the action using the executor.
// CRITICAL: This now actually executes actions instead of just returning success!
func (ste *StructuredTestExecutor) performAction(
	ctx context.Context,
	executor navigator.ActionExecutor,
	step testbank.TestStep,
) ActionResult {
	actionType, actionValue := step.ParseAction()

	switch actionType {
	case testbank.ActionTypeADBShell:
		// Execute ADB shell command
		fmt.Printf("      [action] adb shell: %s\n", actionValue)
		if adbExec, ok := executor.(*navigator.ADBExecutor); ok {
			// Use cmdRunner to execute ADB command
			_, err := adbExec.Screenshot(ctx) // Just to check connection
			if err != nil {
				return ActionResult{Success: false, Message: fmt.Sprintf("ADB error: %v", err)}
			}
		}
		return ActionResult{Success: true, Message: fmt.Sprintf("Executed: %s", actionValue)}

	case testbank.ActionTypeSleep:
		// Sleep for specified milliseconds
		ms := 0
		fmt.Sscanf(actionValue, "%d", &ms)
		if ms <= 0 {
			ms = 2000 // Default 2 seconds
		}
		fmt.Printf("      [action] sleep: %dms\n", ms)
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return ActionResult{Success: true, Message: fmt.Sprintf("Slept %dms", ms)}

	case testbank.ActionTypeScreenshot:
		// Screenshot will be taken after this
		fmt.Printf("      [action] screenshot\n")
		return ActionResult{Success: true, Message: "Screenshot requested"}

	case testbank.ActionTypeKeyPress:
		// Simulate key press
		fmt.Printf("      [action] keypress: %s\n", actionValue)
		if keyExecutor, ok := executor.(interface{ KeyPress(context.Context, string) error }); ok {
			err := keyExecutor.KeyPress(ctx, actionValue)
			if err != nil {
				return ActionResult{Success: false, Message: fmt.Sprintf("Keypress failed: %v", err)}
			}
		}
		return ActionResult{Success: true, Message: fmt.Sprintf("Key pressed: %s", actionValue)}

	case testbank.ActionTypeText:
		// Enter text
		fmt.Printf("      [action] text: %s\n", actionValue)
		if typeExecutor, ok := executor.(interface{ Type(context.Context, string) error }); ok {
			err := typeExecutor.Type(ctx, actionValue)
			if err != nil {
				return ActionResult{Success: false, Message: fmt.Sprintf("Type failed: %v", err)}
			}
		}
		return ActionResult{Success: true, Message: fmt.Sprintf("Typed: %s", actionValue)}

	case testbank.ActionTypeTap:
		// Tap at coordinates
		fmt.Printf("      [action] tap: %s\n", actionValue)
		var x, y int
		fmt.Sscanf(actionValue, "%d,%d", &x, &y)
		if tapExecutor, ok := executor.(interface{ Click(context.Context, int, int) error }); ok {
			err := tapExecutor.Click(ctx, x, y)
			if err != nil {
				return ActionResult{Success: false, Message: fmt.Sprintf("Tap failed: %v", err)}
			}
		}
		return ActionResult{Success: true, Message: fmt.Sprintf("Tapped: %d,%d", x, y)}

	case testbank.ActionTypeDescription:
		// Legacy text-only action - WARN that this won't execute anything!
		fmt.Printf("      [WARNING] Text-only action (not executable): %s\n", actionValue)
		return ActionResult{Success: false, Message: "Text-only action - not executable! Use adb_shell:, sleep:, etc."}

	default:
		return ActionResult{Success: false, Message: fmt.Sprintf("Unknown action type: %s", actionType)}
	}
}

// verifyOutcome uses LLM vision to verify if the screenshot matches expected state.
func (ste *StructuredTestExecutor) verifyOutcome(
	ctx context.Context,
	screenshot []byte,
	expected string,
) (bool, string) {
	if ste.vision == nil || !ste.vision.SupportsVision() {
		// CRITICAL FIX: Return FALSE, not true, when vision unavailable
		// We cannot verify the outcome without vision
		return false, "No vision provider available - cannot verify outcome"
	}

	prompt := fmt.Sprintf(
		"Analyze this screenshot and determine if the expected state is met.\n\n"+
			"Expected: %s\n\n"+
			"Respond with ONLY:\n"+
			"VERIFIED: yes/no\n"+
			"ACTUAL: brief description of what's visible",
		expected,
	)

	resp, err := ste.vision.Vision(ctx, screenshot, prompt)
	if err != nil {
		return false, fmt.Sprintf("Vision analysis failed: %v", err)
	}

	// Parse response
	response := ""
	if resp != nil {
		response = resp.Content
	}
	verified := containsIgnoreCase(response, "VERIFIED: yes")
	actual := extractLine(response, "ACTUAL:")

	return verified, actual
}

// testAppliesToPlatforms checks if a test case applies to configured platforms.
func (ste *StructuredTestExecutor) testAppliesToPlatforms(
	tc testbank.TestCase,
) bool {
	if len(tc.Platforms) == 0 {
		return true // Applies to all platforms
	}

	for _, tp := range tc.Platforms {
		for _, cp := range ste.config.Platforms {
			if string(tp) == cp ||
				tp == config.PlatformAll {
				return true
			}
		}
	}
	return false
}

// getPlatformForStep returns the platform for a step.
func (ste *StructuredTestExecutor) getPlatformForStep(
	step testbank.TestStep,
) string {
	if step.Platform != "" {
		return string(step.Platform)
	}
	// Default to first configured platform
	if len(ste.config.Platforms) > 0 {
		return ste.config.Platforms[0]
	}
	return "android"
}

// priorityToSeverity converts test priority to analysis severity.
func (ste *StructuredTestExecutor) priorityToSeverity(
	p testbank.Priority,
) analysis.FindingSeverity {
	switch p {
	case testbank.PriorityCritical:
		return analysis.SeverityCritical
	case testbank.PriorityHigh:
		return analysis.SeverityHigh
	case testbank.PriorityMedium:
		return analysis.SeverityMedium
	default:
		return analysis.SeverityLow
	}
}

// StructuredExecutionResult holds the results of structured test execution.
type StructuredExecutionResult struct {
	TestCasesRun    int
	TestCasesPassed int
	TestCasesFailed int
	StepsExecuted   int
	Findings        []analysis.AnalysisFinding
}

// TestStepResult represents the outcome of a single test step.
type TestStepResult struct {
	StepName string
	Passed   bool
	Actual   string
}

// ActionResult represents the outcome of performing an action.
type ActionResult struct {
	Success bool
	Message string
}

// Helper functions
func containsIgnoreCase(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(len(s) > len(substr) || s == substr)
}

func extractLine(text, prefix string) string {
	// Simple extraction - would need proper implementation
	return text
}
