// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// --- pure-parse unit tests (no ffmpeg needed) ---

func TestParseAstats_OverallAndPerChannel(t *testing.T) {
	// A representative astats stderr: two channel blocks then Overall.
	out := `
[Parsed_astats_0 @ 0x1] Channel: 1
[Parsed_astats_0 @ 0x1] RMS level dB: -22.500000
[Parsed_astats_0 @ 0x1] Peak level dB: -6.020600
[Parsed_astats_0 @ 0x1] Channel: 2
[Parsed_astats_0 @ 0x1] RMS level dB: -20.100000
[Parsed_astats_0 @ 0x1] Peak level dB: -5.000000
[Parsed_astats_0 @ 0x1] Overall
[Parsed_astats_0 @ 0x1] DC offset: 0.000012
[Parsed_astats_0 @ 0x1] RMS level dB: -21.300000
[Parsed_astats_0 @ 0x1] Peak level dB: -5.000000
[Parsed_astats_0 @ 0x1] Flat factor: 0.000000
[Parsed_astats_0 @ 0x1] Peak count: 2.000000
`
	p := &presence{}
	parseAstats(out, p)
	if len(p.PerChannelRMS) != 2 {
		t.Fatalf("want 2 per-channel RMS, got %v", p.PerChannelRMS)
	}
	if p.RMSLevelDB != -21.3 {
		t.Fatalf("overall RMS want -21.3, got %v", p.RMSLevelDB)
	}
	if p.PeakLevelDB != -5.0 {
		t.Fatalf("overall peak want -5.0, got %v", p.PeakLevelDB)
	}
}

func TestDBValue_Sentinels(t *testing.T) {
	if dbValue("-inf") != -120.0 || dbValue("nan") != -120.0 {
		t.Fatalf("inf/nan must map to -120 (silence)")
	}
	if dbValue("-18.5") != -18.5 {
		t.Fatalf("numeric passthrough failed")
	}
}

func TestSilenceRegex(t *testing.T) {
	out := `[silencedetect @ 0x1] silence_start: 1.2
[silencedetect @ 0x1] silence_end: 3.4 | silence_duration: 2.2
[silencedetect @ 0x1] silence_end: 9.0 | silence_duration: 0.6`
	ms := reSilEnd.FindAllStringSubmatch(out, -1)
	if len(ms) != 2 {
		t.Fatalf("want 2 silence events, got %d", len(ms))
	}
	if atofSafe(ms[0][1]) != 2.2 {
		t.Fatalf("first gap 2.2, got %s", ms[0][1])
	}
}

func TestAnalyze_MissingInput(t *testing.T) {
	env, code := analyze(analyzeCfg{Input: ""})
	if code != 2 || env.OK {
		t.Fatalf("empty input should be exit 2, got %d ok=%v", code, env.OK)
	}
}

// --- real-ffmpeg anti-bluff e2e (§11.4.27): generate known WAVs, prove the
//     analyzer reads real channels / presence / verdict over real samples. ---

func haveFF(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("SKIP-OK: ffmpeg not installed (§11.4.3 topology skip)")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("SKIP-OK: ffprobe not installed (§11.4.3 topology skip)")
	}
}

func genWav(t *testing.T, path string, args ...string) {
	t.Helper()
	full := append([]string{"-y"}, args...)
	full = append(full, path)
	cmd := exec.Command("ffmpeg", full...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg gen failed: %v\n%s", err, out)
	}
}

// A 6-channel sine WAV → PASS, channels==6, present.
func TestAnalyze_SixChannelSine_PASS(t *testing.T) {
	haveFF(t)
	dir := t.TempDir()
	wav := filepath.Join(dir, "six.wav")
	genWav(t, wav, "-f", "lavfi", "-i", "sine=frequency=440:duration=2:sample_rate=48000", "-ac", "6")
	env, code := analyze(analyzeCfg{Input: wav, ExpectChannels: 6, MinRMSDB: -50, SilenceNoiseDB: -50, SilenceMinSec: 0.5, ClipThreshDB: -0.1})
	if code != 0 {
		t.Fatalf("exit want 0, got %d (%s)", code, env.Error)
	}
	if env.Channels != 6 {
		t.Fatalf("channels want 6, got %d layout=%s", env.Channels, env.ChannelLayout)
	}
	if env.Presence == nil || env.Presence.Silent {
		t.Fatalf("6ch sine must be present (not silent): %+v", env.Presence)
	}
	if env.Verdict != "PASS" {
		t.Fatalf("verdict want PASS, got %s reasons=%v", env.Verdict, env.Reasons)
	}
}

// A silent (anullsrc) stereo WAV → FAIL (no audible signal).
func TestAnalyze_SilentStereo_FAIL(t *testing.T) {
	haveFF(t)
	dir := t.TempDir()
	wav := filepath.Join(dir, "silent.wav")
	genWav(t, wav, "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo", "-t", "2")
	env, code := analyze(analyzeCfg{Input: wav, MinRMSDB: -50, SilenceNoiseDB: -50, SilenceMinSec: 0.5, ClipThreshDB: -0.1})
	if code != 0 {
		t.Fatalf("exit want 0 (verdict path), got %d", code)
	}
	if env.Presence == nil || !env.Presence.Silent {
		t.Fatalf("anullsrc must be detected SILENT, got %+v", env.Presence)
	}
	if env.Verdict != "FAIL" {
		t.Fatalf("silent audio verdict want FAIL, got %s", env.Verdict)
	}
}

// A stereo sine with -expect-channels 6 → FAIL (downmix/collapse detector).
func TestAnalyze_StereoExpect6_DownmixFAIL(t *testing.T) {
	haveFF(t)
	dir := t.TempDir()
	wav := filepath.Join(dir, "stereo.wav")
	genWav(t, wav, "-f", "lavfi", "-i", "sine=frequency=440:duration=2:sample_rate=48000", "-ac", "2")
	env, code := analyze(analyzeCfg{Input: wav, ExpectChannels: 6, MinRMSDB: -50, SilenceNoiseDB: -50, SilenceMinSec: 0.5, ClipThreshDB: -0.1})
	if code != 0 {
		t.Fatalf("exit want 0, got %d", code)
	}
	if env.Channels != 2 {
		t.Fatalf("channels want 2, got %d", env.Channels)
	}
	if env.Verdict != "FAIL" {
		t.Fatalf("stereo-vs-expect-6 verdict want FAIL (collapse), got %s reasons=%v", env.Verdict, env.Reasons)
	}
}
