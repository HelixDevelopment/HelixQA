// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command qa-audio-analyze is a project-agnostic "HEAR" analyzer: it turns a
// CAPTURED audio (or A/V) file into a structured, anti-bluff audio verdict by
// running REAL ffprobe + ffmpeg filters over the actual samples — NOT merely
// asserting "recording succeeded".
//
// It answers the three questions the operator's mandate asks of captured audio
// (§11.4.5 audio-quality census + §11.4.69 audio_output sink evidence):
//
//	(1) CHANNEL COUNT + layout + sample-rate + codec (ffprobe) — proves the
//	    output is the claimed 2.0 / 5.1 / 7.1, catches a downmix collapse.
//	(2) PRESENCE — overall + per-channel RMS/peak level (ffmpeg astats) +
//	    mean/max volume (volumedetect) — proves there is an audible signal,
//	    not digital silence.
//	(3) GLITCH CENSUS on the captured signal — silence-gap/dropout events
//	    (silencedetect), clipping (astats peak count at 0 dBFS), DC offset,
//	    flat-region factor. (Hardware XRUN/underrun counters come from the
//	    device's own logcat, collected separately by the caller; this is the
//	    captured-signal analog.)
//
// It is fully decoupled from any consuming project (§11.4.28): it reads only a
// file path + flags and prints structured JSON. It shells out to the standard
// ffprobe/ffmpeg binaries and imports nothing beyond the Go stdlib, so it
// builds and runs anywhere. When ffprobe/ffmpeg is absent it emits an honest
// SKIP-with-reason envelope (§11.4.3) — never a fabricated PASS.
//
// Usage:
//
//	qa-audio-analyze -input capture.wav [-expect-channels 6] [-min-rms-db -50] \
//	    [-silence-noise-db -50] [-silence-min-sec 0.5]
//
// Exit codes: 0 = analysis produced a verdict (see .verdict field), 2 = bad
// input, 3 = ffprobe/ffmpeg unavailable (SKIP), 4 = the file has no audio.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// audioStream is one ffprobe audio stream row.
type audioStream struct {
	Index         int     `json:"index"`
	Codec         string  `json:"codec"`
	Channels      int     `json:"channels"`
	ChannelLayout string  `json:"channel_layout"`
	SampleRate    int     `json:"sample_rate"`
	BitsPerSample int     `json:"bits_per_sample"`
	DurationSec   float64 `json:"duration_sec"`
}

// presence holds the loudness/RMS evidence proving there is a real signal.
type presence struct {
	RMSLevelDB       float64   `json:"rms_level_db"`   // overall astats RMS
	PeakLevelDB      float64   `json:"peak_level_db"`  // overall astats peak
	MeanVolumeDB     float64   `json:"mean_volume_db"` // volumedetect mean
	MaxVolumeDB      float64   `json:"max_volume_db"`  // volumedetect max
	PerChannelRMS    []float64 `json:"per_channel_rms_db"`
	Silent           bool      `json:"silent"` // overall RMS below the -min-rms-db floor (no audible signal)
	HaveAstats       bool      `json:"have_astats"`
	HaveVolumedetect bool      `json:"have_volumedetect"`
}

// glitchCensus is the captured-signal defect census.
type glitchCensus struct {
	SilenceEvents     int     `json:"silence_events"`    // silencedetect gap count
	SilenceTotalSec   float64 `json:"silence_total_sec"` // sum of gap durations
	LongestSilence    float64 `json:"longest_silence_sec"`
	ClippingPeakCount int     `json:"clipping_peak_count"` // astats "Peak count" (samples at full scale)
	ClippingDetected  bool    `json:"clipping_detected"`
	DCOffset          float64 `json:"dc_offset"`
	FlatFactor        float64 `json:"flat_factor"` // astats flat-region factor (a stuck/flat signal)
}

// verdictEnvelope is the top-level machine-parseable output.
type verdictEnvelope struct {
	OK             bool          `json:"ok"`
	Input          string        `json:"input"`
	AudioStreams   []audioStream `json:"audio_streams"`
	Channels       int           `json:"channels"` // selected stream channel count
	ChannelLayout  string        `json:"channel_layout"`
	SampleRate     int           `json:"sample_rate"`
	Codec          string        `json:"codec"`
	Presence       *presence     `json:"presence"`
	GlitchCensus   *glitchCensus `json:"glitch_census"`
	ExpectChannels int           `json:"expect_channels,omitempty"`
	Verdict        string        `json:"verdict"` // PASS | DEGRADED | FAIL | SKIP
	Reasons        []string      `json:"reasons"`
	Error          string        `json:"error,omitempty"`
	Skip           string        `json:"skip,omitempty"`
	AnalyzeMS      int64         `json:"analyze_ms"`
}

