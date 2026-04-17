package desktop

import (
	"reflect"
	"testing"
)

func TestParseFlags_ExtractsKeyValues(t *testing.T) {
	args := []string{"--name", "Save", "--process", "app", "--bonus", "x"}
	got := parseFlags(args)
	want := map[string]string{"--name": "Save", "--process": "app", "--bonus": "x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseFlags = %+v, want %+v", got, want)
	}
}

func TestParseFlags_IgnoresDangling(t *testing.T) {
	args := []string{"--only"}
	got := parseFlags(args)
	if len(got) != 0 {
		t.Errorf("dangling flag should be ignored, got %+v", got)
	}
}

func TestParseFlags_RespectsDashDashBoundary(t *testing.T) {
	args := []string{"foo", "--name", "bar"}
	got := parseFlags(args)
	if got["--name"] != "bar" {
		t.Errorf("--name not captured: %+v", got)
	}
}

func TestATSPIBackend_CloseNoOpWithoutConn(t *testing.T) {
	b := NewATSPIBackend()
	if err := b.Close(); err != nil {
		t.Fatalf("Close should be safe before connect, got %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("second Close should be safe, got %v", err)
	}
}
