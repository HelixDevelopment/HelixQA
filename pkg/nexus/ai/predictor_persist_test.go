// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ai

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPredictor_P3_SaveAndLoadRoundTrip locks in P3 from
// docs/nexus/remaining-work.md: weights + bias survive a
// save-load round-trip exactly.
func TestPredictor_P3_SaveAndLoadRoundTrip(t *testing.T) {
	p := NewPredictor()
	// Mutate the predictor so we can verify persistence carries the
	// new state forward.
	for i := 0; i < 50; i++ {
		p.Observe(FlakeSample{Retries: 8, Pass: false, HourOfDay: 3})
	}
	probBefore := p.Probability(FlakeSample{Retries: 5, HourOfDay: 3})
	biasBefore := p.bias

	path := filepath.Join(t.TempDir(), "predictor.json")
	if err := p.SaveWeights(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	fresh := NewPredictor()
	if err := fresh.LoadWeights(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	if fresh.bias != biasBefore {
		t.Errorf("bias mismatch: got %f, want %f", fresh.bias, biasBefore)
	}
	probAfter := fresh.Probability(FlakeSample{Retries: 5, HourOfDay: 3})
	if probAfter != probBefore {
		t.Errorf("probability mismatch: got %f, want %f", probAfter, probBefore)
	}
}

func TestPredictor_P3_LoadAtomicity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "predictor.json")

	p := NewPredictor()
	if err := p.SaveWeights(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Atomic-write guarantee: no .tmp sibling remains after a clean
	// save.
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf(".tmp must be cleaned up after atomic rename")
	}
}

func TestPredictor_P3_SchemaVersionMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "predictor.json")
	// Seed a blob with a bogus version number.
	if err := os.WriteFile(path, []byte(`{"version":999,"weight":{"x":1},"bias":0}`), 0o644); err != nil {
		t.Fatal(err)
	}
	p := NewPredictor()
	err := p.LoadWeights(path)
	if err == nil || !strings.Contains(err.Error(), "schema version") {
		t.Errorf("expected version-mismatch error, got %v", err)
	}
}

func TestPredictor_P3_NewPredictorFromFile_MissingFileFallsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "never-existed.json")
	p, err := NewPredictorFromFile(path)
	if err != nil {
		t.Fatalf("missing file must not be an error: %v", err)
	}
	if p == nil {
		t.Fatal("missing file must yield a default predictor")
	}
	// Sanity: default weights present.
	if p.weight["retries"] == 0 {
		t.Error("default predictor must have non-zero retries weight")
	}
}

func TestPredictor_P3_SaveRejectsEmptyPath(t *testing.T) {
	p := NewPredictor()
	err := p.SaveWeights("")
	if err == nil {
		t.Fatal("empty path must error")
	}
}
