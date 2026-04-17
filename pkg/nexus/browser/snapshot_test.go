package browser

import (
	"strings"
	"testing"
)

func TestSnapshotFromHTML_Empty(t *testing.T) {
	if _, err := SnapshotFromHTML("   ", nil); err == nil {
		t.Fatal("expected error on empty html")
	}
}

func TestSnapshotFromHTML_FindsInteractiveElements(t *testing.T) {
	html := `
		<html><body>
			<h1>Heading not interactive</h1>
			<button id="signin">Sign in</button>
			<a href="/home">Home</a>
			<input id="email" name="email" type="text" />
			<input type="checkbox" aria-label="Accept terms" />
			<select id="country"><option>US</option></select>
			<textarea id="bio"></textarea>
			<div role="button" aria-label="Close">x</div>
			<span>Nothing to click</span>
		</body></html>
	`
	snap, err := SnapshotFromHTML(html, []byte("fake-frame"))
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Elements) != 7 {
		t.Fatalf("expected 7 interactive elements, got %d: %+v", len(snap.Elements), snap.Elements)
	}
	// Refs must be sequential and stable in document order.
	for i, el := range snap.Elements {
		want := "e" + itoa(i+1)
		if string(el.Ref) != want {
			t.Errorf("element %d ref = %q, want %q", i, el.Ref, want)
		}
	}
}

func TestSnapshotFromHTML_RolesMapped(t *testing.T) {
	html := `<button id="b">ok</button><a href=x>link</a><input type=checkbox id=c><input type=text id=t><select id=s></select>`
	snap, err := SnapshotFromHTML(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	wants := []string{"button", "link", "checkbox", "textbox", "combobox"}
	if len(snap.Elements) != len(wants) {
		t.Fatalf("expected %d elements, got %d", len(wants), len(snap.Elements))
	}
	for i, want := range wants {
		if snap.Elements[i].Role != want {
			t.Errorf("element %d role = %q, want %q", i, snap.Elements[i].Role, want)
		}
	}
}

func TestSnapshotFromHTML_SelectorPrefersID(t *testing.T) {
	html := `<button id="primary">Click</button>`
	snap, _ := SnapshotFromHTML(html, nil)
	if len(snap.Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(snap.Elements))
	}
	if snap.Elements[0].Selector != "#primary" {
		t.Errorf("selector = %q, want #primary", snap.Elements[0].Selector)
	}
}

func TestSnapshotFromHTML_SelectorFallsBackToName(t *testing.T) {
	html := `<input name="email" type="text" />`
	snap, _ := SnapshotFromHTML(html, nil)
	if !strings.Contains(snap.Elements[0].Selector, `name="email"`) {
		t.Errorf("selector should contain name=email, got %q", snap.Elements[0].Selector)
	}
}

func TestSnapshotFromHTML_IgnoresMalformedHTML(t *testing.T) {
	html := `<button id="a">ok</button><div`
	snap, err := SnapshotFromHTML(html, nil)
	if err != nil {
		t.Fatalf("malformed html should still parse what it can, got err %v", err)
	}
	if len(snap.Elements) != 1 {
		t.Errorf("expected 1 element from malformed html, got %d", len(snap.Elements))
	}
}

func TestSnapshotFromHTML_AriaLabelBecomesName(t *testing.T) {
	html := `<div role="button" aria-label="Close dialog">x</div>`
	snap, _ := SnapshotFromHTML(html, nil)
	if len(snap.Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(snap.Elements))
	}
	if snap.Elements[0].Name != "Close dialog" {
		t.Errorf("name = %q, want 'Close dialog'", snap.Elements[0].Name)
	}
}

func TestParseAttrs_ToleratesMissingQuotes(t *testing.T) {
	lower := ` id=test role=button data-x=1 `
	orig := ` id=test role=button data-x=1 `
	attrs := parseAttrs(lower, orig)
	if attrs["id"] != "test" || attrs["role"] != "button" || attrs["data-x"] != "1" {
		t.Errorf("unquoted attrs not parsed: %+v", attrs)
	}
}

// itoa is a local minimal int-to-string helper so we avoid importing strconv
// in the test and keep compile speed tight.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	n := i
	buf := [12]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
