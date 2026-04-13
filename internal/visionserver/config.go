// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package visionserver provides an HTTP server that exposes cheaper vision
// analysis capabilities over a simple JSON API.
package visionserver

import (
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for the vision server. Fields are
// populated by LoadConfig from HELIX_VISION_* environment variables; callers
// may also construct a Config directly (e.g. in tests) and pass it to
// NewServer.
type Config struct {
	// Provider is the name of the primary vision provider to use (e.g.
	// "auto", "qwen25vl", "glm4v").
	Provider string

	// FallbackEnabled controls whether the server tries the FallbackChain
	// when the primary provider fails.
	FallbackEnabled bool

	// FallbackChain is the ordered list of provider names to try after the
	// primary provider fails. Only used when FallbackEnabled is true.
	FallbackChain []string

	// ParallelExecution enables firing all providers concurrently and
	// returning the fastest successful result.
	ParallelExecution bool

	// Timeout is the per-request deadline applied to each vision analysis
	// call. A zero value means no deadline.
	Timeout time.Duration

	// LearningEnabled enables the few-shot learning subsystem so that past
	// results inform future prompts.
	LearningEnabled bool

	// ExactCache enables exact-match result caching keyed on a hash of the
	// image bytes and prompt.
	ExactCache bool

	// Differential enables differential (delta-frame) caching that reuses
	// results when the current frame is visually similar to a recent one.
	Differential bool

	// VectorMemory enables semantic vector memory for long-term learning
	// across sessions.
	VectorMemory bool

	// FewShot enables retrieval of few-shot examples from past sessions to
	// inject into prompts.
	FewShot bool

	// PersistPath is the filesystem path where persistent learning state is
	// stored. An empty string disables persistence.
	PersistPath string

	// MaxMemories is the upper bound on the number of past observations
	// retained in memory.
	MaxMemories int

	// ChangeThreshold is the minimum normalised pixel-difference ratio
	// required to consider two consecutive frames "changed". Range: [0, 1].
	ChangeThreshold float64

	// ListenAddr is the TCP address and port the HTTP server listens on
	// (e.g. ":8090").
	ListenAddr string
}

// LoadConfig reads HELIX_VISION_* environment variables and returns a Config
// populated with their values. Missing or unparseable variables fall back to
// the documented defaults so that the server is always runnable without any
// explicit configuration.
func LoadConfig() *Config {
	c := &Config{
		Provider:          getEnvOrDefault("HELIX_VISION_PROVIDER", "auto"),
		FallbackEnabled:   getEnvBool("HELIX_VISION_FALLBACK_ENABLED", true),
		FallbackChain:     getEnvStringSlice("HELIX_VISION_FALLBACK_CHAIN", []string{"qwen25vl", "glm4v", "uitars", "showui"}),
		ParallelExecution: getEnvBool("HELIX_VISION_PARALLEL", true),
		Timeout:           getEnvDuration("HELIX_VISION_TIMEOUT", 30*time.Second),
		LearningEnabled:   getEnvBool("HELIX_VISION_LEARNING", true),
		ExactCache:        getEnvBool("HELIX_VISION_EXACT_CACHE", true),
		Differential:      getEnvBool("HELIX_VISION_DIFFERENTIAL", true),
		VectorMemory:      getEnvBool("HELIX_VISION_VECTOR_MEMORY", true),
		FewShot:           getEnvBool("HELIX_VISION_FEW_SHOT", true),
		PersistPath:       getEnvOrDefault("HELIX_VISION_PERSIST_PATH", ""),
		MaxMemories:       getEnvInt("HELIX_VISION_MAX_MEMORIES", 100000),
		ChangeThreshold:   getEnvFloat64("HELIX_VISION_CHANGE_THRESHOLD", 0.05),
		ListenAddr:        getEnvOrDefault("HELIX_VISION_LISTEN_ADDR", ":8090"),
	}
	return c
}

// getEnvOrDefault returns the value of the named environment variable, or
// defaultVal when the variable is unset or empty.
func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// getEnvBool parses the named environment variable as a boolean. Accepted
// truthy strings (case-insensitive): "1", "true", "yes", "on". All other
// non-empty values are treated as false. When the variable is unset the
// defaultVal is returned.
func getEnvBool(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// getEnvInt parses the named environment variable as a base-10 integer.
// Returns defaultVal when the variable is unset or cannot be parsed.
func getEnvInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

// getEnvFloat64 parses the named environment variable as a float64. Returns
// defaultVal when the variable is unset, cannot be parsed, or is NaN/Inf.
func getEnvFloat64(key string, defaultVal float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return defaultVal
	}
	return f
}

// getEnvDuration parses the named environment variable using time.ParseDuration.
// Returns defaultVal when the variable is unset or cannot be parsed.
func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return defaultVal
	}
	return d
}

// getEnvStringSlice parses the named environment variable as a
// comma-separated list of strings. Returns defaultVal when the variable is
// unset or empty.
func getEnvStringSlice(key string, defaultVal []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	if len(result) == 0 {
		return defaultVal
	}
	return result
}
