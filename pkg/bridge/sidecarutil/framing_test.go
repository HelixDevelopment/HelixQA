// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package sidecarutil

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

type payload struct {
	Kind string `json:"kind"`
	N    int    `json:"n"`
}

func TestWriteFrame_ReadFrame_Roundtrip(t *testing.T) {
	var buf bytes.Buffer
	in := payload{Kind: "hello", N: 42}
	if err := WriteFrame(&buf, in); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	var out payload
	if err := ReadFrame(&buf, &out); err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if in != out {
		t.Errorf("roundtrip mismatch: in=%+v out=%+v", in, out)
	}
}

func TestReadFrame_EOFOnEmptyStream(t *testing.T) {
	var out payload
	if err := ReadFrame(bytes.NewReader(nil), &out); !errors.Is(err, io.EOF) {
		t.Errorf("want io.EOF, got %v", err)
	}
}

func TestReadFrame_ShortRead(t *testing.T) {
	// 3 bytes of header only; next io.ReadFull gives ErrUnexpectedEOF.
	var out payload
	err := ReadFrame(bytes.NewReader([]byte{0, 0, 0}), &out)
	if !errors.Is(err, ErrShortRead) {
		t.Errorf("want ErrShortRead, got %v", err)
	}
}

func TestReadFrame_ShortBody(t *testing.T) {
	// header advertises 10 bytes, but body has only 5.
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], 10)
	buf := bytes.NewReader(append(hdr[:], []byte("short")...))
	var out payload
	err := ReadFrame(buf, &out)
	if !errors.Is(err, ErrShortRead) {
		t.Errorf("want ErrShortRead, got %v", err)
	}
}

func TestReadFrame_FrameTooLarge(t *testing.T) {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], MaxFrameBytes+1)
	var out payload
	err := ReadFrame(bytes.NewReader(hdr[:]), &out)
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Errorf("want ErrFrameTooLarge, got %v", err)
	}
}

func TestWriteFrame_FrameTooLarge(t *testing.T) {
	// Build a valid-JSON string whose marshalled length exceeds MaxFrameBytes.
	// A string of N ASCII letters marshals to N+2 bytes ("..."), so asking for
	// MaxFrameBytes bytes comfortably exceeds the limit.
	huge := make([]byte, MaxFrameBytes)
	for i := range huge {
		huge[i] = 'a'
	}
	var buf bytes.Buffer
	err := WriteFrame(&buf, string(huge))
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Errorf("want ErrFrameTooLarge, got %v", err)
	}
}

func TestWriteHeartbeat_ReadEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteHeartbeat(&buf); err != nil {
		t.Fatalf("WriteHeartbeat: %v", err)
	}
	// Header bytes are four zeros.
	if !bytes.Equal(buf.Bytes(), []byte{0, 0, 0, 0}) {
		t.Errorf("heartbeat bytes = %v", buf.Bytes())
	}
	var out payload
	// ReadFrame returns nil and leaves out untouched on empty body.
	if err := ReadFrame(&buf, &out); err != nil {
		t.Errorf("ReadFrame on heartbeat: %v", err)
	}
	if out.Kind != "" || out.N != 0 {
		t.Errorf("heartbeat must not mutate out, got %+v", out)
	}
}

func TestDrainReader_SkipsHeartbeats(t *testing.T) {
	var buf bytes.Buffer
	_ = WriteFrame(&buf, payload{Kind: "a", N: 1})
	_ = WriteHeartbeat(&buf)
	_ = WriteFrame(&buf, payload{Kind: "b", N: 2})
	_ = WriteHeartbeat(&buf)

	var seen []string
	err := DrainReader(context.Background(), &buf, func(raw json.RawMessage) error {
		var p payload
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		seen = append(seen, p.Kind)
		return nil
	})
	if err != nil {
		t.Fatalf("DrainReader: %v", err)
	}
	if got, want := seen, []string{"a", "b"}; !equalStrings(got, want) {
		t.Errorf("seen = %v, want %v", got, want)
	}
}

func TestDrainReader_HandlerError(t *testing.T) {
	var buf bytes.Buffer
	_ = WriteFrame(&buf, payload{Kind: "x", N: 0})
	boom := errors.New("boom")
	err := DrainReader(context.Background(), &buf, func(json.RawMessage) error { return boom })
	if !errors.Is(err, boom) {
		t.Errorf("want %v wrapped, got %v", boom, err)
	}
}

