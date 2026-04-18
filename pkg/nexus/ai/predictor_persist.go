// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// P3 fix (docs/nexus/remaining-work.md): Predictor weights +
// bias are now persistable to disk so training does not evaporate
// across process restarts. The on-disk schema is a tiny JSON blob
// with a version field so future Predictor revisions can migrate.

// predictorFileSchema is the serialised wire format.
type predictorFileSchema struct {
	Version int                `json:"version"`
	Weight  map[string]float64 `json:"weight"`
	Bias    float64            `json:"bias"`
}

const predictorSchemaVersion = 1

// SaveWeights writes the Predictor's current weights + bias to path.
// The write is atomic: data is flushed to a sibling .tmp file first
// and then renamed into place, so a crash mid-write never produces
// a half-written predictor blob.
func (p *Predictor) SaveWeights(path string) error {
	if path == "" {
		return errors.New("predictor: empty save path")
	}
	p.mu.RLock()
	snapshot := predictorFileSchema{
		Version: predictorSchemaVersion,
		Weight:  make(map[string]float64, len(p.weight)),
		Bias:    p.bias,
	}
	for k, v := range p.weight {
		snapshot.Weight[k] = v
	}
	p.mu.RUnlock()

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("predictor: marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("predictor: mkdir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("predictor: write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("predictor: rename: %w", err)
	}
	return nil
}

// LoadWeights reads a predictor blob from path and merges its
// contents into p. Returns os.ErrNotExist when the file does not
// exist so callers can distinguish "no training yet" from "corrupt
// blob". The version field must match — stale schemas return an
// explicit error instead of silently loading mismatched weights.
func (p *Predictor) LoadWeights(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var snapshot predictorFileSchema
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("predictor: parse %q: %w", path, err)
	}
	if snapshot.Version != predictorSchemaVersion {
		return fmt.Errorf(
			"predictor: schema version mismatch at %q: got %d, want %d",
			path, snapshot.Version, predictorSchemaVersion,
		)
	}
	if len(snapshot.Weight) == 0 {
		return fmt.Errorf("predictor: %q has empty weight map", path)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.weight = snapshot.Weight
	p.bias = snapshot.Bias
	return nil
}

// NewPredictorFromFile is the convenience constructor operators call
// at process start: loads weights when the file exists, falls back
// to NewPredictor defaults otherwise. Any parse error is returned so
// the caller can decide whether to abort boot or to continue with
// defaults.
func NewPredictorFromFile(path string) (*Predictor, error) {
	p := NewPredictor()
	err := p.LoadWeights(path)
	switch {
	case err == nil:
		return p, nil
	case errors.Is(err, os.ErrNotExist):
		return p, nil
	default:
		return nil, err
	}
}
