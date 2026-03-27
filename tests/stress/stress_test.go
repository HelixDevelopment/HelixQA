// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package stress_test exercises HelixQA components under concurrent load.
// All tests use t.Parallel() and real SQLite databases to surface races
// and contention issues that unit tests with sequential execution miss.
package stress_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/learning"
	"digital.vasic.helixqa/pkg/llm"
	"digital.vasic.helixqa/pkg/memory"
	"digital.vasic.helixqa/pkg/planning"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// newStressStore opens a temporary SQLite-backed memory.Store and registers
// a cleanup function so the database is closed when the test finishes.
func newStressStore(t *testing.T) *memory.Store {
	t.Helper()
	s, err := memory.NewStore(filepath.Join(t.TempDir(), "stress_test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// mockStressProvider is a minimal llm.Provider that returns a canned
// response immediately, without any network calls.
type mockStressProvider struct {
	name    string
	vision  bool
	resp    *llm.Response
	callErr error
}

func (m *mockStressProvider) Name() string         { return m.name }
func (m *mockStressProvider) SupportsVision() bool { return m.vision }

func (m *mockStressProvider) Chat(
	_ context.Context,
	_ []llm.Message,
) (*llm.Response, error) {
	if m.callErr != nil {
		return nil, m.callErr
	}
	return m.resp, nil
}

func (m *mockStressProvider) Vision(
	_ context.Context,
	_ []byte,
	_ string,
) (*llm.Response, error) {
	if m.callErr != nil {
		return nil, m.callErr
	}
	return m.resp, nil
}

// ── Concurrent session creation ───────────────────────────────────────────────

// TestMemory_ConcurrentSessionCreation creates 100 sessions from separate
// goroutines. It verifies that all sessions are persisted without error and
// that each inserted session is individually retrievable.
func TestMemory_ConcurrentSessionCreation(t *testing.T) {
	t.Parallel()

	const workers = 100
	s := newStressStore(t)

	var wg sync.WaitGroup
	errs := make([]error, workers)

	for i := 0; i < workers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			sess := memory.Session{
				ID:        fmt.Sprintf("sess-concurrent-%04d", i),
				StartedAt: time.Now().UTC(),
				Platforms: "android",
			}
			errs[i] = s.CreateSession(sess)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		assert.NoError(t, err,
			"goroutine %d: CreateSession must not error", i)
	}

	// Verify all sessions were actually persisted.
	sessions, err := s.ListSessions(0)
	require.NoError(t, err)
	assert.Len(t, sessions, workers,
		"all %d sessions must be retrievable after concurrent inserts", workers)
}

// ── Concurrent finding creation ───────────────────────────────────────────────

// TestMemory_ConcurrentFindingCreation seeds a single session, then creates
// 100 findings from concurrent goroutines — each with a unique HELIX-NNN ID.
// All 100 must be persisted without error.
func TestMemory_ConcurrentFindingCreation(t *testing.T) {
	t.Parallel()

	const workers = 100
	s := newStressStore(t)

	// Insert the parent session once before concurrent work starts.
	require.NoError(t, s.CreateSession(memory.Session{
		ID:        "sess-findings-concurrent",
		StartedAt: time.Now().UTC(),
	}))

	var wg sync.WaitGroup
	errs := make([]error, workers)

	for i := 0; i < workers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			f := memory.Finding{
				ID:        fmt.Sprintf("HELIX-%04d", i+1),
				SessionID: "sess-findings-concurrent",
				Severity:  "medium",
				Category:  "stress",
				Title:     fmt.Sprintf("Concurrent finding %d", i),
				Status:    "open",
			}
			errs[i] = s.CreateFinding(f)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		assert.NoError(t, err,
			"goroutine %d: CreateFinding must not error", i)
	}

	// All findings must be queryable by status.
	open, err := s.ListFindingsByStatus("open")
	require.NoError(t, err)
	assert.Len(t, open, workers,
		"all %d findings must be retrievable after concurrent inserts", workers)
}

// ── Concurrent LLM adaptive provider calls ───────────────────────────────────

// TestLLM_AdaptiveProviderConcurrentCalls fires 50 concurrent Chat calls
// through an AdaptiveProvider backed by two mock providers. The test verifies
// that no races occur (use -race flag) and that all calls succeed.
func TestLLM_AdaptiveProviderConcurrentCalls(t *testing.T) {
	t.Parallel()

	const workers = 50

	primary := &mockStressProvider{
		name: "primary",
		resp: &llm.Response{Content: "ok", Model: "mock-1"},
	}
	fallback := &mockStressProvider{
		name: "fallback",
		resp: &llm.Response{Content: "ok-fallback", Model: "mock-2"},
	}

	ap := llm.NewAdaptiveProvider(primary, fallback)

	var wg sync.WaitGroup
	errs := make([]error, workers)
	resps := make([]*llm.Response, workers)

	msgs := []llm.Message{{Role: llm.RoleUser, Content: "ping"}}

	for i := 0; i < workers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := ap.Chat(context.Background(), msgs)
			errs[i] = err
			resps[i] = resp
		}()
	}
	wg.Wait()

	for i := 0; i < workers; i++ {
		assert.NoError(t, errs[i],
			"goroutine %d: Chat must not error", i)
		require.NotNil(t, resps[i],
			"goroutine %d: response must not be nil", i)
		assert.NotEmpty(t, resps[i].Content,
			"goroutine %d: response content must not be empty", i)
	}
}

