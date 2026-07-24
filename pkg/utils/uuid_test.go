package utils

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewUUID(t *testing.T) {
	id := NewUUID()
	if id == uuid.Nil {
		t.Fatal("NewUUID should not return nil UUID")
	}
}

func TestParse_Valid(t *testing.T) {
	id, err := Parse("550e8400-e29b-41d4-a716-446655440000")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if id.String() != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("UUID = %q, want %q", id.String(), "550e8400-e29b-41d4-a716-446655440000")
	}
}

func TestParse_Invalid(t *testing.T) {
	_, err := Parse("not-a-uuid")
	if err == nil {
		t.Fatal("Parse should fail for invalid UUID")
	}
}

func TestNewUUIDString(t *testing.T) {
	s := NewUUIDString()
	if len(s) != 36 {
		t.Errorf("NewUUIDString length = %d, want 36", len(s))
	}
}

func TestMustParse_Valid(t *testing.T) {
	id := MustParse("550e8400-e29b-41d4-a716-446655440000")
	if id.String() != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("MustParse returned %q, want %q", id.String(), "550e8400-e29b-41d4-a716-446655440000")
	}
}

func TestMustParse_PanicsOnInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustParse should panic on invalid UUID")
		}
	}()
	MustParse("not-a-uuid")
}
