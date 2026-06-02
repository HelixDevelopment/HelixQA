// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package visionnav

import (
	"context"
	"errors"
	"testing"

	"digital.vasic.helixqa/pkg/navigator"
)

// compile-time proof that the real navigator.ADBExecutor structurally
// satisfies the local DeviceExecutor contract. THIS is the load-bearing
// decoupling claim from session.go ("pkg/navigator's ADBExecutor
// satisfies it structurally") — if a future signature drift breaks it,
// this assertion fails to compile and the package won't build.
var _ DeviceExecutor = (*navigator.ADBExecutor)(nil)

// recordingExec is a unit-test fake DeviceExecutor that records calls.
// Mocks are permitted in unit tests only (§11.4.27).
type recordingExec struct {
	calls    []string
	failOn   string // verb to fail on, "" = never
	shotData []byte
	shotErr  error
}

func (r *recordingExec) Screenshot(_ context.Context) ([]byte, error) {
	r.calls = append(r.calls, "screenshot")
	if r.shotErr != nil {
		return nil, r.shotErr
	}
	return r.shotData, nil
}
func (r *recordingExec) Click(_ context.Context, x, y int) error {
	r.calls = append(r.calls, "click")
	return r.maybeFail("click")
}
func (r *recordingExec) Type(_ context.Context, text string) error {
	r.calls = append(r.calls, "type:"+text)
	return r.maybeFail("type")
}
func (r *recordingExec) KeyPress(_ context.Context, key string) error {
	r.calls = append(r.calls, "key:"+key)
	return r.maybeFail("key")
}
func (r *recordingExec) Back(_ context.Context) error {
	r.calls = append(r.calls, "back")
	return r.maybeFail("back")
}
func (r *recordingExec) Home(_ context.Context) error {
	r.calls = append(r.calls, "home")
	return r.maybeFail("home")
}
func (r *recordingExec) Shell(_ context.Context, cmd string) ([]byte, error) {
	r.calls = append(r.calls, "shell:"+cmd)
	return nil, r.maybeFail("shell")
}
func (r *recordingExec) maybeFail(verb string) error {
	if r.failOn == verb {
		return errors.New("forced failure on " + verb)
	}
	return nil
}

func TestNewADBActor_NilExec(t *testing.T) {
	if _, err := NewADBActor(nil); err == nil {
		t.Fatal("expected error for nil exec")
	}
}

func TestADBActor_Screenshot(t *testing.T) {
	exec := &recordingExec{shotData: []byte("PNGDATA")}
	a, err := NewADBActor(exec)
	if err != nil {
		t.Fatalf("NewADBActor: %v", err)
	}
	got, err := a.Screenshot(context.Background())
	if err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	if string(got) != "PNGDATA" {
		t.Fatalf("Screenshot data = %q, want PNGDATA", got)
	}
	if len(exec.calls) != 1 || exec.calls[0] != "screenshot" {
		t.Fatalf("calls = %v, want [screenshot]", exec.calls)
	}
}

func TestADBActor_Dispatch_Grammar(t *testing.T) {
	tests := []struct {
		name     string
		action   string
		wantCall string // "" = expect no executor call (observe-only)
		wantErr  bool
	}{
		{"empty observe-only", "", "", false},
		{"noop", "noop", "", false},
		{"wait", "wait", "", false},
		{"tap", "tap 100 200", "click", false},
		{"click alias", "click 5 6", "click", false},
		{"tap bad arity", "tap 100", "", true},
		{"tap non-int x", "tap a 2", "", true},
		{"tap non-int y", "tap 1 b", "", true},
		{"key", "key KEYCODE_ENTER", "key:KEYCODE_ENTER", false},
		{"key bad arity", "key", "", true},
		{"back", "back", "back", false},
		{"home", "home", "home", false},
		{"text multiword", "text hello world", "type:hello world", false},
		{"text empty", "text", "", true},
		{"shell", "shell am force-stop x", "shell:am force-stop x", false},
		{"launch", "launch monkey -p x 1", "shell:monkey -p x 1", false},
		{"shell empty", "shell", "", true},
		{"unknown verb", "frobnicate now", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &recordingExec{}
			a, _ := NewADBActor(exec)
			err := a.Dispatch(context.Background(), tt.action)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Dispatch(%q) expected error, got nil (calls=%v)", tt.action, exec.calls)
				}
				return
			}
			if err != nil {
				t.Fatalf("Dispatch(%q) unexpected error: %v", tt.action, err)
			}
			if tt.wantCall == "" {
				if len(exec.calls) != 0 {
					t.Fatalf("Dispatch(%q) expected no executor call, got %v", tt.action, exec.calls)
				}
				return
			}
			if len(exec.calls) != 1 || exec.calls[0] != tt.wantCall {
				t.Fatalf("Dispatch(%q) calls = %v, want [%s]", tt.action, exec.calls, tt.wantCall)
			}
		})
	}
}

func TestADBActor_Dispatch_PropagatesExecError(t *testing.T) {
	exec := &recordingExec{failOn: "click"}
	a, _ := NewADBActor(exec)
	if err := a.Dispatch(context.Background(), "tap 1 2"); err == nil {
		t.Fatal("expected executor error to propagate, got nil")
	}
}
