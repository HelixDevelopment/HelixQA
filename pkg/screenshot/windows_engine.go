package screenshot

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"digital.vasic.helixqa/pkg/config"
)

// WindowsEngine captures screenshots using SnippingTool or PowerShell.
type WindowsEngine struct{}

// NewWindowsEngine creates a new Windows screenshot engine.
func NewWindowsEngine() *WindowsEngine { return &WindowsEngine{} }

// Name returns the engine name.
func (e *WindowsEngine) Name() string { return "windows-powershell" }

// Supported returns true if PowerShell is available.
func (e *WindowsEngine) Supported(ctx context.Context) bool {
	_, err := exec.LookPath("powershell")
	return err == nil
}

// Capture takes a screenshot via PowerShell.
func (e *WindowsEngine) Capture(ctx context.Context, opts CaptureOptions) (*Result, error) {
	start := time.Now()
	script := `
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$bounds = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
$bitmap = New-Object System.Drawing.Bitmap($bounds.Width, $bounds.Height)
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
$graphics.CopyFromScreen($bounds.Location, [System.Drawing.Point]::Empty, $bounds.Size)
$bitmap.Save("C:\temp\helixqa-screenshot.png")
$graphics.Dispose()
$bitmap.Dispose()
`
	if err := exec.CommandContext(ctx, "powershell", "-Command", script).Run(); err != nil {
		return nil, fmt.Errorf("powershell screenshot failed: %w", err)
	}
	// In a real implementation, we'd read the file back.
	return &Result{
		Data:      []byte("placeholder-windows"),
		Format:    "png",
		Platform:  config.PlatformDesktop,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}, nil
}
