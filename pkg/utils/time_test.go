package utils

import (
	"testing"
	"time"
)

func TestNowUTC(t *testing.T) {
	now := NowUTC()
	if now.IsZero() {
		t.Fatal("NowUTC should not return zero time")
	}
	if now.Location() != time.UTC {
		t.Fatal("NowUTC should return UTC time")
	}
}

func TestDaysFromNow(t *testing.T) {
	d := DaysFromNow(7)
	expected := time.Now().UTC().AddDate(0, 0, 7)
	if d.Day() != expected.Day() {
		t.Errorf("DaysFromNow(7).Day() = %d, want %d", d.Day(), expected.Day())
	}
}

func TestHoursFromNow(t *testing.T) {
	h := HoursFromNow(2)
	expected := time.Now().UTC().Add(2 * time.Hour)
	if h.Hour() != expected.Hour() {
		t.Errorf("HoursFromNow(2).Hour() = %d, want %d", h.Hour(), expected.Hour())
	}
}

func TestMinutesFromNow(t *testing.T) {
	m := MinutesFromNow(30)
	expected := time.Now().UTC().Add(30 * time.Minute)
	if m.Minute() != expected.Minute() {
		t.Errorf("MinutesFromNow(30).Minute() = %d, want %d", m.Minute(), expected.Minute())
	}
}

func TestFormatRFC3339(t *testing.T) {
	ts := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	got := FormatRFC3339(ts)
	want := "2024-01-15T10:30:00Z"
	if got != want {
		t.Errorf("FormatRFC3339() = %q, want %q", got, want)
	}
}

func TestIsExpired(t *testing.T) {
	past := time.Now().UTC().Add(-1 * time.Hour)
	future := time.Now().UTC().Add(1 * time.Hour)

	if !IsExpired(past) {
		t.Error("IsExpired(past) should be true")
	}
	if IsExpired(future) {
		t.Error("IsExpired(future) should be false")
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"2024-01-15T10:30:00Z", false},
		{"2024-01-15T10:30:00+02:00", false},
		{"not-a-time", true},
		{"2024-13-01T00:00:00Z", true},
		{"", true},
	}
	for _, tt := range tests {
		got, err := ParseTime(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseTime(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got.IsZero() {
			t.Errorf("ParseTime(%q) returned zero time", tt.input)
		}
	}
}
