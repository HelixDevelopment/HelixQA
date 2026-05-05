package fixtures

import "testing"

func TestClean(t *testing.T) {
	got := 1 + 1
	want := 2
	if got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}