// analyzeCfg holds the flag-driven analysis parameters (split out for tests).
type analyzeCfg struct {
	Input          string
	ExpectChannels int
	MinRMSDB       float64
	SilenceNoiseDB float64
	SilenceMinSec  float64
	ClipThreshDB   float64
	StreamIdx      int
}

func main() {
	var cfg analyzeCfg
	flag.StringVar(&cfg.Input, "input", "", "path to the captured audio/AV file to analyze")
	flag.IntVar(&cfg.ExpectChannels, "expect-channels", 0, "if >0, FAIL when the captured audio has fewer channels (downmix/collapse detector)")
	flag.Float64Var(&cfg.MinRMSDB, "min-rms-db", -50.0, "overall RMS below this (dBFS) is treated as digital silence (no audible signal)")
	flag.Float64Var(&cfg.SilenceNoiseDB, "silence-noise-db", -50.0, "silencedetect noise floor in dB")
	flag.Float64Var(&cfg.SilenceMinSec, "silence-min-sec", 0.5, "silencedetect minimum gap duration in seconds")
	flag.Float64Var(&cfg.ClipThreshDB, "clip-peak-db", -0.1, "peak level at/above this (dBFS) counts as clipping")
	flag.IntVar(&cfg.StreamIdx, "stream", 0, "which audio stream (0:a:N) to analyze for presence/glitch")
	flag.Parse()

	env, code := analyze(cfg)
	emit(env)
	os.Exit(code)
}

// analyze runs the full ffprobe+ffmpeg analysis and returns the envelope + the
// process exit code. Split from main() so tests can drive it end-to-end with
// real ffmpeg over real generated WAVs (§11.4.27 no-fakes-beyond-unit).
func analyze(cfg analyzeCfg) (verdictEnvelope, int) {
	start := time.Now()

	if cfg.Input == "" {
		return verdictEnvelope{OK: false, Error: "missing -input <file>"}, 2
	}
	if fi, err := os.Stat(cfg.Input); err != nil || fi.Size() == 0 {
		return verdictEnvelope{OK: false, Input: cfg.Input, Error: "input file missing or 0 bytes (capture failed)"}, 2
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return verdictEnvelope{OK: false, Input: cfg.Input, Verdict: "SKIP", Skip: "ffprobe not installed", Reasons: []string{"ffprobe unavailable — cannot analyze channels"}}, 3
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return verdictEnvelope{OK: false, Input: cfg.Input, Verdict: "SKIP", Skip: "ffmpeg not installed", Reasons: []string{"ffmpeg unavailable — cannot analyze presence/glitch"}}, 3
	}

	env := verdictEnvelope{OK: true, Input: cfg.Input, ExpectChannels: cfg.ExpectChannels}

	streams, err := probeStreams(cfg.Input)
	if err != nil {
		env.OK = false
		env.Error = "ffprobe failed: " + err.Error()
		return env, 2
	}
	env.AudioStreams = streams
	if len(streams) == 0 {
		env.Verdict = "FAIL"
		env.Reasons = []string{"the file has NO audio stream — capture produced video-only or empty audio"}
		env.AnalyzeMS = time.Since(start).Milliseconds()
		return env, 4
	}
	sel := 0
	if cfg.StreamIdx >= 0 && cfg.StreamIdx < len(streams) {
		sel = cfg.StreamIdx
	}
	s := streams[sel]
	env.Channels = s.Channels
	env.ChannelLayout = s.ChannelLayout
	env.SampleRate = s.SampleRate
	env.Codec = s.Codec

	pres := analyzePresence(cfg.Input, cfg.StreamIdx, cfg.MinRMSDB)
	env.Presence = pres
	gc := analyzeGlitch(cfg.Input, cfg.StreamIdx, cfg.SilenceNoiseDB, cfg.SilenceMinSec, cfg.ClipThreshDB)
	env.GlitchCensus = gc

	// Verdict.
	switch {
	case pres == nil || !pres.HaveAstats:
		env.Verdict = "DEGRADED"
		env.Reasons = append(env.Reasons, "could not compute RMS presence (astats failed) — channel metadata read but signal presence unproven")
	case pres.Silent:
		env.Verdict = "FAIL"
		env.Reasons = append(env.Reasons, fmt.Sprintf("captured audio is SILENT (overall RMS %.1f dBFS below the %.1f dB floor) — no audible signal reached the sink", pres.RMSLevelDB, cfg.MinRMSDB))
	case cfg.ExpectChannels > 0 && s.Channels < cfg.ExpectChannels:
		env.Verdict = "FAIL"
		env.Reasons = append(env.Reasons, fmt.Sprintf("channel collapse: captured %d channels (%s) < expected %d — a downmix/collapse regression (§11.4.111)", s.Channels, s.ChannelLayout, cfg.ExpectChannels))
	default:
		env.Verdict = "PASS"
		env.Reasons = append(env.Reasons, fmt.Sprintf("audible signal present (RMS %.1f dBFS, peak %.1f dBFS) over %d channel(s) (%s) @ %d Hz %s", pres.RMSLevelDB, pres.PeakLevelDB, s.Channels, s.ChannelLayout, s.SampleRate, s.Codec))
	}
	// Downgrade a passing verdict to DEGRADED on a real captured-signal defect.
	if env.Verdict == "PASS" && gc != nil {
		if gc.ClippingDetected {
			env.Verdict = "DEGRADED"
			env.Reasons = append(env.Reasons, fmt.Sprintf("clipping detected (%d full-scale peak samples) — distortion", gc.ClippingPeakCount))
		} else if gc.LongestSilence >= 1.5 {
			env.Verdict = "DEGRADED"
			env.Reasons = append(env.Reasons, fmt.Sprintf("a %.1fs silence gap in the capture window — possible dropout/underrun", gc.LongestSilence))
		}
	}
	if gc != nil {
		env.Reasons = append(env.Reasons, fmt.Sprintf("glitch census: %d silence-gap event(s) totalling %.2fs (longest %.2fs), clipping=%v, dc_offset=%.4f, flat_factor=%.3f",
			gc.SilenceEvents, gc.SilenceTotalSec, gc.LongestSilence, gc.ClippingDetected, gc.DCOffset, gc.FlatFactor))
	}

	env.AnalyzeMS = time.Since(start).Milliseconds()
	return env, 0
}

