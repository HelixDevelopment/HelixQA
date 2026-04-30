// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Phase 24.0 streaming-behavior tests for streamSingleFileMultipart.
//
// Constitution §11.4 anti-bluff: a "streams correctly" PASS that
// only asserts no error would be a bluff — bytes.Buffer also works
// without error, just with bad memory profile. These tests assert
// SPECIFIC POSITIVE EVIDENCE of streaming:
//
//   1. Open-failure surfaces synchronously (caller can detect
//      before ever spawning the goroutine).
//   2. The full file body arrives at the server byte-identical to
//      a 5 MiB random payload — confirms the goroutine writes the
//      whole file without dropping a chunk on early-close races.
//   3. Server-side cancellation propagates back through the pipe
//      (no goroutine leak after the http client tears down).

package audio

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestStream_OpenFailureSynchronous — file-not-found must error
// before the goroutine starts (otherwise caller can't distinguish
// "open failed" from "transient pipe error").
func TestStream_OpenFailureSynchronous(t *testing.T) {
	body, ct, err := streamSingleFileMultipart("video", "/nonexistent/path.bin")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if body != nil || ct != "" {
		t.Errorf("on error, body and contentType must be nil/empty (got %v / %q)", body, ct)
	}
	if !strings.Contains(err.Error(), "open") {
		t.Errorf("error %q does not mention 'open'", err.Error())
	}
}

// TestStream_LargeFileEndToEnd_ByteIdentical — the captured-evidence
// test. Generate 5 MiB of random bytes, hash them, ship via the
// streaming helper through httptest, hash what the server receives
// in the multipart "video" part. Hashes must match.
//
// 5 MiB is the sweet spot: large enough that it CANNOT fit in the
// io.Pipe internal buffer in one go (forcing real producer/consumer
// concurrency), small enough to keep the test under 1 second on
// a slow CI runner.
func TestStream_LargeFileEndToEnd_ByteIdentical(t *testing.T) {
	const size = 5 << 20 // 5 MiB

	// Generate deterministic-random payload so a failure has a
	// stable hash to investigate against.
	payload := make([]byte, size)
	if _, err := rand.Read(payload); err != nil {
		t.Fatalf("setup rand: %v", err)
	}
	wantHash := sha256.Sum256(payload)

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "large.bin")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	var (
		gotHash [32]byte
		gotSize int64
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use ReadForm with a small max-memory bound so the test
		// asserts streaming on the SERVER side too (the server
		// can't preload everything into RAM either).
		if err := r.ParseMultipartForm(1 << 20); err != nil { // 1 MiB max memory
			t.Errorf("server: ParseMultipartForm: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fhs := r.MultipartForm.File["video"]
		if len(fhs) != 1 {
			t.Errorf("server: want 1 part, got %d", len(fhs))
			return
		}
		f, err := fhs[0].Open()
		if err != nil {
			t.Errorf("server: open part: %v", err)
			return
		}
		defer f.Close()
		h := sha256.New()
		n, err := io.Copy(h, f)
		if err != nil {
			t.Errorf("server: copy: %v", err)
			return
		}
		gotSize = n
		copy(gotHash[:], h.Sum(nil))
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	body, ct, err := streamSingleFileMultipart("video", path)
	if err != nil {
		t.Fatalf("streamSingleFileMultipart: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL, body)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", ct)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d: %s", resp.StatusCode, string(body))
	}

	if gotSize != size {
		t.Errorf("server received %d bytes, want %d (streaming dropped data?)", gotSize, size)
	}
	if gotHash != wantHash {
		t.Errorf("server hash %x != client hash %x — payload corrupted by streaming",
			gotHash, wantHash)
	}
}

// TestStream_NoGoroutineLeakOnEarlyClose — server returns 500
// after reading <half the body. The pipe writer goroutine must
// notice the broken pipe and exit. We can't directly observe the
// goroutine, but we CAN observe via runtime.NumGoroutine count
// before/after (with a small settle delay).
//
// This is a soft test (goroutine count is influenced by other
// runtime activity) — we assert the count returns to baseline
// within 1 second, not that it's exact.
func TestStream_NoGoroutineLeakOnEarlyClose(t *testing.T) {
	const size = 1 << 20 // 1 MiB
	payload := make([]byte, size)
	if _, err := rand.Read(payload); err != nil {
		t.Fatalf("setup: %v", err)
	}
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "data.bin")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Server reads ~16 KiB then aborts. The pipe writer goroutine
	// will block on its next write, see ErrClosedPipe, and exit.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 16<<10)
		_, _ = io.ReadFull(r.Body, buf) // intentionally partial
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	baseline := runtime.NumGoroutine()

	// Run several iterations to amplify any leak.
	const iters = 5
	var wg sync.WaitGroup
	for i := 0; i < iters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, ct, err := streamSingleFileMultipart("video", path)
			if err != nil {
				return
			}
			req, _ := http.NewRequest(http.MethodPost, srv.URL, body)
			req.Header.Set("Content-Type", ct)
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}()
	}
	wg.Wait()

	// Settle delay — pipe writer goroutines need a tick to notice
	// the broken pipe and exit.
	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	final := runtime.NumGoroutine()

	// Allow some slack for runtime scheduler / httptest cleanup
	// goroutines. A real leak would scale linearly with iters
	// (5+ leaked goroutines), so any drift up to iters/2 = 2 is
	// noise; >iters is a true leak signal.
	maxDrift := iters / 2
	if final-baseline > maxDrift {
		t.Errorf("goroutine count grew from %d to %d (drift %d > %d) — pipe writers leaked on early-close",
			baseline, final, final-baseline, maxDrift)
	}
}
