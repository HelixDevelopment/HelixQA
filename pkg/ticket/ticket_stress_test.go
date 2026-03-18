// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ticket

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/detector"
	"digital.vasic.helixqa/pkg/validator"
)

func TestGenerator_Stress_ManyTickets(t *testing.T) {
	dir := t.TempDir()
	gen := New(WithOutputDir(dir))

	tickets := make([]*Ticket, 200)
	for i := 0; i < 200; i++ {
		sr := &validator.StepResult{
			StepName: fmt.Sprintf("step-%d", i),
			Status:   validator.StepFailed,
			Platform: config.PlatformAndroid,
			Detection: &detector.DetectionResult{
				HasCrash:   i%2 == 0,
				HasANR:     i%2 != 0,
				StackTrace: fmt.Sprintf("trace-%d", i),
				LogEntries: []string{fmt.Sprintf("log-%d", i)},
			},
			StartTime: time.Now(),
			EndTime:   time.Now(),
			Duration:  time.Second,
			Error:     fmt.Sprintf("error-%d", i),
		}
		tickets[i] = gen.GenerateFromStep(sr, fmt.Sprintf("TC-%d", i))
	}

	// Write all tickets.
	paths, err := gen.WriteAll(tickets)
	require.NoError(t, err)
	assert.Len(t, paths, 200)

	// Verify all files exist.
	for _, p := range paths {
		_, err := os.Stat(p)
		assert.NoError(t, err)
	}
}

func TestGenerator_Stress_ConcurrentRender(t *testing.T) {
	gen := New()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ticket := &Ticket{
				ID:             fmt.Sprintf("HQA-%04d", idx),
				Title:          fmt.Sprintf("Concurrent issue %d", idx),
				Severity:       SeverityHigh,
				Platform:       config.PlatformWeb,
				Description:    "Concurrently rendered ticket",
				StackTrace:     "concurrent trace",
				Logs:           []string{"log1", "log2"},
				Screenshots:    []string{"/shot1.png"},
				CreatedAt:      time.Now(),
				StepsToReproduce: []string{"step1", "step2"},
			}
			md := gen.RenderMarkdown(ticket)
			assert.NotEmpty(t, md)
		}(i)
	}
	wg.Wait()
}

func TestGenerator_Stress_LargeTicket(t *testing.T) {
	gen := New(WithOutputDir(t.TempDir()))

	// Create a ticket with lots of data.
	logs := make([]string, 1000)
	for i := range logs {
		logs[i] = fmt.Sprintf(
			"[%s] %s line %d: detailed log message with context",
			time.Now().Format(time.RFC3339),
			"E/AndroidRuntime", i,
		)
	}

	screenshots := make([]string, 50)
	for i := range screenshots {
		screenshots[i] = fmt.Sprintf("/evidence/screenshot-%04d.png", i)
	}

	ticket := &Ticket{
		ID:       "HQA-LARGE",
		Title:    "Large ticket with extensive evidence",
		Severity: SeverityCritical,
		Platform: config.PlatformAndroid,
		Description: "This is a very detailed description with " +
			"lots of context about the crash scenario.",
		StepsToReproduce: []string{
			"Open the application",
			"Navigate to settings",
			"Change theme to dark mode",
			"Go back to editor",
			"Type a long document",
			"Save the document",
			"Observe the crash",
		},
		ExpectedBehavior: "Document saved successfully",
		ActualBehavior:   "Application crashed with OOM",
		StackTrace: "java.lang.OutOfMemoryError: Failed to allocate\n" +
			"\tat android.graphics.BitmapFactory.nativeDecodeStream\n" +
			"\tat android.graphics.BitmapFactory.decodeStream\n" +
			"\tat com.example.ImageLoader.load\n" +
			"\tat com.example.Editor.onResume",
		Logs:        logs,
		Screenshots: screenshots,
		CreatedAt:   time.Now(),
		Labels:      []string{"crash", "oom", "android", "critical"},
	}

	path, err := gen.WriteTicket(ticket)
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	// Large ticket should produce a substantial file.
	assert.Greater(t, info.Size(), int64(10000))
}

func BenchmarkGenerator_RenderMarkdown(b *testing.B) {
	gen := New()
	ticket := &Ticket{
		ID:               "HQA-BENCH",
		Title:            "Benchmark ticket",
		Severity:         SeverityCritical,
		Platform:         config.PlatformAndroid,
		Description:      "Benchmark rendering",
		StepsToReproduce: []string{"step1", "step2", "step3"},
		StackTrace:       "java.lang.Exception\n\tat line1\n\tat line2",
		Logs:             []string{"log1", "log2", "log3"},
		Screenshots:      []string{"/s1.png", "/s2.png"},
		CreatedAt:        time.Now(),
		Labels:           []string{"crash", "android"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.RenderMarkdown(ticket)
	}
}

func BenchmarkGenerator_WriteTicket(b *testing.B) {
	dir := b.TempDir()
	gen := New(WithOutputDir(dir))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ticket := &Ticket{
			ID:        fmt.Sprintf("BENCH-%06d", i),
			Title:     "Bench",
			Severity:  SeverityLow,
			Platform:  config.PlatformWeb,
			CreatedAt: time.Now(),
		}
		path := filepath.Join(dir, fmt.Sprintf("BENCH-%06d.md", i))
		os.WriteFile(path, gen.RenderMarkdown(ticket), 0644)
	}
}
