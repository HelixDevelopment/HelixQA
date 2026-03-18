// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ticket

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/detector"
	"digital.vasic.helixqa/pkg/validator"
)

func TestGenerator_GenerateFromStep_Crash(t *testing.T) {
	gen := New()
	sr := &validator.StepResult{
		StepName: "Open settings",
		Status:   validator.StepFailed,
		Platform: config.PlatformAndroid,
		Detection: &detector.DetectionResult{
			HasCrash:       true,
			StackTrace:     "java.lang.NullPointerException\n\tat com.example.App.crash",
			LogEntries:     []string{"FATAL EXCEPTION: main", "Process: com.example, PID: 1234"},
			ScreenshotPath: "/evidence/crash-001.png",
		},
		PreScreenshot:  "/evidence/pre-001.png",
		PostScreenshot: "/evidence/post-001.png",
		StartTime:      time.Now().Add(-2 * time.Second),
		EndTime:        time.Now(),
		Duration:       2 * time.Second,
		Error:          "crash detected",
	}

	ticket := gen.GenerateFromStep(sr, "TC-001")

	assert.Equal(t, "HQA-0001", ticket.ID)
	assert.Contains(t, ticket.Title, "Crash")
	assert.Contains(t, ticket.Title, "Open settings")
	assert.Equal(t, SeverityCritical, ticket.Severity)
	assert.Equal(t, config.PlatformAndroid, ticket.Platform)
	assert.Equal(t, "TC-001", ticket.TestCaseID)
	assert.Equal(t, "Application crashed", ticket.ActualBehavior)
	assert.NotEmpty(t, ticket.StackTrace)
	assert.Len(t, ticket.Logs, 2)
	assert.Len(t, ticket.Screenshots, 3) // pre + post + detection
	assert.Contains(t, ticket.Labels, "crash")
	assert.Contains(t, ticket.Labels, "android")
}

func TestGenerator_GenerateFromStep_ANR(t *testing.T) {
	gen := New()
	sr := &validator.StepResult{
		StepName: "Save document",
		Status:   validator.StepFailed,
		Platform: config.PlatformAndroid,
		Detection: &detector.DetectionResult{
			HasANR:     true,
			LogEntries: []string{"ANR in com.example"},
		},
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Duration:  5 * time.Second,
		Error:     "ANR detected",
	}

	ticket := gen.GenerateFromStep(sr, "TC-002")

	assert.Equal(t, "HQA-0001", ticket.ID)
	assert.Contains(t, ticket.Title, "ANR")
	assert.Equal(t, SeverityHigh, ticket.Severity)
	assert.Contains(t, ticket.Labels, "anr")
}

func TestGenerator_GenerateFromStep_GenericFailure(t *testing.T) {
	gen := New()
	sr := &validator.StepResult{
		StepName:  "Export HTML",
		Status:    validator.StepFailed,
		Platform:  config.PlatformWeb,
		Detection: &detector.DetectionResult{},
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Duration:  1 * time.Second,
		Error:     "element not found",
	}

	ticket := gen.GenerateFromStep(sr, "TC-003")

	assert.Contains(t, ticket.Title, "failed")
	assert.Equal(t, SeverityMedium, ticket.Severity)
	assert.Equal(t, "element not found", ticket.ActualBehavior)
	assert.Contains(t, ticket.Labels, "failure")
}

func TestGenerator_GenerateFromDetection_Crash(t *testing.T) {
	gen := New()
	dr := &detector.DetectionResult{
		Platform:       config.PlatformDesktop,
		HasCrash:       true,
		StackTrace:     "Exception in thread main",
		LogEntries:     []string{"Fatal error"},
		ScreenshotPath: "/evidence/desktop-crash.png",
	}

	ticket := gen.GenerateFromDetection(
		dr, "Background process crashed",
	)

	assert.Equal(t, "HQA-0001", ticket.ID)
	assert.Equal(t, SeverityCritical, ticket.Severity)
	assert.Contains(t, ticket.Title, "Crash on desktop")
	assert.Len(t, ticket.Screenshots, 1)
}

func TestGenerator_GenerateFromDetection_ANR(t *testing.T) {
	gen := New()
	dr := &detector.DetectionResult{
		Platform:   config.PlatformAndroid,
		HasANR:     true,
		LogEntries: []string{"ANR timeout"},
	}

	ticket := gen.GenerateFromDetection(dr, "Main thread blocked")

	assert.Equal(t, SeverityHigh, ticket.Severity)
	assert.Contains(t, ticket.Title, "ANR")
}

func TestGenerator_Counter_Increments(t *testing.T) {
	gen := New()
	sr := &validator.StepResult{
		StepName:  "step",
		Platform:  config.PlatformWeb,
		Detection: &detector.DetectionResult{},
	}

	t1 := gen.GenerateFromStep(sr, "TC-A")
	t2 := gen.GenerateFromStep(sr, "TC-B")
	t3 := gen.GenerateFromStep(sr, "TC-C")

	assert.Equal(t, "HQA-0001", t1.ID)
	assert.Equal(t, "HQA-0002", t2.ID)
	assert.Equal(t, "HQA-0003", t3.ID)
}

