// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-bridge is a loopback HTTP+SSE service that brokers
// between an LLM operator (Claude / HelixQA orchestrator) and the
// host-side dual-display recording + analyzer infrastructure
// (Bug #14.A dual_display_record.sh + Bug #14.B action_timeline.sh +
// Bug #14.C recording-analyzer).
//
// Endpoints (all loopback-only, no auth — listener pinned to 127.0.0.1):
//
//	POST /v1/recording/start    — start dual-display recording for a test
//	POST /v1/recording/stop     — stop recording and return file sizes
//	POST /v1/analyze/start      — spawn recording-analyzer streaming findings
//	GET  /v1/findings/stream    — SSE feed of analyzer findings (JSONL)
//	GET  /v1/health             — Whisper + Tesseract + filesystem probe
//	GET  /v1/timeline           — action_timeline.sh JSONL for a test
//
// Anti-bluff design (Constitution §11.4): every response carries enough
// captured evidence (file sizes, sync paths, exit codes, model versions)
// for the operator to verify the requested side-effect actually happened.
// "OK"-only responses without measurable state delta are forbidden.
//
// Project-agnostic: HelixQA core stays generic (per HELIXQA-AGNOSTIC
// Constitution rules). All consumer-specific paths (the bash helpers,
// the recording temp dir) come from request parameters or runtime
// configuration — no project-specific symbols are baked into
// `digital.vasic.helixqa/cmd/helixqa-bridge`.
//
// Run modes:
//
//	helixqa-bridge                 — start the server (blocks until SIGTERM/SIGINT)
//	helixqa-bridge --health-check  — one-shot probe; exits 0 iff all probes PASS
//
// Bounded by design: every external os/exec invocation is wrapped in a
// context.WithTimeout so a stuck shell helper cannot hold a request
// goroutine forever. The process gracefully shuts down on SIGTERM/SIGINT
// by closing the listener and draining outstanding SSE clients.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"digital.vasic.helixqa/pkg/audio"
)

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

// Config is everything the bridge needs at startup. All fields have
// sensible defaults so the zero value (after applyDefaults) is
// runnable.
type Config struct {
	// ListenAddr is the loopback bind address ("127.0.0.1:7842" by default).
	// MUST start with "127." to keep the bridge auth-free.
	ListenAddr string

	// RecordScript is the absolute path to dual_display_record.sh.
	// Resolved at startup. If empty, defaults to the repo-relative
	// location.
	RecordScript string

	// AnalyzerBinary is the absolute path to recording-analyzer (or just
	// "recording-analyzer" to look up via $PATH).
	AnalyzerBinary string

	// RecordingDir is where dual_display_record.sh writes its outputs
	// (mirrors the script's OUT_DIR). Used by /v1/health to verify
	// writability and by /v1/recording/stop to read back file sizes.
	RecordingDir string

	// FindingsFile is the JSONL stream the analyzer appends to. /v1/findings/stream
	// tails this file as it grows.
	FindingsFile string

	// TimelineDir is where action_timeline.sh writes per-test JSONL.
	// /v1/timeline reads from <TimelineDir>/<test_name>.jsonl.
	TimelineDir string

	// WhisperURL / TesseractURL — the audio-stack containers the
	// analyzer depends on. Probed by /v1/health.
	WhisperURL   string
	TesseractURL string

	// RecordTimeout caps a single dual_display_record.sh start/stop call.
	// 60s is generous: the script's screenrecord spin-up is sub-second
	// but the post-stop pull can take 30+ s for long recordings.
	RecordTimeout time.Duration

	// AnalyzeTimeout caps the analyzer process lifetime per /v1/analyze/start.
	// 300s default — analyzers stream findings continuously, but a single
	// run above this is suspicious.
	AnalyzeTimeout time.Duration

	// HealthTimeout caps the Whisper + Tesseract probe.
	HealthTimeout time.Duration
}

