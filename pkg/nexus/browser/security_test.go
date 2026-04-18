package browser

import (
	"context"
	"strings"
	"testing"

	"digital.vasic.helixqa/pkg/nexus"
)

// TestSecurity_SchemeBlocksAreExhaustive guards against a reviewer adding
// a new unsafe scheme to the list but forgetting to wire the block.
func TestSecurity_SchemeBlocksAreExhaustive(t *testing.T) {
	e, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{Engine: EngineChromedp})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	defer s.Close()

	blocked := []string{
		"file:///etc/passwd",
		"FILE:///etc/passwd", // case-insensitive
		"File:///etc/passwd", // mixed case
		"javascript:alert(1)",
		"JavaScript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"DATA:text/html,x",
		"vbscript:msgbox",
	}
	for _, bad := range blocked {
		if err := e.Navigate(context.Background(), s, bad); err == nil {
			t.Errorf("expected block for %q", bad)
		}
	}
}

// TestSecurity_AllowlistIsCaseSensitiveForHost keeps surprise out of the
// allowlist semantics. A misconfigured operator should get a clear "host
// not in allowlist" rather than a silent pass caused by case folding.
func TestSecurity_AllowlistIsCaseSensitiveForHost(t *testing.T) {
	e, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{
		Engine:       EngineChromedp,
		AllowedHosts: []string{"catalogizer.local"},
	})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	defer s.Close()

	if err := e.Navigate(context.Background(), s, "https://Catalogizer.local/"); err == nil {
		t.Fatal("mixed-case host must not match the canonical allowlist entry without an explicit toLower() in the allowlist builder")
	}
	if err := e.Navigate(context.Background(), s, "https://catalogizer.local/"); err != nil {
		t.Fatalf("canonical host should be allowed: %v", err)
	}
}

// TestSecurity_AllowlistAcceptsPortsAndPaths proves the allowlist only
// considers the hostname component, not the full URL.
func TestSecurity_AllowlistAcceptsPortsAndPaths(t *testing.T) {
	e, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{
		Engine:       EngineChromedp,
		AllowedHosts: []string{"catalogizer.local"},
	})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	defer s.Close()
	for _, ok := range []string{
		"https://catalogizer.local/",
		"https://catalogizer.local:8080/path",
		"http://user:pass@catalogizer.local/",
		"https://catalogizer.local/?q=1",
	} {
		if err := e.Navigate(context.Background(), s, ok); err != nil {
			t.Errorf("should be allowed: %q, got %v", ok, err)
		}
	}
}

// TestSecurity_MaxBodyBytesHasSensibleDefault guards against a change
// that silently drops the cap.
func TestSecurity_MaxBodyBytesHasSensibleDefault(t *testing.T) {
	e, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{Engine: EngineChromedp})
	if e.cfg.MaxBodyBytes < 1<<20 {
		t.Errorf("MaxBodyBytes default %d is suspiciously small", e.cfg.MaxBodyBytes)
	}
	if e.cfg.MaxBodyBytes > 256<<20 {
		t.Errorf("MaxBodyBytes default %d is larger than a reasonable per-response cap", e.cfg.MaxBodyBytes)
	}
}

// TestSecurity_NavigateEmptyURLMessage makes sure the denial message is
// recognisable so operators grepping logs can find the right events.
func TestSecurity_NavigateEmptyURLMessage(t *testing.T) {
	e, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{Engine: EngineChromedp})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	defer s.Close()

	err := e.Navigate(context.Background(), s, "")
	if err == nil {
		t.Fatal("expected error on empty URL")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error message should mention 'empty', got %q", err.Error())
	}
}