// TestLLM_AdaptiveProviderConcurrentCalls_Fallback exercises the fallback
// path under concurrency: the primary provider always fails, so all 50
// goroutines must receive a response from the fallback without races.
func TestLLM_AdaptiveProviderConcurrentCalls_Fallback(t *testing.T) {
	t.Parallel()

	const workers = 50

	primary := &mockStressProvider{
		name:    "failing-primary",
		callErr: errors.New("primary unavailable"),
	}
	fallback := &mockStressProvider{
		name: "fallback",
		resp: &llm.Response{Content: "fallback-response", Model: "mock-2"},
	}

	ap := llm.NewAdaptiveProvider(primary, fallback)

	var wg sync.WaitGroup
	errs := make([]error, workers)

	msgs := []llm.Message{{Role: llm.RoleUser, Content: "ping"}}

	for i := 0; i < workers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := ap.Chat(context.Background(), msgs)
			errs[i] = err
		}()
	}
	wg.Wait()

	for i, err := range errs {
		assert.NoError(t, err,
			"goroutine %d: fallback path must not error", i)
	}
}

// ── Knowledge base large-project ingestion ────────────────────────────────────

// TestKnowledgeBase_LargeProjectIngestion creates a temporary project tree
// with 1000 stub Go source files and verifies that BuildKnowledgeBase
// completes without panicking or returning an error. This exercises the
// file-walking and screen-extraction paths at scale.
func TestKnowledgeBase_LargeProjectIngestion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// Create a realistic-looking project layout with many source files.
	srcDir := filepath.Join(root, "internal", "handlers")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	for i := 0; i < 1000; i++ {
		name := fmt.Sprintf("handler_%04d.go", i)
		content := fmt.Sprintf("package handlers\n\n// Handler%04d serves /api/v1/item-%04d\nfunc Handler%04d() {}\n",
			i, i, i)
		require.NoError(t, os.WriteFile(
			filepath.Join(srcDir, name), []byte(content), 0o644,
		))
	}

	// Docs directory so ReadDocs has something to walk.
	docsDir := filepath.Join(root, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(docsDir, "README.md"),
		[]byte("# Large Project\nStress test project.\n"),
		0o644,
	))

	// BuildKnowledgeBase must not crash or return an error.
	assert.NotPanics(t, func() {
		kb, err := learning.BuildKnowledgeBase(root, nil)
		assert.NoError(t, err)
		if kb != nil {
			assert.NotEmpty(t, kb.Summary())
		}
	})
}

// ── Plan reconciler large bank load ──────────────────────────────────────────

// TestPlanReconciler_LargeBankLoad creates a YAML bank file with 500 test
// cases, loads it via BankReconciler.LoadBankDir, and verifies that all
// entries are registered and that Reconcile produces correct results within
// a reasonable time budget.
func TestPlanReconciler_LargeBankLoad(t *testing.T) {
	t.Parallel()

	const bankSize = 500
	dir := t.TempDir()

	// Build a large YAML bank file programmatically.
	var sb strings.Builder
	sb.WriteString("version: \"1.0\"\nname: \"Stress Bank\"\ntest_cases:\n")
	for i := 0; i < bankSize; i++ {
		fmt.Fprintf(&sb,
			"  - id: TC-%04d\n    name: \"Stress test case %d\"\n    category: stress\n    priority: 1\n    platforms: [android, web]\n",
			i+1, i,
		)
	}

	bankPath := filepath.Join(dir, "stress_bank.yaml")
	require.NoError(t, os.WriteFile(bankPath, []byte(sb.String()), 0o644))

	rec := planning.NewBankReconciler()
	start := time.Now()
	require.NoError(t, rec.LoadBankDir(dir))
	loadDuration := time.Since(start)

	assert.Equal(t, bankSize, rec.ExistingCount(),
		"all %d bank entries must be registered", bankSize)

	// Loading 500 entries must complete well within 2 seconds.
	assert.Less(t, loadDuration, 2*time.Second,
		"LoadBankDir with %d entries should be fast (got %v)", bankSize, loadDuration)

	// Reconcile against a generated plan that fully overlaps the bank.
	generated := make([]planning.PlannedTest, bankSize/2)
	for i := range generated {
		generated[i] = planning.PlannedTest{
			ID:   fmt.Sprintf("GEN-%04d", i),
			Name: fmt.Sprintf("Stress test case %d", i),
		}
	}

	reconcileStart := time.Now()
	reconciled := rec.Reconcile(generated)
	reconcileDuration := time.Since(reconcileStart)

	assert.Len(t, reconciled, bankSize/2)
	assert.Less(t, reconcileDuration, 2*time.Second,
		"Reconcile with %d tests should be fast (got %v)", bankSize/2, reconcileDuration)

	// Every generated test matches a bank entry by name — all must be existing.
	existingCount := 0
	for _, pt := range reconciled {
		if pt.IsExisting {
			existingCount++
		}
	}
	assert.Equal(t, bankSize/2, existingCount,
		"all generated tests should match bank entries")
}
