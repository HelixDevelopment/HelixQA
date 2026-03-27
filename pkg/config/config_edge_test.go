// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfig_StepDelay_UnknownSpeed(t *testing.T) {
	cfg := &Config{Speed: "unknown"}
	// Default case returns normal delay.
	assert.Equal(t, 500*time.Millisecond, cfg.StepDelay())
}

func TestConfig_ExpandedPlatforms_EmptySlice(t *testing.T) {
	cfg := &Config{Platforms: []Platform{}}
	expanded := cfg.ExpandedPlatforms()
	assert.Empty(t, expanded)
}

func TestConfig_ExpandedPlatforms_AllMixed(t *testing.T) {
	// If "all" is anywhere in the list, it expands.
	cfg := &Config{Platforms: []Platform{
		PlatformAndroid, PlatformAll,
	}}
	expanded := cfg.ExpandedPlatforms()
	assert.Equal(t, []Platform{
		PlatformAndroid, PlatformAndroidTV,
		PlatformWeb, PlatformDesktop,
	}, expanded)
}

func TestParsePlatforms_DuplicatePlatforms(t *testing.T) {
	platforms, err := ParsePlatforms("android,android")
	assert.NoError(t, err)
	assert.Len(t, platforms, 2)
}

func TestParseBanks_OnlyCommas(t *testing.T) {
	banks := ParseBanks(",,,")
	assert.Empty(t, banks)
}

func TestParseBanks_OnlySpaces(t *testing.T) {
	banks := ParseBanks("   ")
	assert.Empty(t, banks)
}

func TestParseBanks_LeadingTrailingCommas(t *testing.T) {
	banks := ParseBanks(",a.json,")
	assert.Equal(t, []string{"a.json"}, banks)
}

func TestConfig_Validate_NegativeStepTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Banks = []string{"test.json"}
	cfg.StepTimeout = -5 * time.Second
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "step timeout")
}

func TestConfig_Validate_ZeroTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Banks = []string{"test.json"}
	cfg.Timeout = 0
	err := cfg.Validate()
	assert.Error(t, err)
}

func TestConfig_FullConfig(t *testing.T) {
	cfg := &Config{
		Banks:          []string{"bank.json"},
		Platforms:      []Platform{PlatformAndroid},
		Device:         "emulator-5554",
		PackageName:    "com.test",
		OutputDir:      "/tmp/output",
		Speed:          SpeedFast,
		ReportFormat:   ReportJSON,
		ValidateSteps:  true,
		Record:         true,
		Verbose:        true,
		Timeout:        1 * time.Hour,
		StepTimeout:    5 * time.Minute,
		BrowserURL:     "http://localhost:3000",
		DesktopProcess: "java",
		DesktopPID:     42,
	}
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestParsePlatforms_Whitespace(t *testing.T) {
	platforms, err := ParsePlatforms(" android ")
	assert.NoError(t, err)
	assert.Equal(t, []Platform{PlatformAndroid}, platforms)
}

func TestParsePlatforms_CaseSensitive(t *testing.T) {
	_, err := ParsePlatforms("Android")
	assert.Error(t, err)
}
