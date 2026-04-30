// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Streaming multipart helper — shared by whisper_client.go and
// tesseract_client.go. Replaces the in-memory bytes.Buffer approach
// from Phase 23.8/23.9 so that long QA videos no longer have to be
// held in RAM in the test runner.
//
// Phase 24.0 — memory-safety hardening. The previous in-memory
// buffer would OOM the test runner on multi-GB inputs (e.g. a
// 2-hour 1080p QA capture). By streaming via io.Pipe, the http
// client reads the multipart body as the goroutine writes it, so
// only a small kernel buffer (~64 KiB) sits in RAM at a time.

package audio

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
)

// streamSingleFileMultipart returns a streaming multipart body
// containing the file at path under the given form-field name,
// plus the Content-Type header value to set on the request.
//
// The body is backed by io.Pipe: a goroutine writes the multipart-
// encoded file into the pipe writer; the http client reads from
// the pipe reader. The file is NEVER loaded into memory in full —
// only ~64 KiB sits in the pipe buffer at any instant.
//
// If os.Open fails, the function returns synchronously with the
// error (no goroutine is spawned).
//
// Goroutine lifecycle: the goroutine closes the file and the pipe
// writer on exit. If the http client cancels mid-send (context
// timeout, connection drop), the pipe writer's next Write returns
// io.ErrClosedPipe; io.Copy returns that error; deferred close
// runs; goroutine exits. No leak.
func streamSingleFileMultipart(partName, path string) (io.Reader, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("audio: open %q: %w", path, err)
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	contentType := mw.FormDataContentType()

	go func() {
		defer f.Close()
		defer pw.Close()

		part, err := mw.CreateFormFile(partName, filepath.Base(path))
		if err != nil {
			pw.CloseWithError(fmt.Errorf("audio: create form file: %w", err))
			return
		}
		if _, err := io.Copy(part, f); err != nil {
			pw.CloseWithError(fmt.Errorf("audio: copy file into multipart: %w", err))
			return
		}
		if err := mw.Close(); err != nil {
			pw.CloseWithError(fmt.Errorf("audio: close multipart: %w", err))
			return
		}
	}()

	return pr, contentType, nil
}
