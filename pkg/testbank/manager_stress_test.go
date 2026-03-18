// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package testbank

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/config"
)

func TestManager_Stress_LargeBank(t *testing.T) {
	dir := t.TempDir()

	// Generate a bank with 500 test cases.
	var cases string
	for i := 0; i < 500; i++ {
		cases += fmt.Sprintf(`  - id: STRESS-%04d
    name: "Stress test %d"
    category: stress
    priority: medium
    tags: [stress, load]
`, i, i)
	}
	content := "version: \"1.0\"\nname: Stress Bank\ntest_cases:\n" + cases
	path := filepath.Join(dir, "stress.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	mgr := NewManager()
	require.NoError(t, mgr.LoadFile(path))
	assert.Equal(t, 500, mgr.Count())

	// Verify filtering.
	all := mgr.All()
	assert.Len(t, all, 500)

	byTag := mgr.ByTag("stress")
	assert.Len(t, byTag, 500)

	defs := mgr.ToDefinitions(config.PlatformAndroid)
	assert.Len(t, defs, 500)
}

func TestManager_Stress_MultipleBankFiles(t *testing.T) {
	dir := t.TempDir()

	// Create 20 bank files with 25 test cases each.
	for f := 0; f < 20; f++ {
		var cases string
		for i := 0; i < 25; i++ {
			id := fmt.Sprintf("MB-%02d-%03d", f, i)
			cases += fmt.Sprintf(`  - id: %s
    name: "Multi-bank test %s"
    category: multi
    priority: high
`, id, id)
		}
		content := fmt.Sprintf(
			"version: \"1.0\"\nname: Bank %d\ntest_cases:\n%s",
			f, cases,
		)
		path := filepath.Join(dir, fmt.Sprintf("bank-%02d.yaml", f))
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}

	mgr := NewManager()
	require.NoError(t, mgr.LoadDir(dir))
	assert.Equal(t, 500, mgr.Count())
}

func TestManager_Stress_ConcurrentReads(t *testing.T) {
	dir := t.TempDir()
	writeSampleBank(t, dir, "core.yaml")

	mgr := NewManager()
	require.NoError(t, mgr.LoadFile(
		filepath.Join(dir, "core.yaml"),
	))

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mgr.All()
			_ = mgr.Count()
			_ = mgr.ForPlatform(config.PlatformAndroid)
			_ = mgr.ByCategory("functional")
			_ = mgr.ByTag("core")
			_, _ = mgr.Get("TC-001")
		}()
	}
	wg.Wait()
}

func TestManager_Stress_SaveAndReload(t *testing.T) {
	dir := t.TempDir()

	// Create a bank, save it, reload it — 50 iterations.
	for i := 0; i < 50; i++ {
		bf := &BankFile{
			Version: "1.0",
			Name:    fmt.Sprintf("Iteration %d", i),
			TestCases: []TestCase{
				{
					ID:       fmt.Sprintf("ITER-%04d", i),
					Name:     fmt.Sprintf("Test %d", i),
					Category: "iteration",
					Priority: PriorityMedium,
				},
			},
		}
		path := filepath.Join(dir, fmt.Sprintf("iter-%d.yaml", i))
		require.NoError(t, SaveFile(path, bf))

		loaded, err := LoadFile(path)
		require.NoError(t, err)
		assert.Equal(t, bf.Name, loaded.Name)
	}
}

func BenchmarkManager_LoadFile(b *testing.B) {
	dir := b.TempDir()
	var cases string
	for i := 0; i < 100; i++ {
		cases += fmt.Sprintf(`  - id: BM-%04d
    name: "Benchmark test %d"
    category: bench
    priority: low
`, i, i)
	}
	content := "version: \"1.0\"\nname: Bench\ntest_cases:\n" + cases
	path := filepath.Join(dir, "bench.yaml")
	os.WriteFile(path, []byte(content), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = LoadFile(path)
	}
}

func BenchmarkManager_ForPlatform(b *testing.B) {
	dir := b.TempDir()
	var cases string
	for i := 0; i < 200; i++ {
		platform := ""
		if i%3 == 0 {
			platform = "\n    platforms: [android]"
		} else if i%3 == 1 {
			platform = "\n    platforms: [web]"
		}
		cases += fmt.Sprintf(`  - id: FP-%04d
    name: "Platform test %d"
    category: bench
    priority: medium%s
`, i, i, platform)
	}
	content := "version: \"1.0\"\nname: Platform Bench\ntest_cases:\n" + cases
	path := filepath.Join(dir, "platform.yaml")
	os.WriteFile(path, []byte(content), 0644)

	mgr := NewManager()
	mgr.LoadFile(path)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.ForPlatform(config.PlatformAndroid)
	}
}

func BenchmarkManager_ToDefinitions(b *testing.B) {
	dir := b.TempDir()
	var cases string
	for i := 0; i < 200; i++ {
		cases += fmt.Sprintf(`  - id: TD-%04d
    name: "Def test %d"
    category: bench
    priority: high
    dependencies: [TD-0001, TD-0002]
`, i, i)
	}
	content := "version: \"1.0\"\nname: Def Bench\ntest_cases:\n" + cases
	path := filepath.Join(dir, "defs.yaml")
	os.WriteFile(path, []byte(content), 0644)

	mgr := NewManager()
	mgr.LoadFile(path)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.ToDefinitions(config.PlatformAll)
	}
}