func (c *Config) applyDefaults() {
	if c.ListenAddr == "" {
		c.ListenAddr = "127.0.0.1:7842"
	}
	if c.RecordScript == "" {
		c.RecordScript = "scripts/testing/dual_display_record.sh"
	}
	if c.AnalyzerBinary == "" {
		c.AnalyzerBinary = "recording-analyzer"
	}
	if c.RecordingDir == "" {
		c.RecordingDir = "/tmp/__test_recording"
	}
	if c.FindingsFile == "" {
		c.FindingsFile = "/tmp/__recording_findings.jsonl"
	}
	if c.TimelineDir == "" {
		c.TimelineDir = "/tmp"
	}
	if c.WhisperURL == "" {
		c.WhisperURL = audio.DefaultWhisperBaseURL
	}
	if c.TesseractURL == "" {
		c.TesseractURL = audio.DefaultTesseractBaseURL
	}
	if c.RecordTimeout == 0 {
		c.RecordTimeout = 60 * time.Second
	}
	if c.AnalyzeTimeout == 0 {
		c.AnalyzeTimeout = 300 * time.Second
	}
	if c.HealthTimeout == 0 {
		c.HealthTimeout = 5 * time.Second
	}
}

// validateLoopback returns an error if ListenAddr is not a loopback bind.
// The bridge is unauthenticated by design — accidentally binding 0.0.0.0
// would expose recording control to the LAN.
func (c *Config) validateLoopback() error {
	host, _, err := net.SplitHostPort(c.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen address %q invalid: %w", c.ListenAddr, err)
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("listen address host %q is not a literal IP or 'localhost'", host)
	}
	if !ip.IsLoopback() {
		return fmt.Errorf("listen address %q is not loopback (127.x.x.x / ::1) — "+
			"bridge is unauthenticated and MUST NOT bind a routable address", host)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Wire types
// ---------------------------------------------------------------------------

type startRecordRequest struct {
	TestName     string `json:"test_name"`
	DeviceSerial string `json:"device_serial"`
	IntervalMS   int    `json:"interval_ms"` // hint, currently informational
}

type startRecordResponse struct {
	Status   string `json:"status"`
	SyncPath string `json:"sync_path"`
	TestName string `json:"test_name"`
	Detail   string `json:"detail,omitempty"`
}

type stopRecordRequest struct {
	TestName     string `json:"test_name"`
	DeviceSerial string `json:"device_serial"`
}

type stopRecordResponse struct {
	Status        string `json:"status"`
	PrimarySize   int64  `json:"primary_size"`
	SecondarySize int64  `json:"secondary_size"`
	SyncPath      string `json:"sync_path"`
	TestName      string `json:"test_name"`
	Detail        string `json:"detail,omitempty"`
}

type analyzeStartRequest struct {
	TestName   string `json:"test_name"`
	IntervalMS int    `json:"interval_ms"`
}

type analyzeStartResponse struct {
	Status   string `json:"status"`
	PID      int    `json:"pid"`
	TestName string `json:"test_name"`
	Detail   string `json:"detail,omitempty"`
}

type healthResponse struct {
	WhisperOK             bool   `json:"whisper_ok"`
	TesseractOK           bool   `json:"tesseract_ok"`
	RecordingDirWritable  bool   `json:"recording_dir_writable"`
	AnalyzerBinaryPresent bool   `json:"analyzer_binary_present"`
	RecordScriptPresent   bool   `json:"record_script_present"`
	WhisperDetail         string `json:"whisper_detail,omitempty"`
	TesseractDetail       string `json:"tesseract_detail,omitempty"`
	WhisperError          string `json:"whisper_error,omitempty"`
	TesseractError        string `json:"tesseract_error,omitempty"`
}

type errorResponse struct {
	Error  string `json:"error"`
	Detail string `json:"detail,omitempty"`
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// testNameRE locks test_name to a path-safe alphabet. The bridge
// concatenates test_name into filesystem paths (timeline lookup,
// findings filtering); without this guard, "../etc/passwd" would
// escape the timeline directory.
var testNameRE = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func validateTestName(name string) error {
	if name == "" {
		return errors.New("test_name is required")
	}
	if len(name) > 128 {
		return errors.New("test_name too long (>128 chars)")
	}
	if !testNameRE.MatchString(name) {
		return errors.New("test_name must match [A-Za-z0-9_-]+ (no path separators, no whitespace)")
	}
	return nil
}

// deviceSerialRE matches typical ADB serials (alnum + dot/dash/colon/
// underscore). Empty is allowed — the bash script picks the first
// device when serial is empty.
var deviceSerialRE = regexp.MustCompile(`^[A-Za-z0-9._:-]*$`)

func validateDeviceSerial(serial string) error {
	if len(serial) > 128 {
		return errors.New("device_serial too long (>128 chars)")
	}
	if !deviceSerialRE.MatchString(serial) {
		return errors.New("device_serial contains forbidden characters")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Command runner abstraction (testable)
// ---------------------------------------------------------------------------

// commandRunner is the seam tests use to inject a fake exec implementation.
// In production it's execRunner (wraps os/exec). In tests it's a lambda
// that records calls and returns canned output.
type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
	Start(ctx context.Context, name string, args ...string) (pid int, cleanup func(), err error)
}

// execRunner is the production commandRunner — straight os/exec.
type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

func (execRunner) Start(ctx context.Context, name string, args ...string) (int, func(), error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return 0, func() {}, err
	}
	pid := cmd.Process.Pid
	cleanup := func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}
	// Reap exit when ctx fires so we don't leak zombies.
	go func() {
		_ = cmd.Wait()
	}()
	return pid, cleanup, nil
}

// ---------------------------------------------------------------------------
// Bridge server
// ---------------------------------------------------------------------------

// Server is the bridge HTTP+SSE server.
type Server struct {
	cfg    Config
	runner commandRunner

	// audioProbe is overridable for tests; production path uses real
	// pkg/audio clients against cfg.WhisperURL / cfg.TesseractURL.
	audioProbe func(ctx context.Context, whisperURL, tesseractURL string, perTimeout time.Duration) (whisperOK bool, whisperDetail string, whisperErr error,
		tesseractOK bool, tesseractDetail string, tesseractErr error)

	// analyzerMu guards the active analyzer process slot. Only one
	// analyzer runs at a time per bridge instance — concurrent
	// analyzers would race on FindingsFile.
	analyzerMu     sync.Mutex
	analyzerActive bool
	analyzerPID    int
	analyzerCancel context.CancelFunc
	analyzerStop   func()
}

// NewServer constructs a Server with production defaults.
func NewServer(cfg Config) *Server {
	cfg.applyDefaults()
	return &Server{
		cfg:        cfg,
		runner:     execRunner{},
		audioProbe: defaultAudioProbe,
	}
}

// Handler returns the http.Handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/recording/start", s.handleRecordingStart)
	mux.HandleFunc("POST /v1/recording/stop", s.handleRecordingStop)
	mux.HandleFunc("POST /v1/analyze/start", s.handleAnalyzeStart)
	mux.HandleFunc("GET /v1/findings/stream", s.handleFindingsStream)
	mux.HandleFunc("GET /v1/health", s.handleHealth)
	mux.HandleFunc("GET /v1/timeline", s.handleTimeline)
	return mux
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (s *Server) handleRecordingStart(w http.ResponseWriter, r *http.Request) {
	var req startRecordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}
	if err := validateTestName(req.TestName); err != nil {
		writeError(w, http.StatusBadRequest, "invalid test_name", err.Error())
		return
	}
	if err := validateDeviceSerial(req.DeviceSerial); err != nil {
		writeError(w, http.StatusBadRequest, "invalid device_serial", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.RecordTimeout)
	defer cancel()

	args := []string{s.cfg.RecordScript, "start", req.TestName}
	if req.DeviceSerial != "" {
		args = append(args, req.DeviceSerial)
	}
	out, err := s.runner.Run(ctx, "bash", args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError,
			"recording start failed",
			fmt.Sprintf("exit error: %v; output: %s", err, truncate(string(out), 4096)))
		return
	}

	syncPath := filepath.Join(s.cfg.RecordingDir,
		fmt.Sprintf("%s__%s__sync.json", sanitizeForPath(req.TestName), pathDeviceSerial(req.DeviceSerial)))

	writeJSON(w, http.StatusOK, startRecordResponse{
		Status:   "started",
		SyncPath: syncPath,
		TestName: req.TestName,
		Detail:   truncate(strings.TrimSpace(string(out)), 1024),
	})
}

func (s *Server) handleRecordingStop(w http.ResponseWriter, r *http.Request) {
	var req stopRecordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}
	if err := validateTestName(req.TestName); err != nil {
		writeError(w, http.StatusBadRequest, "invalid test_name", err.Error())
		return
	}
	if err := validateDeviceSerial(req.DeviceSerial); err != nil {
		writeError(w, http.StatusBadRequest, "invalid device_serial", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.RecordTimeout)
	defer cancel()

	args := []string{s.cfg.RecordScript, "stop", req.TestName}
	if req.DeviceSerial != "" {
		args = append(args, req.DeviceSerial)
	}
	out, err := s.runner.Run(ctx, "bash", args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError,
			"recording stop failed",
			fmt.Sprintf("exit error: %v; output: %s", err, truncate(string(out), 4096)))
		return
	}

	prefix := filepath.Join(s.cfg.RecordingDir,
		fmt.Sprintf("%s__%s", sanitizeForPath(req.TestName), pathDeviceSerial(req.DeviceSerial)))
	primary := prefix + "__display_0.mp4"
	secondary := prefix + "__display_2.mp4"
	syncPath := prefix + "__sync.json"

	writeJSON(w, http.StatusOK, stopRecordResponse{
		Status:        "stopped",
		PrimarySize:   fileSize(primary),
		SecondarySize: fileSize(secondary),
		SyncPath:      syncPath,
		TestName:      req.TestName,
		Detail:        truncate(strings.TrimSpace(string(out)), 1024),
	})
}

