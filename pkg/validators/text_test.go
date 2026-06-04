package validators

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile writes content to a temp file with the given name and returns its path.
func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
	return p
}

func writeBytes(t *testing.T, name string, content []byte) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
	return p
}

func TestTextValidator_Supports(t *testing.T) {
	v := NewTextValidator()
	if !v.Supports("a.txt") {
		t.Error("expected Supports(.txt) = true")
	}
	if !v.Supports("a.log") {
		t.Error("expected Supports(.log) = true")
	}
	if v.Supports("a.png") {
		t.Error("expected Supports(.png) = false")
	}
	if v.Supports("a.json") {
		t.Error("expected Supports(.json) = false (json is its own type)")
	}
}

func TestTextValidator_MissingFile(t *testing.T) {
	v := NewTextValidator()
	res, err := v.Validate(filepath.Join(t.TempDir(), "does_not_exist.txt"))
	if err != nil {
		t.Fatalf("Validate returned err (should report via result): %v", err)
	}
	if res.IsValid {
		t.Error("expected IsValid=false for missing file")
	}
	if len(res.Errors) == 0 {
		t.Error("expected an error entry for missing file")
	}
}

func TestTextValidator_EmptyFileWarns(t *testing.T) {
	v := NewTextValidator()
	p := writeFile(t, "empty.txt", "")
	res, err := v.Validate(p)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsValid {
		t.Error("empty file should still be IsValid (warning, not error)")
	}
	foundEmptyWarn := false
	for _, w := range res.Warnings {
		if w == "file is empty (0 bytes)" {
			foundEmptyWarn = true
		}
	}
	if !foundEmptyWarn {
		t.Errorf("expected empty-file warning, got warnings=%v", res.Warnings)
	}
	if res.Metadata["size_bytes"].(int64) != 0 {
		t.Errorf("size_bytes = %v, want 0", res.Metadata["size_bytes"])
	}
}

func TestTextValidator_Log(t *testing.T) {
	content := "INFO starting up\n" +
		"WARNING low disk\n" +
		"ERROR connection refused\n" +
		"FATAL out of memory\n" +
		"panic: nil deref\n" +
		"info done\n"
	v := NewTextValidator()
	p := writeFile(t, "run.log", content)
	res, err := v.Validate(p)
	if err != nil {
		t.Fatal(err)
	}
	if res.TextSubtype != TextSubtypeLog {
		t.Fatalf("subtype = %q, want log", res.TextSubtype)
	}
	if got := res.Metadata["line_count"].(int); got != 6 {
		t.Errorf("line_count = %d, want 6", got)
	}
	// "error", "fatal", "panic" -> 3 error entries.
	if got := res.Metadata["error_count"].(int); got != 3 {
		t.Errorf("error_count = %d, want 3", got)
	}
	// "warning" line + "warn" substring of "warning" counts once per line; only
	// the WARNING line matches -> 1.
	if got := res.Metadata["warning_count"].(int); got != 1 {
		t.Errorf("warning_count = %d, want 1", got)
	}
}

func TestTextValidator_CSV(t *testing.T) {
	content := "id,name,score\n1,alice,10\n2,bob,20\n3,carol,30\n"
	v := NewTextValidator()
	p := writeFile(t, "data.csv", content)
	res, err := v.Validate(p)
	if err != nil {
		t.Fatal(err)
	}
	if res.TextSubtype != TextSubtypeCSV {
		t.Fatalf("subtype = %q, want csv", res.TextSubtype)
	}
	if got := res.Metadata["column_count"].(int); got != 3 {
		t.Errorf("column_count = %d, want 3", got)
	}
	if got := res.Metadata["row_count"].(int); got != 3 {
		t.Errorf("row_count = %d, want 3 (excludes header)", got)
	}
	cols, ok := res.Metadata["columns"].([]string)
	if !ok || len(cols) != 3 || cols[1] != "name" {
		t.Errorf("columns = %v, want [id name score]", res.Metadata["columns"])
	}
}

func TestTextValidator_XML(t *testing.T) {
	v := NewTextValidator()

	// Valid XML.
	good := writeFile(t, "ok.xml", `<root><child>value</child></root>`)
	res, err := v.Validate(good)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsValid {
		t.Errorf("valid XML marked invalid: %v", res.Errors)
	}
	if res.Metadata["valid_xml"] != true {
		t.Errorf("valid_xml metadata = %v, want true", res.Metadata["valid_xml"])
	}

	// Malformed XML -> IsValid false with a parse error.
	bad := writeFile(t, "bad.xml", `<root><child>value</root>`)
	res2, err := v.Validate(bad)
	if err != nil {
		t.Fatal(err)
	}
	if res2.IsValid {
		t.Error("malformed XML should be IsValid=false")
	}
	if len(res2.Errors) == 0 {
		t.Error("malformed XML should report an error")
	}
}

func TestTextValidator_Markdown(t *testing.T) {
	content := "# Title\n\n" +
		"Some text with a [link](http://example.com).\n\n" +
		"## Section\n\n" +
		"```go\nfmt.Println(\"hi\")\n```\n\n" +
		"More words here.\n"
	v := NewTextValidator()
	p := writeFile(t, "doc.md", content)
	res, err := v.Validate(p)
	if err != nil {
		t.Fatal(err)
	}
	if res.TextSubtype != TextSubtypeMarkdown {
		t.Fatalf("subtype = %q, want markdown", res.TextSubtype)
	}
	// "# " appears in "# Title" and "## Section" both contain "# " -> 2.
	if got := res.Metadata["heading_count"].(int); got != 2 {
		t.Errorf("heading_count = %d, want 2", got)
	}
	if got := res.Metadata["link_count"].(int); got != 1 {
		t.Errorf("link_count = %d, want 1", got)
	}
	// One fenced block = 2 backtick-fences / 2 = 1.
	if got := res.Metadata["code_block_count"].(int); got != 1 {
		t.Errorf("code_block_count = %d, want 1", got)
	}
	if res.Metadata["char_count"].(int) != len(content) {
		t.Errorf("char_count = %v, want %d", res.Metadata["char_count"], len(content))
	}
	if res.Metadata["word_count"].(int) == 0 {
		t.Error("word_count should be > 0")
	}
}

func TestTextValidator_PlainTextUTF8(t *testing.T) {
	v := NewTextValidator()

	// Valid UTF-8 (includes multibyte).
	good := writeFile(t, "notes.txt", "hello, world — café ☕\n")
	res, err := v.Validate(good)
	if err != nil {
		t.Fatal(err)
	}
	if res.TextSubtype != TextSubtypePlain {
		t.Fatalf("subtype = %q, want plain", res.TextSubtype)
	}
	if res.Metadata["valid_utf8"] != true {
		t.Errorf("valid_utf8 = %v, want true", res.Metadata["valid_utf8"])
	}

	// Invalid UTF-8 bytes -> valid_utf8 false + warning. 0xFF is never valid UTF-8.
	bad := writeBytes(t, "bin.txt", []byte{'o', 'k', 0xff, 0xfe, '\n'})
	res2, err := v.Validate(bad)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Metadata["valid_utf8"] != false {
		t.Errorf("valid_utf8 = %v, want false for invalid bytes", res2.Metadata["valid_utf8"])
	}
	foundWarn := false
	for _, w := range res2.Warnings {
		if len(w) >= 5 && w[:5] == "found" {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Errorf("expected invalid-utf8 warning, got %v", res2.Warnings)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}
	for _, tt := range tests {
		if got := formatBytes(tt.in); got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
