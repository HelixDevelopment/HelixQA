// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ticket

import (
	"strings"
	"testing"
	"time"
)

func TestCaptureRich_PopulatesEveryField(t *testing.T) {
	tk := &Ticket{ID: "HQA-0001"}
	origin := time.Now()
	failed := origin.Add(42 * time.Second)
	CaptureRich(tk, RichCaptureInput{
		SessionID:        "session-xyz",
		StepNumber:       7,
		FailedAt:         failed,
		VideoOriginAt:    origin,
		VideoPath:        "/video.mp4",
		BeforeScreenshot: "/evidence/before.png",
		AfterScreenshot:  "/evidence/after.png",
		LLMReasoning: []string{
			"step 1: click login",
			"step 2: wait for dashboard",
		},
		StackTrace:       "NullPointerException at Foo.bar",
		ReproductionBank: "banks/fixes-validation-browser.yaml",
	})
	if tk.SessionID != "session-xyz" {
		t.Errorf("SessionID wrong: %q", tk.SessionID)
	}
	if tk.StepNumber != 7 {
		t.Errorf("StepNumber wrong: %d", tk.StepNumber)
	}
	if tk.VideoTimestamp != "00:42" {
		t.Errorf("VideoTimestamp wrong: %q", tk.VideoTimestamp)
	}
	if len(tk.VideoRefs) != 1 || tk.VideoRefs[0].VideoPath != "/video.mp4" {
		t.Errorf("VideoRefs wrong: %+v", tk.VideoRefs)
	}
	if tk.BeforeScreenshotPath != "/evidence/before.png" {
		t.Errorf("BeforeScreenshotPath wrong")
	}
	if tk.AfterScreenshotPath != "/evidence/after.png" {
		t.Errorf("AfterScreenshotPath wrong")
	}
	if len(tk.LLMReasoningTranscript) != 2 {
		t.Errorf("LLMReasoningTranscript len = %d, want 2", len(tk.LLMReasoningTranscript))
	}
	if tk.StackTrace != "NullPointerException at Foo.bar" {
		t.Errorf("StackTrace not set")
	}
	if tk.ReproductionBank != "banks/fixes-validation-browser.yaml" {
		t.Errorf("ReproductionBank wrong")
	}
	// Screenshots slice should include both frames.
	if len(tk.Screenshots) < 2 {
		t.Errorf("Screenshots should accumulate before+after, got %d", len(tk.Screenshots))
	}
}

func TestCaptureRich_NilTicketIsSafe(t *testing.T) {
	got := CaptureRich(nil, RichCaptureInput{SessionID: "x"})
	if got != nil {
		t.Error("nil ticket should yield nil result")
	}
}

func TestCaptureRich_PartialInputLeavesOthersZero(t *testing.T) {
	tk := &Ticket{ID: "HQA-0002", SessionID: "pre-existing"}
	CaptureRich(tk, RichCaptureInput{
		BeforeScreenshot: "/a.png",
	})
	if tk.SessionID != "pre-existing" {
		t.Error("capture must not overwrite pre-existing SessionID when input is empty")
	}
	if tk.AfterScreenshotPath != "" {
		t.Error("after path should stay empty when not supplied")
	}
}

func TestRenderRichMarkdown_EmitsSectionsForEveryField(t *testing.T) {
	tk := &Ticket{
		ID:                     "HQA-0003",
		SessionID:              "s1",
		StepNumber:             3,
		VideoTimestamp:         "00:30",
		BeforeScreenshotPath:   "/b.png",
		AfterScreenshotPath:    "/a.png",
		LLMReasoningTranscript: []string{"one", "two"},
		ReproductionBank:       "banks/fixes-validation-agent-step.yaml",
		StackTrace:             "trace",
		VideoRefs:              []*VideoReference{{VideoPath: "/v.mp4"}},
	}
	md := RenderRichMarkdown(tk)
	for _, needle := range []string{
		"## Evidence",
		"Session ID:** `s1`",
		"Step number:** 3",
		"Video timestamp:** 00:30",
		"/v.mp4",
		"Before: `/b.png`",
		"After: `/a.png`",
		"Stack trace",
		"1. one",
		"2. two",
		"banks/fixes-validation-agent-step.yaml",
	} {
		if !strings.Contains(md, needle) {
			t.Errorf("rendered markdown missing %q\n---\n%s", needle, md)
		}
	}
}

func TestRenderRichMarkdown_EmptyWhenNoRichFields(t *testing.T) {
	tk := &Ticket{ID: "HQA-0004", Title: "legacy"}
	md := RenderRichMarkdown(tk)
	if md != "" {
		t.Errorf("expected empty markdown for legacy ticket, got %q", md)
	}
}

func TestVerifyMandatory_ReportsMissingFields(t *testing.T) {
	tk := &Ticket{ID: "HQA-0005"}
	missing := VerifyMandatory(tk)
	if len(missing) != len(MandatoryRichFields()) {
		t.Errorf("empty ticket should be missing every mandatory field, got %v", missing)
	}
	tk.SessionID = "s"
	tk.StepNumber = 1
	tk.BeforeScreenshotPath = "/b"
	tk.AfterScreenshotPath = "/a"
	tk.LLMReasoningTranscript = []string{"r"}
	if len(VerifyMandatory(tk)) != 0 {
		t.Errorf("fully populated ticket should have no missing fields, got %v", VerifyMandatory(tk))
	}
}

func TestVerifyMandatory_NilTicketListsAll(t *testing.T) {
	missing := VerifyMandatory(nil)
	if len(missing) != len(MandatoryRichFields()) {
		t.Errorf("nil ticket must list all mandatory fields")
	}
}

func TestFormatMMSS_HandlesNegativeAndLarge(t *testing.T) {
	if got := formatMMSS(-1 * time.Second); got != "00:00" {
		t.Errorf("negative delta should clamp to 00:00, got %q", got)
	}
	if got := formatMMSS(7*time.Minute + 5*time.Second); got != "07:05" {
		t.Errorf("formatMMSS for 7m5s = %q", got)
	}
}
