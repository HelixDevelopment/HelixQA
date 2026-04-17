package browser

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestToAIFriendlyError_Nil(t *testing.T) {
	if s := ToAIFriendlyError(nil); s != "" {
		t.Errorf("nil error should return empty string, got %q", s)
	}
}

func TestToAIFriendlyError_KnownCategories(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		needle  string
	}{
		{"deadline", context.DeadlineExceeded, "took too long"},
		{"cancel", context.Canceled, "cancelled"},
		{"dns", errors.New("failed to navigate: net::ERR_NAME_NOT_RESOLVED"), "could not be resolved"},
		{"connection refused", errors.New("net::ERR_CONNECTION_REFUSED"), "refused the connection"},
		{"offline", errors.New("ERR_INTERNET_DISCONNECTED"), "offline"},
		{"missing element", errors.New("no such element: e12"), "not on the page"},
		{"not visible", errors.New("element is not visible"), "not visible"},
		{"ws close", errors.New("websocket: close 1006"), "connection dropped"},
	}
	for _, c := range cases {
		got := ToAIFriendlyError(c.err)
		if !strings.Contains(got, c.needle) {
			t.Errorf("%s: expected %q in %q", c.name, c.needle, got)
		}
	}
}

func TestToAIFriendlyError_Unknown(t *testing.T) {
	err := errors.New("some opaque driver error")
	got := ToAIFriendlyError(err)
	if !strings.Contains(got, "Browser automation error") {
		t.Errorf("unknown error should fall through to generic wrapper, got %q", got)
	}
	if !strings.Contains(got, "opaque") {
		t.Errorf("unknown error should preserve original message, got %q", got)
	}
}
