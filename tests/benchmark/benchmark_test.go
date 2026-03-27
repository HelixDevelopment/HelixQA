// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package benchmark_test provides Go benchmark tests for the core HelixQA
// packages. Run with: go test ./tests/benchmark/ -bench=. -benchmem
package benchmark_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"digital.vasic.helixqa/pkg/analysis"
	"digital.vasic.helixqa/pkg/learning"
	"digital.vasic.helixqa/pkg/llm"
	"digital.vasic.helixqa/pkg/memory"
	"digital.vasic.helixqa/pkg/planning"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// mockBenchProvider is a minimal llm.Provider that returns a fixed response
// with zero latency, suitable for isolating parse/ranking benchmarks from
// I/O noise.
type mockBenchProvider struct {
	content string
}

func (m *mockBenchProvider) Name() string         { return "bench-mock" }
func (m *mockBenchProvider) SupportsVision() bool { return true }

func (m *mockBenchProvider) Chat(
	_ context.Context,
	_ []llm.Message,
) (*llm.Response, error) {
	return &llm.Response{Content: m.content, Model: "bench"}, nil
}

func (m *mockBenchProvider) Vision(
	_ context.Context,
	_ []byte,
	_ string,
) (*llm.Response, error) {
	return &llm.Response{Content: m.content, Model: "bench"}, nil
}

// newBenchStore opens a temporary SQLite-backed memory.Store. The caller is
// responsible for calling store.Close() via b.Cleanup.
func newBenchStore(b *testing.B) *memory.Store {
	b.Helper()
	s, err := memory.NewStore(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatalf("newBenchStore: %v", err)
	}
	b.Cleanup(func() { _ = s.Close() })
	return s
}

// seedBenchSession inserts a minimal session row for the given ID.
func seedBenchSession(b *testing.B, s *memory.Store, id string) {
	b.Helper()
	if err := s.CreateSession(memory.Session{
		ID:        id,
		StartedAt: time.Now().UTC(),
	}); err != nil {
		b.Fatalf("seedBenchSession %q: %v", id, err)
	}
}

// ── KnowledgeBase benchmarks ─────────────────────────────────────────────────

