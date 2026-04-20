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

// HealthProbe runs `<bin> --health`, waits up to timeout, and returns nil iff
// stdout is exactly "ok\n" and exit status is 0. All sidecars SHOULD implement
// this probe; the HelixQA orchestrator uses it at startup.
func HealthProbe(ctx context.Context, bin string, timeout time.Duration) error {
	if bin == "" {
		return errors.New("sidecarutil: HealthProbe: empty binary name")
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, bin, "--health")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("sidecarutil: HealthProbe(%s): %w", bin, err)
	}
	if string(out) != "ok\n" {
		return fmt.Errorf("sidecarutil: HealthProbe(%s): unexpected stdout %q", bin, string(out))
	}
	return nil
}

// MultiHealth runs HealthProbe across many binaries in parallel, returning a
// map of binary→error. Binaries without errors (nil value) are healthy.
// The map is safe to inspect after return; no goroutines outlive this call.
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
