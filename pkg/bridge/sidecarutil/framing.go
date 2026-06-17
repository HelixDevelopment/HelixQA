// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package sidecarutil provides the stdio framing and health-probe helpers used
// by every HelixQA sidecar (helixqa-capture-linux, helixqa-capture-darwin,
// helixqa-capture-win, helixqa-input, helixqa-axtree-darwin, helixqa-frida,
// helixqa-omniparser, qa-vision-infer, qa-video-decode, qa-vulkan-compute,
// helixqa-langgraph, helixqa-browser-use).
//
// The contract is deliberately minimal so every sidecar — whether Go, C, C++,
// Swift, Rust, or Python — can implement it in a few lines:
//
//  1. Control channel: length-prefixed JSON over stdin/stdout.
//     4-byte big-endian uint32 length, then that many bytes of UTF-8 JSON.
//     A zero-length frame is a valid heartbeat (no payload).
//
//  2. Payload channel: file descriptors passed via SCM_RIGHTS on a Unix-domain
//     socket when large binary frames need to cross the process boundary
//     without being copied through pipes. Uses *net.UnixConn.WriteMsgUnix and
//     ReadMsgUnix — stdlib-only, no cgo.
//
//  3. Health probe: the sidecar MUST accept a single `--health` invocation
//     that prints "ok\n" and exits 0, or prints a diagnostic and exits 1.
//
// This package is CGO-free and has zero third-party dependencies — every
// transport primitive maps to a stdlib call. Keeping it that way preserves
// the CGO_ENABLED=0 invariant on the HelixQA Go host (see
// docs/openclawing/OpenClawing4.md §6.1).
package sidecarutil

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// MaxFrameBytes is the hard ceiling for a single JSON control frame. Sidecars
// that need to send larger payloads must use the payload channel (FD passing).
// Exceeding this limit on the reader side is a protocol error; the framer
// terminates the stream rather than attempting to resync.
const MaxFrameBytes = 16 * 1024 * 1024 // 16 MiB

// ErrFrameTooLarge is returned by ReadFrame when a peer advertises a length
// exceeding MaxFrameBytes.
var ErrFrameTooLarge = errors.New("sidecarutil: frame exceeds MaxFrameBytes")

// ErrShortRead is returned when the peer closes mid-frame.
var ErrShortRead = errors.New("sidecarutil: short read during frame body")

// ErrNoFD is returned by RecvFD when the message contained no SCM_RIGHTS fd.
var ErrNoFD = errors.New("sidecarutil: recvmsg returned no SCM_RIGHTS fd")

// WriteFrame serialises v as JSON and writes it length-prefixed to w.
// Safe for concurrent use only when guarded externally; typical sidecar
// stdout is single-writer so no internal locking is provided.
func WriteFrame(w io.Writer, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("sidecarutil: marshal: %w", err)
	}
	if len(body) > MaxFrameBytes {
		return fmt.Errorf("%w: len=%d", ErrFrameTooLarge, len(body))
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(body)))
	if _, err := w.Write(hdr[:]); err != nil {
		return fmt.Errorf("sidecarutil: write header: %w", err)
	}
	if len(body) == 0 {
		return nil
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("sidecarutil: write body: %w", err)
	}
	return nil
}

// WriteHeartbeat emits a zero-length frame. Peers use this to keep pipes
// alive without incurring JSON-parse cost.
func WriteHeartbeat(w io.Writer) error {
	var hdr [4]byte // uint32(0)
	_, err := w.Write(hdr[:])
	return err
}

// ReadFrame reads one frame from r and unmarshals its body into v. When the
// body is empty (heartbeat), v is left unchanged and nil is returned.
// Returns io.EOF iff the peer closed cleanly between frames.
func ReadFrame(r io.Reader, v any) error {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		if errors.Is(err, io.EOF) {
			return io.EOF
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return ErrShortRead
		}
		return fmt.Errorf("sidecarutil: read header: %w", err)
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n == 0 {
		return nil
	}
	if n > MaxFrameBytes {
		return fmt.Errorf("%w: len=%d", ErrFrameTooLarge, n)
	}
	body := make([]byte, int(n))
	if _, err := io.ReadFull(r, body); err != nil {
		return ErrShortRead
	}
	if v == nil {
		return nil
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("sidecarutil: unmarshal: %w", err)
	}
	return nil
}