func (s *Server) handleAnalyzeStart(w http.ResponseWriter, r *http.Request) {
	var req analyzeStartRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}
	if err := validateTestName(req.TestName); err != nil {
		writeError(w, http.StatusBadRequest, "invalid test_name", err.Error())
		return
	}

	s.analyzerMu.Lock()
	defer s.analyzerMu.Unlock()
	if s.analyzerActive {
		writeError(w, http.StatusConflict, "analyzer already running",
			fmt.Sprintf("PID %d active; stop it before starting a new run", s.analyzerPID))
		return
	}

	interval := req.IntervalMS
	if interval <= 0 {
		interval = 500
	}

	// AnalyzeTimeout bounds the lifetime; if the analyzer naturally
	// finishes earlier, the goroutine that calls Wait() will close out.
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.AnalyzeTimeout)

	args := []string{
		"--test-name", req.TestName,
		"--interval-ms", strconv.Itoa(interval),
		"--findings-file", s.cfg.FindingsFile,
	}
	pid, cleanup, err := s.runner.Start(ctx, s.cfg.AnalyzerBinary, args...)
	if err != nil {
		cancel()
		writeError(w, http.StatusInternalServerError,
			"analyzer spawn failed", err.Error())
		return
	}

	s.analyzerActive = true
	s.analyzerPID = pid
	s.analyzerCancel = cancel
	s.analyzerStop = func() {
		cleanup()
		cancel()
	}

	// Background watchdog: when ctx fires (timeout or explicit stop)
	// release the slot.
	go func() {
		<-ctx.Done()
		s.analyzerMu.Lock()
		if s.analyzerPID == pid {
			s.analyzerActive = false
			s.analyzerPID = 0
			s.analyzerCancel = nil
			s.analyzerStop = nil
		}
		s.analyzerMu.Unlock()
	}()

	writeJSON(w, http.StatusOK, analyzeStartResponse{
		Status:   "started",
		PID:      pid,
		TestName: req.TestName,
		Detail:   fmt.Sprintf("analyzer pid=%d findings=%s interval_ms=%d", pid, s.cfg.FindingsFile, interval),
	})
}

