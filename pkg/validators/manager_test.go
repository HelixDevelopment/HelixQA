package validators

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManager_ValidateFile_DispatchesByType(t *testing.T) {
	m := NewManager()

	// Text file -> text validator -> AssetTypeText.
	txt := writeFile(t, "a.txt", "hello\n")
	res, err := m.ValidateFile(txt)
	if err != nil {
		t.Fatal(err)
	}
	if res.AssetType != AssetTypeText {
		t.Errorf("txt dispatched to %s, want text", res.AssetType)
	}

	// Image file -> image validator -> AssetTypeImage.
	img := writePNG(t, "b.png", 64, 64)
	res2, err := m.ValidateFile(img)
	if err != nil {
		t.Fatal(err)
	}
	if res2.AssetType != AssetTypeImage {
		t.Errorf("png dispatched to %s, want image", res2.AssetType)
	}
}

func TestManager_ValidateFile_UnknownType(t *testing.T) {
	m := NewManager()
	p := writeFile(t, "x.bin", "data")
	res, err := m.ValidateFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if res.AssetType != AssetTypeUnknown {
		t.Errorf("type = %s, want unknown", res.AssetType)
	}
	if !res.IsValid {
		t.Error("unknown type should still be IsValid=true (warning only)")
	}
	if len(res.Warnings) == 0 {
		t.Error("unknown type should carry a 'no validator' warning")
	}
}

func TestManager_GetResultsAndClear(t *testing.T) {
	m := NewManager()
	_, _ = m.ValidateFile(writeFile(t, "a.txt", "x"))
	_, _ = m.ValidateFile(writeFile(t, "b.txt", "y"))
	if got := len(m.GetResults()); got != 2 {
		t.Fatalf("GetResults len = %d, want 2", got)
	}
	m.Clear()
	if got := len(m.GetResults()); got != 0 {
		t.Fatalf("after Clear, GetResults len = %d, want 0", got)
	}
}

func TestManager_ValidateDirectory_NonRecursive(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "hi")
	mustWrite(t, filepath.Join(dir, "b.txt"), "yo")
	sub := filepath.Join(dir, "nested")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(sub, "c.txt"), "deep")

	m := NewManager()
	results, err := m.ValidateDirectory(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	// Non-recursive: only the two top-level files.
	if len(results) != 2 {
		t.Fatalf("non-recursive count = %d, want 2", len(results))
	}
}

func TestManager_ValidateDirectory_Recursive(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "hi")
	sub := filepath.Join(dir, "nested")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(sub, "c.txt"), "deep")
	mustWrite(t, filepath.Join(sub, "d.txt"), "deeper")

	m := NewManager()
	results, err := m.ValidateDirectory(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	// Recursive: top-level + nested = 3.
	if len(results) != 3 {
		t.Fatalf("recursive count = %d, want 3", len(results))
	}
}

func TestManager_ValidateQASession(t *testing.T) {
	session := t.TempDir()
	// Create the expected session layout with real assets.
	shots := filepath.Join(session, "screenshots")
	logs := filepath.Join(session, "logs")
	reports := filepath.Join(session, "reports")
	for _, d := range []string{shots, logs, reports} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Real PNG screenshot.
	var buf bytes.Buffer
	_ = png.Encode(&buf, makeImage(1280, 720))
	mustWriteBytes(t, filepath.Join(shots, "home.png"), buf.Bytes())
	// Log file.
	mustWrite(t, filepath.Join(logs, "run.log"), "INFO ok\nERROR boom\n")
	// Markdown report.
	mustWrite(t, filepath.Join(reports, "summary.md"), "# Report\nAll good.\n")

	m := NewManager()
	results, err := m.ValidateQASession(session)
	if err != nil {
		t.Fatal(err)
	}
	if len(results["screenshots"]) != 1 {
		t.Errorf("screenshots = %d, want 1", len(results["screenshots"]))
	}
	if len(results["logs"]) != 1 {
		t.Errorf("logs = %d, want 1", len(results["logs"]))
	}
	if len(results["reports"]) != 1 {
		t.Errorf("reports = %d, want 1", len(results["reports"]))
	}
	// videos dir absent -> key should not be present.
	if _, ok := results["videos"]; ok {
		t.Error("videos key should be absent when dir missing")
	}
	// Verify the screenshot really decoded to 1280x720.
	shot := results["screenshots"][0]
	if shot.Metadata["width"].(int) != 1280 || shot.Metadata["height"].(int) != 720 {
		t.Errorf("screenshot dims = %vx%v, want 1280x720", shot.Metadata["width"], shot.Metadata["height"])
	}
}

func TestManager_GetSummary(t *testing.T) {
	m := NewManager()
	// One valid text, one invalid image (corrupt), one unknown.
	_, _ = m.ValidateFile(writeFile(t, "a.txt", "ok"))
	_, _ = m.ValidateFile(writeBytes(t, "bad.png", []byte("not a png")))
	_, _ = m.ValidateFile(writeFile(t, "x.bin", "data"))

	s := m.GetSummary()
	if s.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3", s.TotalFiles)
	}
	if s.InvalidFiles != 1 {
		t.Errorf("InvalidFiles = %d, want 1 (corrupt png)", s.InvalidFiles)
	}
	if s.ValidFiles != 2 {
		t.Errorf("ValidFiles = %d, want 2", s.ValidFiles)
	}
	if s.ByType[AssetTypeText] != 1 {
		t.Errorf("ByType[text] = %d, want 1", s.ByType[AssetTypeText])
	}
	if s.ByType[AssetTypeImage] != 1 {
		t.Errorf("ByType[image] = %d, want 1", s.ByType[AssetTypeImage])
	}
	if s.ByType[AssetTypeUnknown] != 1 {
		t.Errorf("ByType[unknown] = %d, want 1", s.ByType[AssetTypeUnknown])
	}
	if !s.HasErrors() {
		t.Error("HasErrors should be true (1 invalid file)")
	}
	// String() should render the counts.
	str := s.String()
	if !strings.Contains(str, "Total files: 3") {
		t.Errorf("summary string missing total: %q", str)
	}
}

func TestValidationSummary_HasErrorsHasWarnings(t *testing.T) {
	s := &ValidationSummary{}
	if s.HasErrors() {
		t.Error("empty summary should have no errors")
	}
	if s.HasWarnings() {
		t.Error("empty summary should have no warnings")
	}
	s.Warnings = []string{"w"}
	if !s.HasWarnings() {
		t.Error("summary with warnings should report HasWarnings")
	}
	if s.HasErrors() {
		t.Error("warnings alone should not count as errors")
	}
	s.Errors = []string{"e"}
	if !s.HasErrors() {
		t.Error("summary with errors should report HasErrors")
	}
	// InvalidFiles alone also triggers HasErrors.
	s2 := &ValidationSummary{InvalidFiles: 1}
	if !s2.HasErrors() {
		t.Error("InvalidFiles>0 should make HasErrors true")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustWriteBytes(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