// DrainReader reads frames from r until EOF or ctx is done, handing each
// frame's raw bytes to handle. Heartbeats (empty frames) are suppressed.
// Errors from handle terminate the loop and are returned.
func DrainReader(ctx context.Context, r io.Reader, handle func(raw json.RawMessage) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var raw json.RawMessage
		err := ReadFrame(r, &raw)
		switch {
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return err
		}
		if len(raw) == 0 {
			continue // heartbeat
		}
		if err := handle(raw); err != nil {
			return fmt.Errorf("sidecarutil: handler: %w", err)
		}
	}
}

// PassFD sends fd across the Unix-domain socket using SCM_RIGHTS. The caller
// retains ownership of fd; the peer gets a duplicate. A single-byte body is
// sent alongside the ancillary data so peers waiting on ReadMsgUnix receive a
// message boundary.
//
// Works on Linux and all current BSDs — no cgo, no platform-specific code.
func PassFD(conn *net.UnixConn, fd int) error {
	if conn == nil {
		return errors.New("sidecarutil: PassFD: nil conn")
	}
	if fd < 0 {
		return fmt.Errorf("sidecarutil: PassFD: invalid fd=%d", fd)
	}
	oob := syscall.UnixRights(fd)
	if _, _, err := conn.WriteMsgUnix([]byte{0}, oob, nil); err != nil {
		return fmt.Errorf("sidecarutil: WriteMsgUnix: %w", err)
	}
	return nil
}

// RecvFD blocks until one fd arrives on conn and returns it. Any extra fds in
// the same message are closed (we advertise one per message).
func RecvFD(conn *net.UnixConn) (int, error) {
	if conn == nil {
		return -1, errors.New("sidecarutil: RecvFD: nil conn")
	}
	body := make([]byte, 1)
	oob := make([]byte, syscall.CmsgSpace(4))
	_, oobn, _, _, err := conn.ReadMsgUnix(body, oob)
	if err != nil {
		return -1, fmt.Errorf("sidecarutil: ReadMsgUnix: %w", err)
	}
	scms, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return -1, fmt.Errorf("sidecarutil: ParseSocketControlMessage: %w", err)
	}
	for _, scm := range scms {
		if scm.Header.Level != syscall.SOL_SOCKET || scm.Header.Type != syscall.SCM_RIGHTS {
			continue
		}
		fds, err := syscall.ParseUnixRights(&scm)
		if err != nil {
			return -1, fmt.Errorf("sidecarutil: ParseUnixRights: %w", err)
		}
		if len(fds) == 0 {
			continue
		}
		for _, extra := range fds[1:] {
			_ = syscall.Close(extra)
		}
		return fds[0], nil
	}
	return -1, ErrNoFD
}

// HealthProbeOptions tunes the retry-with-backoff behaviour of HealthProbe.
// A zero value is invalid; use DefaultHealthProbeOptions and override fields.
//
// The distinction this struct encodes is the whole point of the hardening:
// a sidecar health probe that runs `<bin> --health` (a trivial fork+exec+print)
// must NOT report UNHEALTHY just because the host was briefly CPU-saturated and
// the kernel could not schedule the child before the per-attempt deadline. That
// is a transient, retryable condition. A genuinely sick sidecar — wrong stdout,
// non-zero exit, missing binary — is NOT retryable and is reported promptly.
type HealthProbeOptions struct {
	// PerAttemptTimeout bounds a single `<bin> --health` invocation. If the
	// child is still running when this fires, the attempt is treated as a
	// transient "exceeded budget under load" timeout and is eligible for retry.
	PerAttemptTimeout time.Duration

	// MaxAttempts is the total number of invocations (>=1). Only transient
	// deadline-exceeded attempts are retried; a genuine failure (non-zero exit,
	// wrong stdout, exec error such as missing binary) returns immediately.
	MaxAttempts int

	// BaseBackoff is the delay before the first retry; it doubles each retry
	// (exponential backoff) up to MaxBackoff. Backoff is skipped after the
	// final attempt. Zero means no inter-attempt delay.
	BaseBackoff time.Duration

	// MaxBackoff caps the exponential backoff. Zero means uncapped doubling.
	MaxBackoff time.Duration

	// TotalBudget, if non-zero, bounds the wall-clock time across all attempts
	// and backoffs. Once exceeded, no further attempt is started and the last
	// transient error is returned. This guarantees a hanging/always-timing-out
	// probe returns an error in bounded time rather than retrying forever.
	TotalBudget time.Duration
}

