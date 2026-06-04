package validators

import (
	"testing"
)

func TestDetectAssetType(t *testing.T) {
	tests := []struct {
		name string
		path string
		want AssetType
	}{
		// Video extensions
		{"mp4", "/tmp/clip.mp4", AssetTypeVideo},
		{"mov upper", "/tmp/CLIP.MOV", AssetTypeVideo},
		{"mkv", "rec.mkv", AssetTypeVideo},
		{"webm", "a/b/c.webm", AssetTypeVideo},
		{"m4v", "x.m4v", AssetTypeVideo},
		// Image extensions
		{"png", "shot.png", AssetTypeImage},
		{"jpg", "shot.jpg", AssetTypeImage},
		{"jpeg upper", "SHOT.JPEG", AssetTypeImage},
		{"gif", "anim.gif", AssetTypeImage},
		{"webp", "pic.webp", AssetTypeImage},
		{"ico", "fav.ico", AssetTypeImage},
		// JSON / YAML are their own types, NOT text
		{"json", "data.json", AssetTypeJSON},
		{"yaml", "conf.yaml", AssetTypeYAML},
		{"yml", "conf.yml", AssetTypeYAML},
		// Text family
		{"log", "run.log", AssetTypeText},
		{"txt", "notes.txt", AssetTypeText},
		{"md", "README.md", AssetTypeText},
		{"markdown", "x.markdown", AssetTypeText},
		{"csv", "rows.csv", AssetTypeText},
		{"xml", "doc.xml", AssetTypeText},
		{"html", "page.html", AssetTypeText},
		{"htm", "page.htm", AssetTypeText},
		// Unknown
		{"no ext", "Makefile", AssetTypeUnknown},
		{"binary ext", "lib.so", AssetTypeUnknown},
		{"empty", "", AssetTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectAssetType(tt.path)
			if got != tt.want {
				t.Fatalf("DetectAssetType(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestDetectTextSubtype(t *testing.T) {
	tests := []struct {
		name string
		path string
		want TextSubtype
	}{
		{"log ext", "server.log", TextSubtypeLog},
		{"log infix", "server.log.1", TextSubtypeLog}, // base contains ".log"
		{"markdown md", "doc.md", TextSubtypeMarkdown},
		{"markdown long", "doc.markdown", TextSubtypeMarkdown},
		{"csv", "data.csv", TextSubtypeCSV},
		{"xml", "feed.xml", TextSubtypeXML},
		{"report by name", "qa_report.txt", TextSubtypeReport},
		{"summary by name", "test_summary.txt", TextSubtypeReport},
		{"plain default", "notes.txt", TextSubtypePlain},
		{"plain unknown ext", "data.dat", TextSubtypePlain},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectTextSubtype(tt.path)
			if got != tt.want {
				t.Fatalf("DetectTextSubtype(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// TestDetectTextSubtype_LogBeatsReport documents the precedence: a file whose
// name contains both "report" and a ".log" extension classifies as log, because
// the log check runs first. A mutation reordering the switch would flip this.
func TestDetectTextSubtype_Precedence(t *testing.T) {
	if got := DetectTextSubtype("daily_report.log"); got != TextSubtypeLog {
		t.Fatalf("expected log to win over report, got %q", got)
	}
	if got := DetectTextSubtype("report.md"); got != TextSubtypeMarkdown {
		t.Fatalf("expected markdown to win over report, got %q", got)
	}
}

func TestValidationError_Error(t *testing.T) {
	e := ValidationError{Path: "/a/b.txt", Message: "boom", Code: "E123"}
	got := e.Error()
	want := "[E123] /a/b.txt: boom"
	if got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	// Confirm it satisfies the error interface.
	var _ error = e
}
