package mobile

import (
	"strings"
	"testing"
)

const androidHierarchy = `<?xml version="1.0"?>
<hierarchy>
  <node class="android.widget.FrameLayout">
    <node class="android.widget.Button" text="OK" content-desc="Confirm" clickable="true" enabled="true" visible="true" bounds="[0,0][100,50]"/>
    <node class="android.widget.EditText" text="" content-desc="Search" clickable="true"/>
  </node>
</hierarchy>`

const iosHierarchy = `<?xml version="1.0"?>
<XCUIElementTypeApplication>
  <XCUIElementTypeButton label="Save" value="" enabled="true" visible="true" bounds="{{0,0},{100,50}}"/>
  <XCUIElementTypeTextField label="Email" value="" enabled="true"/>
</XCUIElementTypeApplication>`

func TestParseAccessibilityTree_Android(t *testing.T) {
	root, err := ParseAccessibilityTree(androidHierarchy)
	if err != nil {
		t.Fatal(err)
	}
	if root == nil {
		t.Fatal("nil root")
	}
	// Root hierarchy node has one FrameLayout child containing two leaves.
	count := 0
	_ = root.Walk(func(n *AccessibilityNode) error { count++; return nil })
	if count < 4 {
		t.Errorf("walked %d nodes, want >=4", count)
	}
	btn := root.Find(func(n *AccessibilityNode) bool { return strings.Contains(n.Class, "Button") })
	if btn == nil {
		t.Fatal("did not find Button")
	}
	if btn.Text != "OK" || btn.ContentDesc != "Confirm" {
		t.Errorf("btn text/desc = %q / %q", btn.Text, btn.ContentDesc)
	}
	if !btn.Clickable || !btn.Enabled {
		t.Error("clickable/enabled not parsed")
	}
}

func TestParseAccessibilityTree_IOS(t *testing.T) {
	root, err := ParseAccessibilityTree(iosHierarchy)
	if err != nil {
		t.Fatal(err)
	}
	save := root.Find(func(n *AccessibilityNode) bool { return n.Class == "XCUIElementTypeButton" })
	if save == nil || save.Label != "Save" {
		t.Fatalf("Save button not found: %+v", save)
	}
}

func TestParseAccessibilityTree_Empty(t *testing.T) {
	if _, err := ParseAccessibilityTree("   "); err == nil {
		t.Fatal("empty input should error")
	}
}

func TestNode_WalkAndFind(t *testing.T) {
	root, _ := ParseAccessibilityTree(androidHierarchy)
	var refs []string
	_ = root.Walk(func(n *AccessibilityNode) error {
		refs = append(refs, n.Ref)
		return nil
	})
	if len(refs) < 3 {
		t.Errorf("walked refs = %v", refs)
	}
	// Find with always-true returns the root.
	got := root.Find(func(n *AccessibilityNode) bool { return true })
	if got != root {
		t.Error("find should return root")
	}
	// Find with always-false returns nil.
	if got := root.Find(func(_ *AccessibilityNode) bool { return false }); got != nil {
		t.Error("find should return nil when no match")
	}
}