// handleFindingsStream tails s.cfg.FindingsFile via polling cursor and
// re-emits each line as a Server-Sent Event ("data: <line>\n\n").
//
// Polling design (vs inotify): inotify needs CGO/external bindings or
// golang.org/x/sys/unix wiring that bloats the dependency surface and
// breaks on darwin. A 200 ms tail loop reads only the appended bytes
// (file.Seek + io.ReadAll on a bounded chunk), is portable, and keeps
// the SSE feed at human-perceptible latency. The cursor is the byte
// offset; on truncation (analyzer rotated the log) the cursor resets.
func (s *Server) handleFindingsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError,
			"streaming unsupported", "ResponseWriter is not a Flusher")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // for any reverse proxy
	w.WriteHeader(http.StatusOK)

	// Initial event so the client sees the connection is live.
	fmt.Fprintf(w, "event: ready\ndata: {\"file\":%q}\n\n", s.cfg.FindingsFile)
	flusher.Flush()

	var cursor int64 // byte offset into the findings file
	pollInterval := 200 * time.Millisecond
	heartbeatInterval := 15 * time.Second
	lastHeartbeat := time.Now()
	leftover := ""

	for {
		select {
		case <-r.Context().Done():
			return
		default:
		}

		f, err := os.Open(s.cfg.FindingsFile)
		if err != nil {
			// File doesn't exist yet — heartbeat and wait.
			if time.Since(lastHeartbeat) >= heartbeatInterval {
				fmt.Fprint(w, ": heartbeat\n\n")
				flusher.Flush()
				lastHeartbeat = time.Now()
			}
			select {
			case <-r.Context().Done():
				return
			case <-time.After(pollInterval):
				continue
			}
		}

		fi, statErr := f.Stat()
		if statErr != nil {
			f.Close()
			select {
			case <-r.Context().Done():
				return
			case <-time.After(pollInterval):
				continue
			}
		}
		// Detect truncation/rotation (size shrank below cursor).
		if fi.Size() < cursor {
			cursor = 0
			leftover = ""
		}
		if fi.Size() > cursor {
			if _, err := f.Seek(cursor, io.SeekStart); err == nil {
				buf, _ := io.ReadAll(f)
				cursor += int64(len(buf))
				combined := leftover + string(buf)
				lines := strings.Split(combined, "\n")
				// Last fragment may be a partial line — hold for next tick.
				leftover = lines[len(lines)-1]
				for _, line := range lines[:len(lines)-1] {
					line = strings.TrimRight(line, "\r")
					if line == "" {
						continue
					}
					// SSE: every "data:" line is part of the same event.
					// The findings file is JSONL so one line == one event.
					fmt.Fprintf(w, "data: %s\n\n", line)
					flusher.Flush()
					lastHeartbeat = time.Now()
				}
			}
		}
		f.Close()

		if time.Since(lastHeartbeat) >= heartbeatInterval {
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
			lastHeartbeat = time.Now()
		}

		select {
		case <-r.Context().Done():
			return
		case <-time.After(pollInterval):
		}
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.HealthTimeout*2)
	defer cancel()

	whisperOK, whisperDetail, whisperErr,
		tesseractOK, tesseractDetail, tesseractErr := s.audioProbe(ctx,
		s.cfg.WhisperURL, s.cfg.TesseractURL, s.cfg.HealthTimeout)

	resp := healthResponse{
		WhisperOK:             whisperOK,
		TesseractOK:           tesseractOK,
		RecordingDirWritable:  isDirWritable(s.cfg.RecordingDir),
		AnalyzerBinaryPresent: isExecPresent(s.cfg.AnalyzerBinary),
		RecordScriptPresent:   isFilePresent(s.cfg.RecordScript),
		WhisperDetail:         whisperDetail,
		TesseractDetail:       tesseractDetail,
	}
	if whisperErr != nil {
		resp.WhisperError = whisperErr.Error()
	}
	if tesseractErr != nil {
		resp.TesseractError = tesseractErr.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	testName := r.URL.Query().Get("test_name")
	if err := validateTestName(testName); err != nil {
		writeError(w, http.StatusBadRequest, "invalid test_name", err.Error())
		return
	}

	// Path-traversal proof: testName is locked to [A-Za-z0-9_-]+, so
	// filepath.Join cannot escape s.cfg.TimelineDir. Belt-and-braces:
	// reject any resolved path that doesn't share the prefix.
	candidate := filepath.Join(s.cfg.TimelineDir, fmt.Sprintf("__action_timeline_%s.jsonl", testName))
	abs, err := filepath.Abs(candidate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "abs failed", err.Error())
		return
	}
	absDir, err := filepath.Abs(s.cfg.TimelineDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "abs dir failed", err.Error())
		return
	}
	if !strings.HasPrefix(abs, absDir+string(filepath.Separator)) && abs != absDir {
		writeError(w, http.StatusForbidden, "path traversal blocked",
			fmt.Sprintf("resolved %q escapes %q", abs, absDir))
		return
	}

	// Try the per-test path first; fall back to the shared default.
	path := candidate
	if !isFilePresent(path) {
		shared := filepath.Join(s.cfg.TimelineDir, "__action_timeline.jsonl")
		if isFilePresent(shared) {
			path = shared
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "timeline not found",
			fmt.Sprintf("%s: %v", path, err))
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("X-Timeline-Path", path)
	w.Header().Set("X-Timeline-Size", strconv.FormatInt(int64(len(data)), 10))
	_, _ = w.Write(data)
}

