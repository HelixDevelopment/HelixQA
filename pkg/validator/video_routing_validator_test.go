// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeVision returns a canned answer — lets us exercise every branch
// of Validate without touching a real LLM.
type fakeVision struct {
	answer string
	err    error
}

func (f fakeVision) Describe(ctx context.Context, image []byte, prompt string) (string, error) {
	return f.answer, f.err
}

func shot(data []byte) ScreenshotterFunc {
	return func(ctx context.Context) ([]byte, error) {
		return data, nil
	}
}

func shotErr(err error) ScreenshotterFunc {
	return func(ctx context.Context) ([]byte, error) {
		return nil, err
	}
}

func TestValidator_Validate_VisionPass(t *testing.T) {
	v := NewVideoRoutingValidator(shot(make([]byte, 4096, 4096)), fakeVision{answer: "YES — a movie scene is playing"})
	// Mostly-non-zero bytes: black ratio should be low.
	data := make([]byte, 4096)
	for i := range data {
		data[i] = 0x80
	}
	v.Shot = shot(data)
	r, err := v.Validate(context.Background(), Expectation{DisplayLabel: "TV"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if r.Status != "PASS" {
		t.Errorf("Status = %q, want PASS (reason=%q vision=%q)", r.Status, r.Reason, r.VisionSaid)
	}
	if !strings.Contains(r.Reason, "TV") {
		t.Errorf("Reason missing display label: %q", r.Reason)
	}
}

func TestValidator_Validate_BlackFrameFail(t *testing.T) {
	// All-zero bytes → black ratio ~1.0.
	data := make([]byte, 4096)
	v := NewVideoRoutingValidator(shot(data), fakeVision{answer: "YES"})
	r, err := v.Validate(context.Background(), Expectation{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Status != "FAIL" {
		t.Errorf("Status = %q, want FAIL", r.Status)
	}
	if !strings.Contains(r.Reason, "near-black") {
		t.Errorf("Reason should mention black-frame: %q", r.Reason)
	}
}

func TestValidator_Validate_VisionNo(t *testing.T) {
	data := make([]byte, 4096)
	for i := range data {
		data[i] = 0x80
	}
	v := NewVideoRoutingValidator(shot(data), fakeVision{answer: "No\nthe display shows only a spinner"})
	r, err := v.Validate(context.Background(), Expectation{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Status != "FAIL" {
		t.Errorf("Status = %q, want FAIL", r.Status)
	}
}

func TestValidator_Validate_VisionAmbiguous(t *testing.T) {
	data := make([]byte, 4096)
	for i := range data {
		data[i] = 0x80
	}
	v := NewVideoRoutingValidator(shot(data), fakeVision{answer: "maybe there's video? hard to tell"})
	r, _ := v.Validate(context.Background(), Expectation{})
	if r.Status != "WARN" {
		t.Errorf("Status = %q, want WARN", r.Status)
	}
}

func TestValidator_Validate_NoVisionClient(t *testing.T) {
	data := make([]byte, 4096)
	for i := range data {
		data[i] = 0x80
	}
	v := NewVideoRoutingValidator(shot(data), nil)
	r, _ := v.Validate(context.Background(), Expectation{})
	if r.Status != "INCONCLUSIVE" {
		t.Errorf("Status = %q, want INCONCLUSIVE", r.Status)
	}
}

func TestValidator_Validate_ScreenshotError(t *testing.T) {
	v := NewVideoRoutingValidator(shotErr(errors.New("adb lost")), fakeVision{answer: "YES"})
	r, _ := v.Validate(context.Background(), Expectation{})
	if r.Status != "FAIL" {
		t.Errorf("Status = %q, want FAIL on capture error", r.Status)
	}
	if !strings.Contains(r.Reason, "adb lost") {
		t.Errorf("Reason should include underlying error: %q", r.Reason)
	}
}

func TestValidator_Validate_VisionError(t *testing.T) {
	data := make([]byte, 4096)
	for i := range data {
		data[i] = 0x80
	}
	v := NewVideoRoutingValidator(shot(data), fakeVision{err: errors.New("quota exceeded")})
	r, _ := v.Validate(context.Background(), Expectation{})
	if r.Status != "INCONCLUSIVE" {
		t.Errorf("Status = %q, want INCONCLUSIVE", r.Status)
	}
}

func TestValidator_Validate_MissingShot(t *testing.T) {
	v := &VideoRoutingValidator{} // no shot, no vision
	_, err := v.Validate(context.Background(), Expectation{})
	if err == nil {
		t.Fatal("expected error when Shot is nil")
	}
}

func TestBlackRatioApprox(t *testing.T) {
	allZero := make([]byte, 4096)
	ratio, ok := blackRatioApprox(allZero)
	if !ok || ratio < 0.99 {
		t.Errorf("all-zero → ratio=%v ok=%v, want near-1", ratio, ok)
	}
	allFull := make([]byte, 4096)
	for i := range allFull {
		allFull[i] = 0xFF
	}
	ratio, ok = blackRatioApprox(allFull)
	if !ok || ratio > 0.01 {
		t.Errorf("all-FF → ratio=%v ok=%v, want near-0", ratio, ok)
	}
	tiny := make([]byte, 500)
	_, ok = blackRatioApprox(tiny)
	if ok {
		t.Errorf("tiny input should be refused")
	}
}
