package browser

import (
	"fmt"
	"sort"
	"strings"

	"digital.vasic.helixqa/pkg/nexus"
)

// SnapshotFromHTML parses raw HTML into a nexus.Snapshot with stable
// OpenClaw-style element references (e1, e2, ...). The parser is a small
// regex-free state machine so the package can compile without pulling in
// goquery at this layer; adapters may swap to goquery when fidelity
// matters more than dependency surface.
//
// The function returns an error only for empty input. Malformed HTML is
// handled leniently: unknown tags are skipped, attribute parsing tolerates
// missing quotes, and the returned Elements list is stable in document
// order so consumers can diff two Snapshots side-by-side.
func SnapshotFromHTML(html string, frame []byte) (*nexus.Snapshot, error) {
	if strings.TrimSpace(html) == "" {
		return nil, fmt.Errorf("snapshot: empty html")
	}
	elems := parseInteractive(html)
	return &nexus.Snapshot{
		Frame:    frame,
		Tree:     html,
		Elements: elems,
	}, nil
}

// parseInteractive extracts interactable elements from an HTML document.
// The algorithm is simple by design: it scans for opening tags of known
// interactive types, captures id/name/aria-label/role/title/text, and
// assigns e1, e2, ... references in document order.
//
// The detection set mirrors OpenClaw's browser-tool snapshot rules:
// button, a, input, select, textarea, plus ARIA roles button, link,
// textbox, checkbox, radio, menuitem, tab, switch, combobox, searchbox.
func parseInteractive(html string) []nexus.Element {
	interactiveTags := map[string]bool{
		"button": true, "a": true, "input": true, "select": true, "textarea": true,
	}
	interactiveRoles := map[string]bool{
		"button": true, "link": true, "textbox": true, "checkbox": true,
		"radio": true, "menuitem": true, "tab": true, "switch": true,
		"combobox": true, "searchbox": true,
	}

	var out []nexus.Element
	lower := strings.ToLower(html)

	i := 0
	for i < len(lower) {
		start := strings.IndexByte(lower[i:], '<')
		if start < 0 {
			break
		}
		i += start + 1
		if i >= len(lower) {
			break
		}
		if lower[i] == '/' || lower[i] == '!' {
			continue
		}
		end := findTagEnd(lower, i)
		if end < 0 {
			break
		}
		tag, attrs := parseTag(lower[i:end], html[i:end])
		i = end + 1
		if tag == "" {
			continue
		}

		role := attrs["role"]
		if !interactiveTags[tag] && !interactiveRoles[role] {
			continue
		}

		name := attrs["aria-label"]
		if name == "" {
			name = attrs["name"]
		}
		if name == "" {
			name = attrs["title"]
		}
		if name == "" {
			name = attrs["id"]
		}
		name = decodeHTMLEntities(name)
		if role == "" {
			role = defaultRoleForTag(tag, attrs)
		}

		// B8 fix: Selector built from raw attrs may still carry
		// HTML entities (&amp;, &quot;, etc.). Decode before
		// exposing so downstream JS / CSS lookups round-trip.
		el := nexus.Element{
			Ref:      nexus.ElementRef(fmt.Sprintf("e%d", len(out)+1)),
			Role:     role,
			Name:     name,
			Selector: decodeHTMLEntities(buildSelector(tag, attrs)),
		}
		out = append(out, el)
	}
	return out
}

func findTagEnd(s string, start int) int {
	for j := start; j < len(s); j++ {
		if s[j] == '>' {
			return j
		}
		if s[j] == '<' {
			return -1
		}
	}
	return -1
}

// parseTag returns the lowercase tag name and an attribute map. The
// attribute parser is permissive: quotes optional, multiple spaces,
// self-closing slash tolerated.
func parseTag(lowerBody, origBody string) (string, map[string]string) {
	body := strings.TrimSpace(lowerBody)
	if body == "" {
		return "", nil
	}
	// Extract tag name.
	end := 0
	for end < len(body) && body[end] != ' ' && body[end] != '\t' && body[end] != '\n' && body[end] != '/' {
		end++
	}
	tag := body[:end]
	rest := body[end:]
	// Use the original-case version for attribute values so captured
	// selectors round-trip.
	origRest := origBody[end:]
	return tag, parseAttrs(rest, origRest)
}

func parseAttrs(lowerRest, origRest string) map[string]string {
	attrs := map[string]string{}
	i := 0
	for i < len(lowerRest) {
		for i < len(lowerRest) && (lowerRest[i] == ' ' || lowerRest[i] == '\t' || lowerRest[i] == '\n' || lowerRest[i] == '/') {
			i++
		}
		if i >= len(lowerRest) {
			break
		}
		// Read name.
		start := i
		for i < len(lowerRest) && lowerRest[i] != '=' && lowerRest[i] != ' ' && lowerRest[i] != '\t' && lowerRest[i] != '\n' && lowerRest[i] != '/' {
			i++
		}
		name := lowerRest[start:i]
		if i >= len(lowerRest) || lowerRest[i] != '=' {
			if name != "" {
				attrs[name] = ""
			}
			continue
		}
		i++ // eat '='
		// Read value.
		quote := byte(0)
		if i < len(lowerRest) && (lowerRest[i] == '"' || lowerRest[i] == '\'') {
			quote = lowerRest[i]
			i++
		}
		valStart := i
		if quote != 0 {
			for i < len(lowerRest) && lowerRest[i] != quote {
				i++
			}
		} else {
			for i < len(lowerRest) && lowerRest[i] != ' ' && lowerRest[i] != '\t' && lowerRest[i] != '\n' && lowerRest[i] != '/' {
				i++
			}
		}
		val := origRest[valStart:i]
		if quote != 0 && i < len(lowerRest) {
			i++
		}
		attrs[name] = val
	}
	return attrs
}

func defaultRoleForTag(tag string, attrs map[string]string) string {
	switch tag {
	case "button":
		return "button"
	case "a":
		return "link"
	case "input":
		t := attrs["type"]
		switch t {
		case "button", "submit", "reset":
			return "button"
		case "checkbox":
			return "checkbox"
		case "radio":
			return "radio"
		case "search":
			return "searchbox"
		default:
			return "textbox"
		}
	case "select":
		return "combobox"
	case "textarea":
		return "textbox"
	}
	return "generic"
}

func buildSelector(tag string, attrs map[string]string) string {
	if id := attrs["id"]; id != "" {
		return "#" + id
	}
	if name := attrs["name"]; name != "" {
		return fmt.Sprintf("%s[name=\"%s\"]", tag, name)
	}
	if al := attrs["aria-label"]; al != "" {
		return fmt.Sprintf("%s[aria-label=\"%s\"]", tag, al)
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return tag
}
