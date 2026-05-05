package screenshot

import "time"

// Breakpoint defines a responsive viewport size.
type Breakpoint struct {
	Name   string
	Width  int
	Height int
}

// CaptureOptions parameterises a single screenshot request.
type CaptureOptions struct {
	Format                string
	Quality               int
	Width                 int
	Height                int
	FullPage              bool
	ResponsiveBreakpoints []Breakpoint
	DisplayID             string
	WindowID              string
	WaitForRender         time.Duration
	ValidateContent       bool
	MaxRetries            int
	DarkMode              bool
}

// DefaultBreakpoints are the standard responsive breakpoints.
var DefaultBreakpoints = []Breakpoint{
	{Name: "mobile", Width: 375, Height: 667},
	{Name: "tablet", Width: 768, Height: 1024},
	{Name: "desktop", Width: 1440, Height: 900},
}