// probeStreams runs ffprobe and returns the audio streams.
func probeStreams(input string) ([]audioStream, error) {
	out, err := exec.Command("ffprobe", "-v", "error", "-print_format", "json",
		"-show_streams", "-show_format", input).Output()
	if err != nil {
		return nil, err
	}
	var raw struct {
		Streams []struct {
			Index         int    `json:"index"`
			CodecType     string `json:"codec_type"`
			CodecName     string `json:"codec_name"`
			Channels      int    `json:"channels"`
			ChannelLayout string `json:"channel_layout"`
			SampleRate    string `json:"sample_rate"`
			BitsPerSample int    `json:"bits_per_sample"`
			Duration      string `json:"duration"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	var streams []audioStream
	for _, s := range raw.Streams {
		if s.CodecType != "audio" {
			continue
		}
		streams = append(streams, audioStream{
			Index:         s.Index,
			Codec:         s.CodecName,
			Channels:      s.Channels,
			ChannelLayout: s.ChannelLayout,
			SampleRate:    atoiSafe(s.SampleRate),
			BitsPerSample: s.BitsPerSample,
			DurationSec:   atofSafe(s.Duration),
		})
	}
	return streams, nil
}

var (
	// -inf / -nan carry a leading minus, so the alternation must sit INSIDE the
	// optional sign group — otherwise "RMS level dB: -inf" fails to match and a
	// silent capture is misread as 0 dBFS (the §11.4.6 forbidden false read).
	reRMS       = regexp.MustCompile(`RMS level dB:\s*(-?(?:[0-9.]+|inf|nan))`)
	rePeak      = regexp.MustCompile(`Peak level dB:\s*(-?(?:[0-9.]+|inf|nan))`)
	reChannelHd = regexp.MustCompile(`Channel:\s*\d+`)
	reOverall   = regexp.MustCompile(`\bOverall\b`)
	rePeakCount = regexp.MustCompile(`Peak count:\s*([0-9.]+)`)
	reDCOffset  = regexp.MustCompile(`DC offset:\s*(-?[0-9.]+)`)
	reFlatFac   = regexp.MustCompile(`Flat factor:\s*([0-9.]+)`)
	reMeanVol   = regexp.MustCompile(`mean_volume:\s*(-?[0-9.]+) dB`)
	reMaxVol    = regexp.MustCompile(`max_volume:\s*(-?[0-9.]+) dB`)
	reSilEnd    = regexp.MustCompile(`silence_end:\s*[0-9.]+\s*\|\s*silence_duration:\s*([0-9.]+)`)
)

// analyzePresence computes overall + per-channel RMS/peak via astats and
// mean/max volume via volumedetect (a real filter pass over the samples).
func analyzePresence(input string, streamIdx int, minRMSDB float64) *presence {
	p := &presence{}
	amap := fmt.Sprintf("0:a:%d", streamIdx)

	// astats: per-channel then Overall.
	astatsOut := ffmpegFilter(input, amap, "astats=metadata=1:reset=0")
	if astatsOut != "" {
		p.HaveAstats = true
		parseAstats(astatsOut, p)
	}
	// volumedetect: mean/max volume.
	volOut := ffmpegFilter(input, amap, "volumedetect")
	if volOut != "" {
		if m := reMeanVol.FindStringSubmatch(volOut); len(m) == 2 {
			p.HaveVolumedetect = true
			p.MeanVolumeDB = atofSafe(m[1])
		}
		if m := reMaxVol.FindStringSubmatch(volOut); len(m) == 2 {
			p.MaxVolumeDB = atofSafe(m[1])
		}
	}
	// Silent if overall RMS below the floor (astats -inf maps to -120), with
	// volumedetect mean as a defense-in-depth cross-check (§11.4.6 — two
	// independent measures agree before we call a capture silent).
	if p.HaveAstats {
		p.Silent = p.RMSLevelDB <= minRMSDB
	}
	if p.HaveVolumedetect && p.MeanVolumeDB <= minRMSDB {
		p.Silent = true
	}
	return p
}

// parseAstats extracts overall RMS/peak/peak-count/dc/flat and per-channel RMS.
// astats prints a block per channel, then an "Overall" block. The values AFTER
// the Overall marker are the whole-signal aggregates.
func parseAstats(out string, p *presence) {
	lines := strings.Split(out, "\n")
	inOverall := false
	var perCh []float64
	for _, ln := range lines {
		if reChannelHd.MatchString(ln) {
			inOverall = false
		}
		if reOverall.MatchString(ln) {
			inOverall = true
		}
		if m := reRMS.FindStringSubmatch(ln); len(m) == 2 {
			v := dbValue(m[1])
			if inOverall {
				p.RMSLevelDB = v
			} else {
				perCh = append(perCh, v)
			}
		}
		if inOverall {
			if m := rePeak.FindStringSubmatch(ln); len(m) == 2 {
				p.PeakLevelDB = dbValue(m[1])
			}
		}
	}
	p.PerChannelRMS = perCh
}

// analyzeGlitch runs silencedetect (dropout gaps) + reuses astats for
// clipping (peak count) / DC offset / flat factor.
func analyzeGlitch(input string, streamIdx int, noiseDB, minSec, clipDB float64) *glitchCensus {
	gc := &glitchCensus{}
	amap := fmt.Sprintf("0:a:%d", streamIdx)

	silOut := ffmpegFilter(input, amap, fmt.Sprintf("silencedetect=noise=%gdB:d=%g", noiseDB, minSec))
	for _, m := range reSilEnd.FindAllStringSubmatch(silOut, -1) {
		d := atofSafe(m[1])
		gc.SilenceEvents++
		gc.SilenceTotalSec += d
		if d > gc.LongestSilence {
			gc.LongestSilence = d
		}
	}

	astatsOut := ffmpegFilter(input, amap, "astats=metadata=1:reset=0")
	// Peak count / DC offset / flat factor come from the Overall block.
	inOverall := false
	var peakDB float64
	for _, ln := range strings.Split(astatsOut, "\n") {
		if reOverall.MatchString(ln) {
			inOverall = true
		}
		if inOverall {
			if m := rePeakCount.FindStringSubmatch(ln); len(m) == 2 {
				gc.ClippingPeakCount = int(atofSafe(m[1]))
			}
			if m := reDCOffset.FindStringSubmatch(ln); len(m) == 2 {
				gc.DCOffset = atofSafe(m[1])
			}
			if m := reFlatFac.FindStringSubmatch(ln); len(m) == 2 {
				gc.FlatFactor = atofSafe(m[1])
			}
			if m := rePeak.FindStringSubmatch(ln); len(m) == 2 {
				peakDB = dbValue(m[1])
			}
		}
	}
	// Clipping = the overall peak is at/above the clip threshold AND at least a
	// handful of samples sit at full scale (the astats peak-count).
	gc.ClippingDetected = peakDB >= clipDB && gc.ClippingPeakCount > 4
	return gc
}

// ffmpegFilter runs one ffmpeg filter pass over one audio stream to null and
// returns the combined stderr (where the analysis filters print). Empty on
// failure so the caller can degrade honestly.
func ffmpegFilter(input, amap, filter string) string {
	cmd := exec.Command("ffmpeg", "-hide_banner", "-nostats",
		"-i", input, "-map", amap, "-af", filter, "-f", "null", "-")
	var buf strings.Builder
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	_ = cmd.Run() // filters print to stderr even on a clean exit; ignore exit code
	return buf.String()
}

// dbValue maps astats "inf"/"nan" sentinels to a very-low dB (silence).
func dbValue(s string) float64 {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "inf", "-inf", "nan":
		return -120.0
	}
	return atofSafe(s)
}

func atoiSafe(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func atofSafe(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

func emit(e verdictEnvelope) {
	b, _ := json.Marshal(e)
	fmt.Println(string(b))
}
