package observability

import "fmt"

// sprintfLite is a thin wrapper around fmt.Sprintf used by the metrics
// exposition formatter. It lives in its own file so the metrics path
// stays dependency-free at import analysis time.
func sprintfLite(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