func TestGenerator_RenderMarkdown(t *testing.T) {
	gen := New()
	ticket := &Ticket{
		ID:               "HQA-0001",
		Title:            "Crash during save on Android",
		Severity:         SeverityCritical,
		Platform:         config.PlatformAndroid,
		TestCaseID:       "TC-001",
		Description:      "App crashed when saving a large file.",
		StepsToReproduce: []string{"Open large file", "Tap save", "Observe crash"},
		ExpectedBehavior: "File saved successfully",
		ActualBehavior:   "Application crashed",
		StackTrace:       "java.lang.OutOfMemoryError\n\tat android.app.Activity",
		Logs:             []string{"E/AndroidRuntime: FATAL", "Process killed"},
		Screenshots:      []string{"/evidence/pre.png", "/evidence/crash.png"},
		CreatedAt:        time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
		Labels:           []string{"crash", "android"},
	}

	md := string(gen.RenderMarkdown(ticket))

	assert.Contains(t, md, "# Crash during save on Android")
	assert.Contains(t, md, "| **ID** | HQA-0001 |")
	assert.Contains(t, md, "| **Severity** | CRITICAL |")
	assert.Contains(t, md, "| **Platform** | android |")
	assert.Contains(t, md, "| **Test Case** | TC-001 |")
	assert.Contains(t, md, "| **Labels** | crash, android |")
	assert.Contains(t, md, "## Description")
	assert.Contains(t, md, "## Steps to Reproduce")
	assert.Contains(t, md, "1. Open large file")
	assert.Contains(t, md, "2. Tap save")
	assert.Contains(t, md, "## Expected vs Actual")
	assert.Contains(t, md, "**Expected:** File saved")
	assert.Contains(t, md, "**Actual:** Application crashed")
	assert.Contains(t, md, "## Stack Trace")
	assert.Contains(t, md, "OutOfMemoryError")
	assert.Contains(t, md, "## Logs")
	assert.Contains(t, md, "FATAL")
	assert.Contains(t, md, "## Evidence")
	assert.Contains(t, md, "pre.png")
	assert.Contains(t, md, "Generated by HelixQA")
}

func TestGenerator_RenderMarkdown_MinimalTicket(t *testing.T) {
	gen := New()
	ticket := &Ticket{
		ID:        "HQA-0002",
		Title:     "Minimal issue",
		Severity:  SeverityLow,
		Platform:  config.PlatformWeb,
		CreatedAt: time.Now(),
	}

	md := string(gen.RenderMarkdown(ticket))
	assert.Contains(t, md, "# Minimal issue")
	assert.Contains(t, md, "| **ID** | HQA-0002 |")
	// No steps, evidence, etc.
	assert.NotContains(t, md, "## Steps to Reproduce")
	assert.NotContains(t, md, "## Stack Trace")
	assert.NotContains(t, md, "## Logs")
	assert.NotContains(t, md, "## Evidence")
}

func TestGenerator_WriteTicket(t *testing.T) {
	dir := t.TempDir()
	gen := New(WithOutputDir(dir))

	ticket := &Ticket{
		ID:        "HQA-0001",
		Title:     "Test ticket",
		Severity:  SeverityMedium,
		Platform:  config.PlatformDesktop,
		CreatedAt: time.Now(),
	}

	path, err := gen.WriteTicket(ticket)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "HQA-0001.md"), path)

	// Verify file exists and content.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "# Test ticket")
}

func TestGenerator_WriteAll(t *testing.T) {
	dir := t.TempDir()
	gen := New(WithOutputDir(dir))

	tickets := []*Ticket{
		{
			ID: "HQA-0001", Title: "Issue 1",
			Severity: SeverityHigh, Platform: config.PlatformAndroid,
			CreatedAt: time.Now(),
		},
		{
			ID: "HQA-0002", Title: "Issue 2",
			Severity: SeverityLow, Platform: config.PlatformWeb,
			CreatedAt: time.Now(),
		},
	}

	paths, err := gen.WriteAll(tickets)
	require.NoError(t, err)
	assert.Len(t, paths, 2)

	for _, p := range paths {
		_, err := os.Stat(p)
		assert.NoError(t, err, "file should exist: %s", p)
	}
}

func TestGenerator_WriteTicket_NestedDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "tickets")
	gen := New(WithOutputDir(dir))

	ticket := &Ticket{
		ID: "HQA-0001", Title: "Nested",
		Severity: SeverityLow, Platform: config.PlatformWeb,
		CreatedAt: time.Now(),
	}

	path, err := gen.WriteTicket(ticket)
	require.NoError(t, err)
	assert.FileExists(t, path)
}

func TestSeverity_Values(t *testing.T) {
	// Verify severity string values.
	assert.Equal(t, "critical", string(SeverityCritical))
	assert.Equal(t, "high", string(SeverityHigh))
	assert.Equal(t, "medium", string(SeverityMedium))
	assert.Equal(t, "low", string(SeverityLow))
}

func TestGenerator_RenderMarkdown_EscapesSpecialChars(t *testing.T) {
	gen := New()
	ticket := &Ticket{
		ID:             "HQA-0001",
		Title:          "Error with | pipe and special chars",
		Severity:       SeverityMedium,
		Platform:       config.PlatformAndroid,
		ActualBehavior: "Failed with error: <html>",
		CreatedAt:      time.Now(),
	}

	md := string(gen.RenderMarkdown(ticket))
	// Should still produce valid Markdown.
	assert.True(t, strings.HasPrefix(md, "#"))
}
