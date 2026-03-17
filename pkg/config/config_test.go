// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, []Platform{PlatformAll}, cfg.Platforms)
	assert.Equal(t, "qa-results", cfg.OutputDir)
	assert.Equal(t, SpeedNormal, cfg.Speed)
	assert.Equal(t, ReportMarkdown, cfg.ReportFormat)
	assert.True(t, cfg.ValidateSteps)
	assert.True(t, cfg.Record)
	assert.Equal(t, 30*time.Minute, cfg.Timeout)
	assert.Equal(t, 2*time.Minute, cfg.StepTimeout)
}

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Banks = []string{"test.json"}
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_NoBanks(t *testing.T) {
	cfg := DefaultConfig()
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one test bank")
}

func TestConfig_Validate_NoOutputDir(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Banks = []string{"test.json"}
	cfg.OutputDir = ""
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "output directory")
}

func TestConfig_Validate_InvalidSpeed(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Banks = []string{"test.json"}
	cfg.Speed = "turbo"
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid speed")
}

func TestConfig_Validate_InvalidReportFormat(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Banks = []string{"test.json"}
	cfg.ReportFormat = "pdf"
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid report format")
}

func TestConfig_Validate_InvalidTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Banks = []string{"test.json"}
	cfg.Timeout = -1
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout must be positive")
}

func TestConfig_Validate_InvalidStepTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Banks = []string{"test.json"}
	cfg.StepTimeout = 0
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "step timeout must be positive")
}

func TestConfig_Validate_InvalidPlatform(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Banks = []string{"test.json"}
	cfg.Platforms = []Platform{"windows"}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid platform")
}

func TestConfig_ExpandedPlatforms_All(t *testing.T) {
	cfg := DefaultConfig()
	expanded := cfg.ExpandedPlatforms()
	assert.Equal(t, []Platform{
		PlatformAndroid, PlatformWeb, PlatformDesktop,
	}, expanded)
}

func TestConfig_ExpandedPlatforms_Specific(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Platforms = []Platform{PlatformAndroid, PlatformWeb}
	expanded := cfg.ExpandedPlatforms()
	assert.Equal(t, []Platform{
		PlatformAndroid, PlatformWeb,
	}, expanded)
}

func TestConfig_ExpandedPlatforms_Single(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Platforms = []Platform{PlatformDesktop}
	expanded := cfg.ExpandedPlatforms()
	assert.Equal(t, []Platform{PlatformDesktop}, expanded)
}

func TestConfig_StepDelay_Slow(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Speed = SpeedSlow
	assert.Equal(t, 2*time.Second, cfg.StepDelay())
}

func TestConfig_StepDelay_Normal(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Speed = SpeedNormal
	assert.Equal(t, 500*time.Millisecond, cfg.StepDelay())
}

func TestConfig_StepDelay_Fast(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Speed = SpeedFast
	assert.Equal(t, time.Duration(0), cfg.StepDelay())
}

func TestParsePlatforms_Empty(t *testing.T) {
	platforms, err := ParsePlatforms("")
	require.NoError(t, err)
	assert.Equal(t, []Platform{PlatformAll}, platforms)
}

func TestParsePlatforms_All(t *testing.T) {
	platforms, err := ParsePlatforms("all")
	require.NoError(t, err)
	assert.Equal(t, []Platform{PlatformAll}, platforms)
}

func TestParsePlatforms_Single(t *testing.T) {
	platforms, err := ParsePlatforms("android")
	require.NoError(t, err)
	assert.Equal(t, []Platform{PlatformAndroid}, platforms)
}

func TestParsePlatforms_Multiple(t *testing.T) {
	platforms, err := ParsePlatforms("android,web,desktop")
	require.NoError(t, err)
	assert.Equal(t, []Platform{
		PlatformAndroid, PlatformWeb, PlatformDesktop,
	}, platforms)
}

func TestParsePlatforms_WithSpaces(t *testing.T) {
	platforms, err := ParsePlatforms("android , web")
	require.NoError(t, err)
	assert.Equal(t, []Platform{
		PlatformAndroid, PlatformWeb,
	}, platforms)
}

func TestParsePlatforms_Invalid(t *testing.T) {
	_, err := ParsePlatforms("ios")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid platform")
}

func TestParseBanks_Empty(t *testing.T) {
	banks := ParseBanks("")
	assert.Nil(t, banks)
}

func TestParseBanks_Single(t *testing.T) {
	banks := ParseBanks("test.json")
	assert.Equal(t, []string{"test.json"}, banks)
}

func TestParseBanks_Multiple(t *testing.T) {
	banks := ParseBanks("a.json,b.json,c.json")
	assert.Equal(t, []string{
		"a.json", "b.json", "c.json",
	}, banks)
}

func TestParseBanks_WithSpaces(t *testing.T) {
	banks := ParseBanks("a.json , b.json")
	assert.Equal(t, []string{"a.json", "b.json"}, banks)
}

func TestParseBanks_SkipsEmpty(t *testing.T) {
	banks := ParseBanks("a.json,,b.json")
	assert.Equal(t, []string{"a.json", "b.json"}, banks)
}

func TestPlatformConstants(t *testing.T) {
	assert.Equal(t, Platform("android"), PlatformAndroid)
	assert.Equal(t, Platform("web"), PlatformWeb)
	assert.Equal(t, Platform("desktop"), PlatformDesktop)
	assert.Equal(t, Platform("all"), PlatformAll)
}

func TestSpeedModeConstants(t *testing.T) {
	assert.Equal(t, SpeedMode("slow"), SpeedSlow)
	assert.Equal(t, SpeedMode("normal"), SpeedNormal)
	assert.Equal(t, SpeedMode("fast"), SpeedFast)
}

func TestReportFormatConstants(t *testing.T) {
	assert.Equal(t, ReportFormat("markdown"), ReportMarkdown)
	assert.Equal(t, ReportFormat("html"), ReportHTML)
	assert.Equal(t, ReportFormat("json"), ReportJSON)
}

func TestConfig_Validate_AllSpeedModes(t *testing.T) {
	speeds := []SpeedMode{SpeedSlow, SpeedNormal, SpeedFast}
	for _, s := range speeds {
		cfg := DefaultConfig()
		cfg.Banks = []string{"test.json"}
		cfg.Speed = s
		assert.NoError(t, cfg.Validate(), "speed %s", s)
	}
}

func TestConfig_Validate_AllReportFormats(t *testing.T) {
	formats := []ReportFormat{
		ReportMarkdown, ReportHTML, ReportJSON,
	}
	for _, f := range formats {
		cfg := DefaultConfig()
		cfg.Banks = []string{"test.json"}
		cfg.ReportFormat = f
		assert.NoError(t, cfg.Validate(), "format %s", f)
	}
}

func TestConfig_Validate_AllPlatforms(t *testing.T) {
	platforms := []Platform{
		PlatformAndroid, PlatformWeb, PlatformDesktop,
		PlatformAll,
	}
	for _, p := range platforms {
		cfg := DefaultConfig()
		cfg.Banks = []string{"test.json"}
		cfg.Platforms = []Platform{p}
		assert.NoError(t, cfg.Validate(), "platform %s", p)
	}
}

func TestConfig_Validate_MultipleBanks(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Banks = []string{"a.json", "b.json", "c/"}
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_WithDevice(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Banks = []string{"test.json"}
	cfg.Device = "emulator-5554"
	cfg.PackageName = "com.example.app"
	assert.NoError(t, cfg.Validate())
}
