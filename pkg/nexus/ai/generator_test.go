package ai

import (
	"strings"
	"testing"
)

func TestValidateYAML_RejectsMissingSteps(t *testing.T) {
	err := validateYAML("id: NX-GEN\nname: ok\n")
	if err == nil || !strings.Contains(err.Error(), "steps") {
		t.Errorf("missing steps must fail, got %v", err)
	}
}

func TestValidateYAML_RejectsNonListSteps(t *testing.T) {
	err := validateYAML(`id: NX-GEN
name: ok
steps: "walk to the door"`)
	if err == nil || !strings.Contains(err.Error(), "must be a list") {
		t.Errorf("non-list steps must fail, got %v", err)
	}
}

func TestValidateYAML_RejectsEmptySteps(t *testing.T) {
	err := validateYAML(`id: NX-GEN
name: ok
steps: []`)
	if err == nil || !strings.Contains(err.Error(), "at least one") {
		t.Errorf("empty steps must fail, got %v", err)
	}
}

func TestValidateYAML_RejectsMissingStepFields(t *testing.T) {
	err := validateYAML(`id: NX-GEN
name: ok
steps:
  - name: click
`)
	if err == nil || !strings.Contains(err.Error(), "action") {
		t.Errorf("missing step field must fail, got %v", err)
	}
}

func TestValidateYAML_RejectsEmptyStepString(t *testing.T) {
	err := validateYAML(`id: NX-GEN
name: ok
steps:
  - name: click
    action: ""
    expected: "ok"
`)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("empty field must fail, got %v", err)
	}
}

func TestValidateYAML_RejectsNonStringFields(t *testing.T) {
	err := validateYAML(`id: NX-GEN
name: ok
steps:
  - name: click
    action: 42
    expected: "ok"
`)
	if err == nil || !strings.Contains(err.Error(), "string") {
		t.Errorf("non-string field must fail, got %v", err)
	}
}

func TestValidateYAML_RejectsMalformedRoot(t *testing.T) {
	err := validateYAML(`- a list at root`)
	if err == nil || !strings.Contains(err.Error(), "mapping") {
		t.Errorf("root list must fail, got %v", err)
	}
}

func TestValidateYAML_RejectsIDWithoutPrefix(t *testing.T) {
	err := validateYAML(`id: SOMETHING
name: ok
steps:
  - name: a
    action: b
    expected: c`)
	if err == nil || !strings.Contains(err.Error(), "NX-") {
		t.Errorf("wrong id prefix must fail, got %v", err)
	}
}

func TestValidateYAML_Happy(t *testing.T) {
	err := validateYAML(`id: NX-GEN-demo
name: demo
steps:
  - name: click
    action: navigate
    expected: ok
`)
	if err != nil {
		t.Fatalf("valid yaml rejected: %v", err)
	}
}

func TestStripFences(t *testing.T) {
	cases := map[string]string{
		"```yaml\nid: x\n```": "id: x",
		"```\nid: x\n```":     "id: x",
		"  id: x  ":           "id: x",
	}
	for in, want := range cases {
		if got := stripFences(in); got != want {
			t.Errorf("stripFences(%q) = %q, want %q", in, got, want)
		}
	}
}
