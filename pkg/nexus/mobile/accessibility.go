package mobile

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// AccessibilityNode is a platform-neutral rendering of a native UI node.
// Adapters produce these from Appium's pageSource XML so the AI navigator
// can reason over a uniform tree.
type AccessibilityNode struct {
	Ref         string              `json:"ref"`   // stable a1..aN reference
	Class       string              `json:"class"` // android.widget.Button, XCUIElementTypeButton, ...
	Text        string              `json:"text"`  // visible text content
	ContentDesc string              `json:"content_desc"`
	Label       string              `json:"label"` // iOS accessibility label
	Value       string              `json:"value"`
	Bounds      string              `json:"bounds"`
	Enabled     bool                `json:"enabled"`
	Visible     bool                `json:"visible"`
	Clickable   bool                `json:"clickable"`
	Children    []AccessibilityNode `json:"children,omitempty"`
}

// ParseAccessibilityTree parses the Appium pageSource into a tree. The
// function tolerates both Android UiAutomator XML and iOS XCUITest XML.
// Nodes that do not carry any interaction signal (text, content-desc,
// label, clickable) are still included but get a best-effort Ref so
// callers can count every element. Counter state is local to the call,
// making the function safe for concurrent use.
func ParseAccessibilityTree(source string) (*AccessibilityNode, error) {
	if strings.TrimSpace(source) == "" {
		return nil, fmt.Errorf("accessibility: empty source")
	}
	dec := xml.NewDecoder(strings.NewReader(source))
	var root *AccessibilityNode
	stack := []*AccessibilityNode{}
	counter := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			counter++
			node := &AccessibilityNode{
				Ref:   fmt.Sprintf("a%d", counter),
				Class: t.Name.Local,
			}
			for _, a := range t.Attr {
				applyAttr(node, strings.ToLower(a.Name.Local), a.Value)
			}
			// special-case: some Android dumps use <node class="..."> and
			// put the actual type in the class attribute.
			if t.Name.Local == "node" {
				if cls := findAttr(t.Attr, "class"); cls != "" {
					node.Class = cls
				}
			}
			if root == nil {
				root = node
			} else if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, *node)
				// Re-append a pointer-aware stack entry by re-indexing.
				node = &parent.Children[len(parent.Children)-1]
			}
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if root == nil {
		return nil, fmt.Errorf("accessibility: no root element found")
	}
	return root, nil
}

// Walk invokes visit for every node in depth-first pre-order. Returning
// an error from visit aborts the traversal.
func (n *AccessibilityNode) Walk(visit func(*AccessibilityNode) error) error {
	if n == nil {
		return nil
	}
	if err := visit(n); err != nil {
		return err
	}
	for i := range n.Children {
		if err := n.Children[i].Walk(visit); err != nil {
			return err
		}
	}
	return nil
}

// Find returns the first node satisfying pred, or nil.
func (n *AccessibilityNode) Find(pred func(*AccessibilityNode) bool) *AccessibilityNode {
	var hit *AccessibilityNode
	_ = n.Walk(func(cur *AccessibilityNode) error {
		if pred(cur) {
			hit = cur
			return fmt.Errorf("found") // abort
		}
		return nil
	})
	return hit
}

func applyAttr(n *AccessibilityNode, name, value string) {
	switch name {
	case "text":
		n.Text = value
	case "content-desc", "content_desc", "contentdesc":
		n.ContentDesc = value
	case "label", "accessibilitylabel":
		n.Label = value
	case "value":
		n.Value = value
	case "bounds", "frame":
		n.Bounds = value
	case "enabled":
		n.Enabled = value == "true"
	case "visible":
		n.Visible = value == "true"
	case "clickable":
		n.Clickable = value == "true"
	}
}

func findAttr(attrs []xml.Attr, name string) string {
	for _, a := range attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}