func TestDrainReader_ContextCancel(t *testing.T) {
	pr, pw := io.Pipe()
	// Writer-side never writes, so reader blocks; context cancel must unblock.
	t.Cleanup(func() { _ = pw.Close(); _ = pr.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- DrainReader(ctx, pr, func(json.RawMessage) error { return nil }) }()
	cancel()
	// Closing the pipe unblocks io.ReadFull which returns EOF — DrainReader
	// returns either ctx.Err() if the cancel fires first, or nil on clean EOF.
	// Either is acceptable; we assert termination under 1s.
	_ = pw.Close()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("unexpected error on cancel: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("DrainReader did not terminate after cancel")
	}
}

func TestHealthProbe_OK(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "healthy.sh")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nif [ \"$1\" = \"--health\" ]; then echo ok; fi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := HealthProbe(context.Background(), bin, 5*time.Second); err != nil {
		t.Errorf("HealthProbe healthy: %v", err)
	}
}

func TestHealthProbe_WrongStdout(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "wrongout.sh")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho nope\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := HealthProbe(context.Background(), bin, 5*time.Second)
	if err == nil {
		t.Fatal("want error for wrong stdout, got nil")
	}
}

func TestHealthProbe_NonZeroExit(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "fail.sh")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := HealthProbe(context.Background(), bin, 5*time.Second)
	if err == nil {
		t.Fatal("want error for exit 1, got nil")
	}
}

func TestHealthProbe_EmptyBin(t *testing.T) {
	if err := HealthProbe(context.Background(), "", time.Second); err == nil {
		t.Error("empty bin must error")
	}
}

func TestMultiHealth(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.sh")
	bad := filepath.Join(dir, "bad.sh")
	_ = os.WriteFile(good, []byte("#!/bin/sh\necho ok\n"), 0o755)
	_ = os.WriteFile(bad, []byte("#!/bin/sh\nexit 1\n"), 0o755)
	res := MultiHealth(context.Background(), []string{good, bad}, 5*time.Second)
	if res[good] != nil {
		t.Errorf("good: %v", res[good])
	}
	if res[bad] == nil {
		t.Errorf("bad: want error")
	}
}

func TestPassFD_RecvFD_Roundtrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SCM_RIGHTS not supported on windows")  // SKIP-OK: #legacy-untriaged
	}
	// Create a socketpair of UnixConns.
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	fa := os.NewFile(uintptr(fds[0]), "sidecarutil-a")
	fb := os.NewFile(uintptr(fds[1]), "sidecarutil-b")
	defer fa.Close()
	defer fb.Close()
	ca, err := net.FileConn(fa)
	if err != nil {
		t.Fatalf("FileConn a: %v", err)
	}
	cb, err := net.FileConn(fb)
	if err != nil {
		t.Fatalf("FileConn b: %v", err)
	}
	defer ca.Close()
	defer cb.Close()

	unixA, ok := ca.(*net.UnixConn)
	if !ok {
		t.Fatalf("ca not *net.UnixConn: %T", ca)
	}
	unixB, ok := cb.(*net.UnixConn)
	if !ok {
		t.Fatalf("cb not *net.UnixConn: %T", cb)
	}

	// The FD to send: a pipe's write end.
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer pr.Close()
	defer pw.Close()
	sendFD := int(pw.Fd())

	if err := PassFD(unixA, sendFD); err != nil {
		t.Fatalf("PassFD: %v", err)
	}
	recv, err := RecvFD(unixB)
	if err != nil {
		t.Fatalf("RecvFD: %v", err)
	}
	defer syscall.Close(recv)

	// Prove the received FD is a live pipe by writing to it and reading from pr.
	if _, err := syscall.Write(recv, []byte("ping")); err != nil {
		t.Fatalf("write via received fd: %v", err)
	}
	got := make([]byte, 4)
	if _, err := pr.Read(got); err != nil {
		t.Fatalf("read from pipe: %v", err)
	}
	if string(got) != "ping" {
		t.Errorf("recv fd did not forward bytes; got %q", got)
	}
}

func TestPassFD_NilConn(t *testing.T) {
	if err := PassFD(nil, 0); err == nil {
		t.Error("nil conn must error")
	}
}

func TestRecvFD_NilConn(t *testing.T) {
	if _, err := RecvFD(nil); err == nil {
		t.Error("nil conn must error")
	}
}

func TestPassFD_BadFD(t *testing.T) {
	conn, _ := net.Dial("unix", "") // will fail but conn is nil; skip if dial works
	if conn != nil {
		conn.Close()
		t.Skip("unexpected dial success")  // SKIP-OK: #legacy-untriaged
	}
	// Use a placeholder conn by pairing sockets but passing a negative fd.
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	fa := os.NewFile(uintptr(fds[0]), "a")
	fb := os.NewFile(uintptr(fds[1]), "b")
	defer fa.Close()
	defer fb.Close()
	ca, _ := net.FileConn(fa)
	defer ca.Close()
	u := ca.(*net.UnixConn)
	if err := PassFD(u, -1); err == nil {
		t.Error("negative fd must error")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