// ---------------------------------------------------------------------------
// JSON helpers
// ---------------------------------------------------------------------------

func decodeJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 64*1024))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg, detail string) {
	writeJSON(w, status, errorResponse{Error: msg, Detail: detail})
}

// ---------------------------------------------------------------------------
// Filesystem + process helpers
// ---------------------------------------------------------------------------

func isFilePresent(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isDirWritable(dir string) bool {
	if dir == "" {
		return false
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	probe, err := os.CreateTemp(dir, ".helixqa-bridge-probe-*")
	if err != nil {
		return false
	}
	probeName := probe.Name()
	_ = probe.Close()
	_ = os.Remove(probeName)
	return true
}

func isExecPresent(name string) bool {
	if name == "" {
		return false
	}
	if filepath.IsAbs(name) {
		info, err := os.Stat(name)
		if err != nil || info.IsDir() {
			return false
		}
		return info.Mode()&0o111 != 0
	}
	_, err := exec.LookPath(name)
	return err == nil
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func sanitizeForPath(name string) string {
	// Mirror the bash script's `tr -c 'A-Za-z0-9_' '_'` behaviour so
	// the response paths match what dual_display_record.sh actually
	// writes. validateTestName already enforces a stricter alphabet,
	// so this is only relevant for device serials with dots/colons.
	out := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '_' {
			out = append(out, c)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}

// pathDeviceSerial returns the serial as the bash script will use it
// in filenames. Empty serial in the request means the script will
// auto-pick the first ADB device — at response time we don't yet know
// which serial that is, so we leave the placeholder "*" so the operator
// knows to look up the real one from sync.json.
func pathDeviceSerial(serial string) string {
	if serial == "" {
		return "*"
	}
	return serial
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

// ---------------------------------------------------------------------------
// Audio probe
// ---------------------------------------------------------------------------

func defaultAudioProbe(ctx context.Context, whisperURL, tesseractURL string, perTimeout time.Duration) (
	bool, string, error, bool, string, error,
) {
	wctx, wcancel := context.WithTimeout(ctx, perTimeout)
	defer wcancel()
	wc := audio.NewWhisperClient(whisperURL)
	wh, werr := wc.Health(wctx)
	wOK := werr == nil
	wDetail := ""
	if wOK {
		wDetail = fmt.Sprintf("backend=%s default_model=%s loaded=%v compute=%s",
			wh.Backend, wh.DefaultModel, wh.LoadedModels, wh.ComputeType)
	}

	tctx, tcancel := context.WithTimeout(ctx, perTimeout)
	defer tcancel()
	tc := audio.NewTesseractClient(tesseractURL)
	th, terr := tc.Health(tctx)
	tOK := terr == nil
	tDetail := ""
	if tOK {
		tDetail = fmt.Sprintf("version=%s default_lang=%s default_psm=%d langs=%v",
			th.TesseractVersion, th.DefaultLang, th.DefaultPSM, th.Langs)
	}

	return wOK, wDetail, werr, tOK, tDetail, terr
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

func main() {
	os.Exit(realMain(os.Args[1:], os.Stdout, os.Stderr))
}

func realMain(argv []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("helixqa-bridge", flag.ContinueOnError)
	fs.SetOutput(errOut)

	listen := fs.String("listen", "127.0.0.1:7842", "Loopback bind address (MUST be 127.x.x.x or ::1)")
	recordScript := fs.String("record-script", "scripts/testing/dual_display_record.sh",
		"Path to dual_display_record.sh")
	analyzerBinary := fs.String("analyzer-binary", "recording-analyzer",
		"Path or $PATH name of the recording-analyzer binary")
	recordingDir := fs.String("recording-dir", "/tmp/__test_recording",
		"Directory dual_display_record.sh writes outputs to")
	findingsFile := fs.String("findings-file", "/tmp/__recording_findings.jsonl",
		"JSONL file the analyzer appends findings to (tailed by /v1/findings/stream)")
	timelineDir := fs.String("timeline-dir", "/tmp",
		"Directory action_timeline.sh writes per-test JSONL")
	whisperURL := fs.String("whisper-url", audio.DefaultWhisperBaseURL,
		"Whisper container base URL")
	tesseractURL := fs.String("tesseract-url", audio.DefaultTesseractBaseURL,
		"Tesseract container base URL")
	healthCheck := fs.Bool("health-check", false,
		"One-shot probe: exit 0 iff /v1/health would report all OK; do not start server")
	healthTimeout := fs.Duration("health-timeout", 5*time.Second,
		"Per-service health probe timeout")

	if err := fs.Parse(argv); err != nil {
		return 2
	}

	cfg := Config{
		ListenAddr:     *listen,
		RecordScript:   *recordScript,
		AnalyzerBinary: *analyzerBinary,
		RecordingDir:   *recordingDir,
		FindingsFile:   *findingsFile,
		TimelineDir:    *timelineDir,
		WhisperURL:     *whisperURL,
		TesseractURL:   *tesseractURL,
		HealthTimeout:  *healthTimeout,
	}
	srv := NewServer(cfg)

	if err := srv.cfg.validateLoopback(); err != nil {
		fmt.Fprintf(errOut, "helixqa-bridge: %v\n", err)
		return 2
	}

	if *healthCheck {
		ctx, cancel := context.WithTimeout(context.Background(), srv.cfg.HealthTimeout*4)
		defer cancel()
		whisperOK, whisperDetail, whisperErr,
			tesseractOK, tesseractDetail, tesseractErr := srv.audioProbe(ctx,
			srv.cfg.WhisperURL, srv.cfg.TesseractURL, srv.cfg.HealthTimeout)
		dirOK := isDirWritable(srv.cfg.RecordingDir)
		analyzerOK := isExecPresent(srv.cfg.AnalyzerBinary)
		scriptOK := isFilePresent(srv.cfg.RecordScript)

		fmt.Fprintf(out, "helixqa-bridge --health-check\n")
		fmt.Fprintf(out, "  whisper            : %s (%s)", okStr(whisperOK), whisperDetail)
		if whisperErr != nil {
			fmt.Fprintf(out, " err=%v", whisperErr)
		}
		fmt.Fprintln(out)
		fmt.Fprintf(out, "  tesseract          : %s (%s)", okStr(tesseractOK), tesseractDetail)
		if tesseractErr != nil {
			fmt.Fprintf(out, " err=%v", tesseractErr)
		}
		fmt.Fprintln(out)
		fmt.Fprintf(out, "  recording_dir      : %s (%s)\n", okStr(dirOK), srv.cfg.RecordingDir)
		fmt.Fprintf(out, "  analyzer_binary    : %s (%s)\n", okStr(analyzerOK), srv.cfg.AnalyzerBinary)
		fmt.Fprintf(out, "  record_script      : %s (%s)\n", okStr(scriptOK), srv.cfg.RecordScript)

		if whisperOK && tesseractOK && dirOK && analyzerOK && scriptOK {
			return 0
		}
		return 1
	}

	server := &http.Server{
		Addr:              srv.cfg.ListenAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	listener, err := net.Listen("tcp", srv.cfg.ListenAddr)
	if err != nil {
		fmt.Fprintf(errOut, "helixqa-bridge: listen %s: %v\n", srv.cfg.ListenAddr, err)
		return 1
	}
	fmt.Fprintf(out, "helixqa-bridge: listening on http://%s\n", listener.Addr())
	fmt.Fprintf(out, "  record-script   : %s\n", srv.cfg.RecordScript)
	fmt.Fprintf(out, "  analyzer-binary : %s\n", srv.cfg.AnalyzerBinary)
	fmt.Fprintf(out, "  recording-dir   : %s\n", srv.cfg.RecordingDir)
	fmt.Fprintf(out, "  findings-file   : %s\n", srv.cfg.FindingsFile)
	fmt.Fprintf(out, "  timeline-dir    : %s\n", srv.cfg.TimelineDir)

	// Graceful shutdown wiring.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(listener)
	}()

	select {
	case sig := <-sigCh:
		fmt.Fprintf(out, "helixqa-bridge: received %s, shutting down\n", sig)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// Cancel any active analyzer first so its goroutine exits.
		srv.analyzerMu.Lock()
		if srv.analyzerStop != nil {
			srv.analyzerStop()
		}
		srv.analyzerMu.Unlock()
		if err := server.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(errOut, "helixqa-bridge: shutdown error: %v\n", err)
			return 1
		}
		return 0
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(errOut, "helixqa-bridge: serve error: %v\n", err)
			return 1
		}
		return 0
	}
}

func okStr(ok bool) string {
	if ok {
		return "OK  "
	}
	return "FAIL"
}
