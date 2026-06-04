// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package testbank

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadFile_S1B_RejectsCaseUsingTitleInsteadOfName locks in the
// S1B regression: 7 atmosphere.yaml test cases used `title:` instead
// of the schema's required `name:` key. `title` is not a recognised
// TestCase field, so it deserialises to an empty Name, and IsValid()
// rejects the case with "missing name". A single such case aborted
// loading the entire banks/ directory.
//
// This is the RED guard: a bank whose only case uses `title:` MUST be
// rejected with a "missing name" error.
func TestLoadFile_S1B_RejectsCaseUsingTitleInsteadOfName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "title_typo.yaml")
	content := []byte(`name: title-typo-bank
test_cases:
  - id: CME-KA-AUDIO-MATRIX-001
    title: "this should have been a name: key"
    platforms: [api]
    steps:
      - name: step 1
        action: "a"
        expected: "ok"
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("expected 'missing name' error for case using title: instead of name:")
	}
	if !strings.Contains(err.Error(), "missing name") {
		t.Errorf("error must mention missing name: %v", err)
	}
}

// TestLoadFile_S1B_AcceptsCaseUsingName is the GREEN companion: the
// same case authored with the correct `name:` key loads cleanly.
func TestLoadFile_S1B_AcceptsCaseUsingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "named_bank.yaml")
	content := []byte(`name: named-bank
test_cases:
  - id: CME-KA-AUDIO-MATRIX-001
    name: "§KA Audio Quality Matrix"
    platforms: [api]
    steps:
      - name: step 1
        action: "a"
        expected: "ok"
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	bf, err := LoadFile(path)
	if err != nil {
		t.Fatalf("named case must load cleanly: %v", err)
	}
	if len(bf.TestCases) != 1 {
		t.Fatalf("loaded %d cases, want 1", len(bf.TestCases))
	}
	if bf.TestCases[0].Name == "" {
		t.Error("loaded case has empty Name")
	}
}

// TestLoadFile_S1B_RealAtmosphereBankLoads is the integration GREEN
// guard for the precise S1B defect: the real banks/atmosphere.yaml
// (with the title:->name: fix applied) MUST load without error.
// Before the fix, test case 167 (CME-KA-AUDIO-MATRIX-001) used
// `title:` instead of `name:`, deserialised to an empty Name, and
// aborted the load with "missing name". This resolves the file
// relative to the package (../../banks/atmosphere.yaml) and asserts
// the 7 previously-broken cases now carry a non-empty Name.
//
// NOTE: this targets the FILE, not LoadDir(banks/), because the
// directory carries a SEPARATE pre-existing cross-bank duplicate-id
// defect (CLI-CODEX-001 in cli-agents-comprehensive.yaml +
// cli-agents-test-helixagent.yaml) that is out of S1B scope — the
// missing-name error previously masked it. Tying this regression
// guard to the file keeps it independent of that unrelated blocker.
func TestLoadFile_S1B_RealAtmosphereBankLoads(t *testing.T) {
	path := filepath.Join("..", "..", "banks", "atmosphere.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("SKIP-OK: atmosphere.yaml not present at %s: %v", path, err)
	}
	bf, err := LoadFile(path)
	if err != nil {
		t.Fatalf("real banks/atmosphere.yaml must load cleanly: %v", err)
	}
	if len(bf.TestCases) == 0 {
		t.Fatal("loaded 0 cases from real atmosphere.yaml")
	}
	// The 7 IDs that previously used `title:` instead of `name:`.
	want := []string{
		"CME-KA-AUDIO-MATRIX-001",
		"CME-API-PROXY-001",
		"CME-GR-STREAM-CONTINUITY-001",
		"CME-HF-OVERLAY-NO-SECONDARY-001",
		"CME-GZ-RECENTS-001",
		"CME-AF-RECENTS-CLEAR-ALL-001",
		"CME-VIDEO-COLOR-FIDELITY-001",
	}
	byID := map[string]string{}
	for _, tc := range bf.TestCases {
		byID[tc.ID] = tc.Name
	}
	for _, id := range want {
		name, ok := byID[id]
		if !ok {
			t.Errorf("case %s not found in atmosphere.yaml", id)
			continue
		}
		if name == "" {
			t.Errorf("case %s has empty Name (title:->name: fix regressed)", id)
		}
	}
}
