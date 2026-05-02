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
		PlatformAndroid, PlatformAndroidTV,
		PlatformWeb, PlatformDesktop,
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
	assert.Equal(t, 1*time.Second, cfg.StepDelay()) // Optimized from 2s
}

func TestConfig_StepDelay_Normal(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Speed = SpeedNormal
	assert.Equal(t, 100*time.Millisecond, cfg.StepDelay()) // Optimized from 500ms
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
	_, err := ParsePlatforms("foo")
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

func TestDefaultAutonomousConfig(t *testing.T) {
	ac := DefaultAutonomousConfig()
	assert.True(t, ac.Enabled)
	assert.Equal(t, 0.90, ac.CoverageTarget)
	assert.True(t, ac.CuriosityEnabled)
	assert.Equal(t, 30*time.Minute, ac.CuriosityTimeout)
	assert.Equal(t, 3, ac.AgentPoolSize)
	assert.Equal(t, 60*time.Second, ac.AgentTimeout)
	assert.Equal(t, 3, ac.AgentMaxRetries)
	assert.Equal(t, "auto", ac.VisionProvider)
	assert.True(t, ac.VisionOpenCVEnabled)
	assert.Equal(t, 0.95, ac.VisionSSIMThreshold)
	assert.Equal(t, "./docs", ac.DocsRoot)
	assert.True(t, ac.DocsAutoDiscover)
	assert.Len(t, ac.DocsFormats, 5)
	assert.True(t, ac.RecordingVideo)
	assert.True(t, ac.RecordingScreenshots)
	assert.Equal(t, "medium", ac.RecordingVideoQuality)
	assert.Equal(t, "png", ac.RecordingScreenshotFormat)
	assert.Equal(t, "/usr/bin/ffmpeg", ac.RecordingFFmpegPath)
	assert.Equal(t, "chromium", ac.WebBrowser)
	assert.Equal(t, ":0", ac.DesktopDisplay)
	assert.Len(t, ac.ReportFormats, 3)
	assert.True(t, ac.TicketsEnabled)
	assert.Equal(t, "low", ac.TicketsMinSeverity)
}

func TestDefaultAutonomousConfig_AudioDefaults(t *testing.T) {
	ac := DefaultAutonomousConfig()
	assert.False(t, ac.RecordingAudio)
	assert.Equal(t, "high", ac.RecordingAudioQuality)
	assert.Equal(t, "wav", ac.RecordingAudioFormat)
	assert.Equal(t, "default", ac.RecordingAudioDevice)
}

func TestConfig_AudioRecordingQualityValues(t *testing.T) {
	validQualities := []string{"standard", "high", "ultra"}
	ac := DefaultAutonomousConfig()
	assert.Contains(t, validQualities, ac.RecordingAudioQuality)

	// Verify each quality can be set.
	for _, q := range validQualities {
		ac.RecordingAudioQuality = q
		assert.Equal(t, q, ac.RecordingAudioQuality)
	}

	// Verify formats.
	validFormats := []string{"wav", "flac"}
	assert.Contains(t, validFormats, ac.RecordingAudioFormat)
	for _, f := range validFormats {
		ac.RecordingAudioFormat = f
		assert.Equal(t, f, ac.RecordingAudioFormat)
	}
}

func TestAutonomousConfig_AgentsEnabled(t *testing.T) {
	ac := DefaultAutonomousConfig()
	assert.Contains(t, ac.AgentsEnabled, "opencode")
	assert.Contains(t, ac.AgentsEnabled, "claude-code")
	assert.Contains(t, ac.AgentsEnabled, "gemini")
}

func TestAutonomousConfig_DocsFormats(t *testing.T) {
	ac := DefaultAutonomousConfig()
	assert.Contains(t, ac.DocsFormats, "md")
	assert.Contains(t, ac.DocsFormats, "yaml")
	assert.Contains(t, ac.DocsFormats, "html")
	assert.Contains(t, ac.DocsFormats, "adoc")
	assert.Contains(t, ac.DocsFormats, "rst")
}

func TestConfig_AutonomousField(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Autonomous = DefaultAutonomousConfig()
	assert.True(t, cfg.Autonomous.Enabled)
}

func TestConfig_ValidateWithAutonomous(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Banks = []string{"test.json"}
	cfg.Autonomous = DefaultAutonomousConfig()
	assert.NoError(t, cfg.Validate())
}