// BenchmarkKnowledgeBase_AddScreen measures the per-operation cost of
// AddScreen including the deduplication scan. After the first 1000 unique
// screens are added, the next 1000 calls are all duplicates, exercising
// the dedup path.
func BenchmarkKnowledgeBase_AddScreen(b *testing.B) {
	const unique = 1000
	kb := learning.NewKnowledgeBase()

	// Pre-populate with 1000 unique screens so the dedup scan is non-trivial.
	for i := 0; i < unique; i++ {
		kb.AddScreen(learning.Screen{
			Name:       fmt.Sprintf("Screen%04d", i),
			Platform:   "android",
			Route:      fmt.Sprintf("/screen/%d", i),
			Component:  fmt.Sprintf("ScreenComponent%04d", i),
			SourceFile: fmt.Sprintf("screen_%04d.kt", i),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Alternate between new screens and duplicate insertions to benchmark
		// both code paths within the same run.
		idx := i % (unique * 2)
		kb.AddScreen(learning.Screen{
			Name:     fmt.Sprintf("Screen%04d", idx),
			Platform: "android",
		})
	}
}

// BenchmarkKnowledgeBase_Summary measures the cost of generating the
// human-readable summary string from a knowledge base with 500 screens,
// 200 endpoints, 50 docs, and 20 components.
func BenchmarkKnowledgeBase_Summary(b *testing.B) {
	kb := learning.NewKnowledgeBase()
	kb.ProjectName = "BenchProject"

	for i := 0; i < 500; i++ {
		kb.AddScreen(learning.Screen{
			Name:     fmt.Sprintf("Screen%04d", i),
			Platform: "web",
		})
	}
	for i := 0; i < 200; i++ {
		kb.AddEndpoint(learning.APIEndpoint{
			Method: "GET",
			Path:   fmt.Sprintf("/api/v1/resource/%d", i),
		})
	}
	for i := 0; i < 50; i++ {
		kb.Docs = append(kb.Docs, learning.DocEntry{
			Path:  fmt.Sprintf("docs/doc%03d.md", i),
			Title: fmt.Sprintf("Document %d", i),
		})
	}
	for i := 0; i < 20; i++ {
		kb.Components = append(kb.Components, fmt.Sprintf("component-%02d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = kb.Summary()
	}
}

// ── MemoryStore benchmarks ────────────────────────────────────────────────────

// BenchmarkMemoryStore_CreateFinding measures the cost of a single SQLite
// finding INSERT, including timestamp generation and parameter binding.
// Each iteration uses a unique finding ID to avoid primary-key conflicts.
func BenchmarkMemoryStore_CreateFinding(b *testing.B) {
	s := newBenchStore(b)
	seedBenchSession(b, s, "bench-session-findings")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := memory.Finding{
			ID:        fmt.Sprintf("BENCH-%08d", i),
			SessionID: "bench-session-findings",
			Severity:  "medium",
			Category:  "bench",
			Title:     fmt.Sprintf("Benchmark finding %d", i),
			Status:    "open",
		}
		if err := s.CreateFinding(f); err != nil {
			b.Fatalf("CreateFinding iteration %d: %v", i, err)
		}
	}
}

// ── PriorityRanker benchmark ──────────────────────────────────────────────────

// BenchmarkPriorityRanker_Rank measures the cost of sorting a 1000-element
// PlannedTest slice through PriorityRanker.Rank. The input is re-used across
// iterations (Rank returns a copy, so the original is never mutated).
func BenchmarkPriorityRanker_Rank(b *testing.B) {
	const size = 1000

	tests := make([]planning.PlannedTest, size)
	for i := 0; i < size; i++ {
		tests[i] = planning.PlannedTest{
			ID:         fmt.Sprintf("TC-%04d", i),
			Name:       fmt.Sprintf("Test case %d", i),
			Priority:   (i % 5) + 1,
			IsExisting: i%3 == 0,
			IsNew:      i%3 != 0,
		}
	}

	// Mark every 10th test as having previously failed to exercise that path.
	priorFailures := make(map[string]bool)
	for i := 0; i < size; i += 10 {
		priorFailures[fmt.Sprintf("TC-%04d", i)] = true
	}

	ranker := planning.NewPriorityRanker(priorFailures)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ranker.Rank(tests)
	}
}

// ── BankReconciler benchmark ──────────────────────────────────────────────────

// BenchmarkBankReconciler_Reconcile measures the cost of reconciling a
// 1000-test generated plan against a 1000-entry bank (100% overlap). The
// bank entries are pre-loaded once in setup; only Reconcile is timed.
func BenchmarkBankReconciler_Reconcile(b *testing.B) {
	const size = 1000

	rec := planning.NewBankReconciler()
	for i := 0; i < size; i++ {
		rec.AddExisting(
			fmt.Sprintf("TC-%04d", i),
			fmt.Sprintf("Bench test case %d", i),
			"bench.yaml",
		)
	}

	tests := make([]planning.PlannedTest, size)
	for i := 0; i < size; i++ {
		tests[i] = planning.PlannedTest{
			ID:   fmt.Sprintf("GEN-%04d", i),
			Name: fmt.Sprintf("Bench test case %d", i),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rec.Reconcile(tests)
	}
}

// ── VisionAnalyzer benchmarks ─────────────────────────────────────────────────

// multiFindingJSON is a representative LLM response containing 5 findings,
// used to benchmark JSON parsing inside VisionAnalyzer.parseFindings.
const multiFindingJSON = `[
  {"category":"visual","severity":"high","title":"Button truncated","description":"CTA label is cut off.","evidence":"Shows Subm..."},
  {"category":"ux","severity":"medium","title":"No loading state","description":"Tap produces no feedback.","evidence":"Button unresponsive for 2s"},
  {"category":"accessibility","severity":"high","title":"Low contrast","description":"Text fails WCAG AA.","evidence":"Ratio 2.1:1"},
  {"category":"functional","severity":"critical","title":"Login fails","description":"Invalid credentials accepted.","evidence":"admin/admin succeeds"},
  {"category":"content","severity":"low","title":"Placeholder text","description":"Lorem ipsum visible.","evidence":"Profile bio shows Lorem ipsum"}
]`

// BenchmarkVisionAnalyzer_ParseFindings measures the cost of parsing a
// 5-finding JSON response returned by the mock LLM, including markdown-fence
// stripping, JSON unmarshalling, and field injection.
func BenchmarkVisionAnalyzer_ParseFindings(b *testing.B) {
	mock := &mockBenchProvider{content: multiFindingJSON}
	analyzer := analysis.NewVisionAnalyzer(mock)
	img := []byte("fake-screenshot-bytes")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		findings, err := analyzer.AnalyzeScreenshot(ctx, img, "home", "android")
		if err != nil {
			b.Fatalf("AnalyzeScreenshot: %v", err)
		}
		if len(findings) != 5 {
			b.Fatalf("expected 5 findings, got %d", len(findings))
		}
	}
}

// BenchmarkVisionAnalyzer_ParseFindings_JSONOnly isolates the raw JSON
// unmarshal cost by benchmarking json.Unmarshal directly on the same
// payload, giving a baseline to compare against the full analyzer path.
func BenchmarkVisionAnalyzer_ParseFindings_JSONOnly(b *testing.B) {
	payload := []byte(multiFindingJSON)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var findings []analysis.AnalysisFinding
		if err := json.Unmarshal(payload, &findings); err != nil {
			b.Fatalf("Unmarshal: %v", err)
		}
	}
}