// DefaultHealthProbeOptions returns options tuned for a trivial health probe on
// a possibly-loaded host: 5 attempts, each bounded by timeout, 50ms→800ms
// exponential backoff, and a total budget of 4×timeout so a wedged probe still
// returns an error promptly instead of retrying forever.
func DefaultHealthProbeOptions(timeout time.Duration) HealthProbeOptions {
	return HealthProbeOptions{
		PerAttemptTimeout: timeout,
		MaxAttempts:       5,
		BaseBackoff:       50 * time.Millisecond,
		MaxBackoff:        800 * time.Millisecond,
		TotalBudget:       4 * timeout,
	}
}

// HealthProbe runs `<bin> --health`, waits up to timeout, and returns nil iff
// stdout is exactly "ok\n" and exit status is 0. All sidecars SHOULD implement
// this probe; the HelixQA orchestrator uses it at startup.
//
// HealthProbe is robust under transient host load: if a single invocation is
// SIGKILL'd because it exceeded its per-attempt budget while the host was
// CPU-saturated, the probe retries with exponential backoff (see
// DefaultHealthProbeOptions) rather than reporting a spurious UNHEALTHY that
// would trigger a false failover. A genuinely-unhealthy binary (non-zero exit,
// wrong stdout, missing binary) is NOT retried and is reported promptly.
func HealthProbe(ctx context.Context, bin string, timeout time.Duration) error {
	return HealthProbeWithOptions(ctx, bin, DefaultHealthProbeOptions(timeout))
}

// HealthProbeWithOptions is HealthProbe with explicit retry/backoff tuning. It
// is the extension point for callers that need different load tolerance than
// the default; HealthProbe is the back-compatible convenience wrapper.
func HealthProbeWithOptions(ctx context.Context, bin string, opts HealthProbeOptions) error {
	if bin == "" {
		return errors.New("sidecarutil: HealthProbe: empty binary name")
	}
	if opts.PerAttemptTimeout <= 0 {
		return fmt.Errorf("sidecarutil: HealthProbe(%s): non-positive PerAttemptTimeout", bin)
	}
	attempts := opts.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}

	var deadline time.Time
	if opts.TotalBudget > 0 {
		deadline = time.Now().Add(opts.TotalBudget)
	}

	backoff := opts.BaseBackoff
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		// Honour caller cancellation before spending another attempt.
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return fmt.Errorf("sidecarutil: HealthProbe(%s): %w (last transient: %v)", bin, err, lastErr)
			}
			return fmt.Errorf("sidecarutil: HealthProbe(%s): %w", bin, err)
		}

		err, transient := healthProbeOnce(ctx, bin, opts.PerAttemptTimeout)
		if err == nil {
			return nil
		}
		if !transient {
			// Genuine failure — fail fast, do not retry.
			return err
		}
		// Transient (deadline-exceeded under load). Record and consider retry.
		lastErr = err
		if attempt == attempts {
			break
		}

		// Compute the next backoff, clamped to MaxBackoff and to any remaining
		// total budget so we never sleep past the deadline.
		sleep := backoff
		if opts.MaxBackoff > 0 && sleep > opts.MaxBackoff {
			sleep = opts.MaxBackoff
		}
		if !deadline.IsZero() {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				break // total budget exhausted; stop retrying
			}
			// Need room for at least one more attempt; if a full backoff would
			// leave no time to run the next probe, stop now.
			if remaining <= sleep+opts.PerAttemptTimeout {
				break
			}
		}
		if sleep > 0 {
			t := time.NewTimer(sleep)
			select {
			case <-ctx.Done():
				t.Stop()
				return fmt.Errorf("sidecarutil: HealthProbe(%s): %w (last transient: %v)", bin, ctx.Err(), lastErr)
			case <-t.C:
			}
		}
		// Exponential growth for the next iteration.
		if backoff == 0 {
			backoff = opts.PerAttemptTimeout / 10
		} else {
			backoff *= 2
		}
	}

	if lastErr == nil {
		// Defensive: should not happen, but never return nil on a failed loop.
		lastErr = fmt.Errorf("sidecarutil: HealthProbe(%s): exhausted attempts", bin)
	}
	return fmt.Errorf("sidecarutil: HealthProbe(%s): unhealthy after %d transient attempt(s): %w", bin, attempts, lastErr)
}

