// Package browser is the Nexus browser engine. It provides CDP-native Go
// automation on top of chromedp and go-rod with an OpenClaw-compatible
// snapshot and role-based reference system.
package browser

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
)

// ToAIFriendlyError converts low-level driver errors into short, LLM-
// readable strings so the navigator can plan the next step without a
// human in the loop.
//
// The conversion mirrors OpenClaw's `toAIFriendlyError()` pattern: it
// recognises the common failure modes (timeout, network down, selector
// miss, remote disconnect) and returns a single sentence the model can
// act on. Unknown errors pass through with a small wrapper so no data
// is silently lost.
func ToAIFriendlyError(err error) string {
	if err == nil {
		return ""
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "The page took too long to respond. The site might be slow or the element might not exist yet."
	case errors.Is(err, context.Canceled):
		return "The operation was cancelled before it finished. Retry or move on."
	}

	msg := err.Error()
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "Network timeout while talking to the browser. Try again."
	}

	switch {
	case strings.Contains(msg, "net::ERR_NAME_NOT_RESOLVED"):
		return "The URL's host could not be resolved. Check the URL for typos."
	case strings.Contains(msg, "net::ERR_CONNECTION_REFUSED"):
		return "The target server refused the connection. The service may be down."
	case strings.Contains(msg, "ERR_INTERNET_DISCONNECTED"):
		return "The browser is offline. Restore the connection and retry."
	case strings.Contains(msg, "no such element"), strings.Contains(msg, "element not found"):
		return "The element you asked for is not on the page right now. Take a fresh snapshot and try a different reference."
	case strings.Contains(msg, "not visible"), strings.Contains(msg, "not interactable"):
		return "The element exists but is not visible or interactable. Scroll or wait for it, then retry."
	case strings.Contains(msg, "websocket: close"), strings.Contains(msg, "connection closed"):
		return "The browser connection dropped. A fresh session is required."
	}

	return fmt.Sprintf("Browser automation error: %s", msg)
}