// healthProbeOnce performs a single `<bin> --health` invocation. It returns
// (err, transient): transient is true ONLY when the failure is the per-attempt
// deadline killing the child (i.e. "exceeded budget under load"), which is the
// retryable case. All other failures — non-zero exit, wrong stdout, exec error
// (missing binary, permission denied) — return transient=false so the caller
// fails fast without retrying a genuinely-unhealthy sidecar.
func healthProbeOnce(ctx context.Context, bin string, perAttempt time.Duration) (err error, transient bool) {
	cctx, cancel := context.WithTimeout(ctx, perAttempt)
	defer cancel()
	cmd := exec.CommandContext(cctx, bin, "--health")
	// WaitDelay bounds how long cmd.Output() blocks reading stdout AFTER the
	// process is signalled. Without it, a probe that forks a grandchild (e.g.
	// `sleep` in a shell wrapper) leaves that grandchild holding the stdout
	// pipe open, so cmd.Output() blocks until the grandchild exits — defeating
	// the per-attempt deadline entirely. WaitDelay makes the per-attempt budget
	// a real wall-clock bound regardless of orphaned descendants.
	cmd.WaitDelay = 200 * time.Millisecond
	out, runErr := cmd.Output()
	if runErr != nil {
		// The per-attempt deadline fired and killed the child: retryable.
		// We check the per-attempt context (cctx), NOT the caller ctx, so that
		// caller cancellation is reported as a genuine (non-transient) error.
		if errors.Is(cctx.Err(), context.DeadlineExceeded) && !errors.Is(ctx.Err(), context.Canceled) {
			return fmt.Errorf("sidecarutil: HealthProbe(%s): attempt exceeded %s budget: %w", bin, perAttempt, runErr), true
		}
		// Any other error (genuine non-zero exit, missing binary, caller
		// cancellation) is NOT retryable.
		return fmt.Errorf("sidecarutil: HealthProbe(%s): %w", bin, runErr), false
	}
	if string(out) != "ok\n" {
		return fmt.Errorf("sidecarutil: HealthProbe(%s): unexpected stdout %q", bin, string(out)), false
	}
	return nil, false
}

// MultiHealth runs HealthProbe across many binaries in parallel, returning a
// map of binary→error. Binaries without errors (nil value) are healthy.
// The map is safe to inspect after return; no goroutines outlive this call.
//
// Each binary inherits HealthProbe's transient-load retry-with-backoff, so a
// briefly CPU-saturated host (e.g. when probing many sidecars at once) does not
// produce spurious UNHEALTHY verdicts; genuinely-unhealthy binaries still fail.
func MultiHealth(ctx context.Context, bins []string, timeout time.Duration) map[string]error {
	results := make(map[string]error, len(bins))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, b := range bins {
		wg.Add(1)
		go func(b string) {
			defer wg.Done()
			err := HealthProbe(ctx, b, timeout)
			mu.Lock()
			results[b] = err
			mu.Unlock()
		}(b)
	}
	wg.Wait()
	return results
}
