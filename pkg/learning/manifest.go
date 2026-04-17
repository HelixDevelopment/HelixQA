// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package learning

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ComponentType classifies a project component so scanners know which
// source extensions, regex patterns, and platform labels to apply.
// HelixQA is a generic framework — these values are the ONLY thing
// callers need to supply to wire their project's layout into the
// learning pipeline. No project-specific directory names or package
// identifiers are baked into HelixQA itself.
type ComponentType string

const (
	// ComponentGoAPI is a Go HTTP service (Gin / Echo / chi / stdlib).
	ComponentGoAPI ComponentType = "go_api"
	// ComponentReactWeb is a React / Vite / Next front-end.
	ComponentReactWeb ComponentType = "react_web"
	// ComponentAndroid is an Android phone app (Compose / XML).
	ComponentAndroid ComponentType = "android"
	// ComponentAndroidTV is an Android TV (Leanback) app.
	ComponentAndroidTV ComponentType = "androidtv"
	// ComponentDesktop is a desktop shell (Tauri / Electron).
	ComponentDesktop ComponentType = "desktop"
	// ComponentInstaller is an installer wizard.
	ComponentInstaller ComponentType = "installer"
	// ComponentGeneric is a fallback for components that do not map to
	// any of the typed scanners above — they still appear in component
	// inventories but carry no automatic screen/endpoint extraction.
	ComponentGeneric ComponentType = "generic"
)

// Component describes a single logical unit of the project under test.
// Name is the display label; Dir is the directory relative to the
// project root; Type selects which scanner(s) apply.
type Component struct {
	Name string
	Dir  string
	Type ComponentType
}

// ProjectManifest is the project-specific input HelixQA consumes to
// learn about a codebase. It replaces hardcoded directory names and
// package identifiers so the library stays usable across any project.
//
// The zero value is valid and means "auto-discover". Callers that want
// deterministic behaviour supply their own Components + paths.
type ProjectManifest struct {
	// Components lists every logical unit the learner should scan. The
	// slice is empty by default — when empty, ProjectManifest.Resolve
	// performs a best-effort auto-discovery based on file markers
	// (go.mod, package.json with react, AndroidManifest.xml with
	// android.software.leanback, Cargo.toml + src-tauri, etc.).
	Components []Component

	// EnvFilePaths lists .env files to scan for credentials, relative
	// to the project root. When empty, ProjectManifest.Resolve falls
	// back to `{root}/.env` plus the .env inside each discovered
	// Go-API component directory.
	EnvFilePaths []string

	// DeepLinkScheme is the URI scheme the project uses for deep
	// linking (e.g. "myapp"). Empty = scheme-agnostic; downstream
	// planners skip deep-link-specific prompts.
	DeepLinkScheme string

	// PlaywrightNodeModulePaths lists directories to probe for a local
	// Playwright install when the navigator spins up a web runner.
	// Empty = look at every discovered React component's
	// `node_modules/` followed by the project-root `node_modules/`.
	PlaywrightNodeModulePaths []string
}

// Resolve returns a manifest guaranteed to have non-empty Components
// and EnvFilePaths — it fills in any missing fields from auto-
// discovery rooted at root. Callers that already populated every
// field get back an unchanged copy.
func (m ProjectManifest) Resolve(root string) ProjectManifest {
	out := ProjectManifest{
		Components:                append([]Component(nil), m.Components...),
		EnvFilePaths:              append([]string(nil), m.EnvFilePaths...),
		DeepLinkScheme:            m.DeepLinkScheme,
		PlaywrightNodeModulePaths: append([]string(nil), m.PlaywrightNodeModulePaths...),
	}
	if len(out.Components) == 0 {
		out.Components = autoDiscoverComponents(root)
	}
	if len(out.EnvFilePaths) == 0 {
		out.EnvFilePaths = defaultEnvFilePaths(root, out.Components)
	}
	return out
}

// ComponentsByType returns every component whose Type matches any of
// the provided types. The return slice is nil-safe for range.
func (m ProjectManifest) ComponentsByType(types ...ComponentType) []Component {
	var out []Component
	for _, c := range m.Components {
		for _, t := range types {
			if c.Type == t {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

// autoDiscoverComponents walks the immediate children of root and
// classifies each directory by cheap marker files.
func autoDiscoverComponents(root string) []Component {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []Component
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if c, ok := classifyComponentDir(e.Name(), dir); ok {
			out = append(out, c)
		}
	}
	return out
}

// classifyComponentDir applies marker-file heuristics to decide the
// ComponentType of dir. The returned bool is false when the directory
// does not look like a supported component.
func classifyComponentDir(name, dir string) (Component, bool) {
	isAndroidRoot := hasFile(dir, "AndroidManifest.xml") ||
		hasFile(filepath.Join(dir, "app", "src", "main"), "AndroidManifest.xml") ||
		hasFile(filepath.Join(dir, "src", "main"), "AndroidManifest.xml")
	hasGradle := hasFile(dir, "build.gradle") || hasFile(dir, "build.gradle.kts") ||
		hasFile(filepath.Join(dir, "app"), "build.gradle") ||
		hasFile(filepath.Join(dir, "app"), "build.gradle.kts")

	switch {
	case isAndroidRoot || hasGradle:
		if dirContainsAny(dir, []string{"leanback", "tvprovider", "android.software.leanback"}) {
			return Component{Name: name, Dir: dir, Type: ComponentAndroidTV}, true
		}
		return Component{Name: name, Dir: dir, Type: ComponentAndroid}, true
	case hasFile(dir, "Cargo.toml") && hasDir(dir, "src-tauri"):
		return Component{Name: name, Dir: dir, Type: ComponentDesktop}, true
	case hasFile(dir, "tauri.conf.json"):
		return Component{Name: name, Dir: dir, Type: ComponentDesktop}, true
	case hasFile(dir, "package.json"):
		pkg := readFileLower(filepath.Join(dir, "package.json"))
		switch {
		case strings.Contains(pkg, `"react"`):
			return Component{Name: name, Dir: dir, Type: ComponentReactWeb}, true
		case strings.Contains(pkg, `"@tauri-apps/api"`):
			return Component{Name: name, Dir: dir, Type: ComponentDesktop}, true
		default:
			return Component{Name: name, Dir: dir, Type: ComponentGeneric}, true
		}
	case hasFile(dir, "go.mod"):
		// Treat Go modules with a main.go referencing Gin/Echo/chi as
		// API components; others fall back to generic.
		if dirContainsAny(dir, []string{"gin-gonic/gin", "labstack/echo", "go-chi/chi"}) {
			return Component{Name: name, Dir: dir, Type: ComponentGoAPI}, true
		}
		return Component{Name: name, Dir: dir, Type: ComponentGeneric}, true
	}
	return Component{}, false
}

// defaultEnvFilePaths returns the conventional .env locations to scan
// when the manifest omits EnvFilePaths: the project root + each
// discovered Go-API component's local .env + a handful of well-known
// backend directory names.
func defaultEnvFilePaths(root string, components []Component) []string {
	paths := []string{filepath.Join(root, ".env")}
	seen := map[string]struct{}{paths[0]: {}}
	for _, c := range components {
		if c.Type != ComponentGoAPI {
			continue
		}
		p := filepath.Join(c.Dir, ".env")
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	for _, fallback := range []string{"backend", "server", "api"} {
		p := filepath.Join(root, fallback, ".env")
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	return paths
}

func hasFile(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, name))
	return err == nil && !info.IsDir()
}

func hasDir(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, name))
	return err == nil && info.IsDir()
}

func readFileLower(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.ToLower(string(data))
}

// dirContains grep-likes for needle inside any regular file under dir
// (bounded to 256 files + 2MiB per file so auto-discovery stays
// cheap). Returns true on the first hit.
func dirContains(dir, needle string) bool {
	return dirContainsAny(dir, []string{needle})
}

func dirContainsAny(dir string, needles []string) bool {
	if len(needles) == 0 {
		return false
	}
	const maxFiles = 256
	const maxBytes = 2 << 20
	count := 0
	var hit bool
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || hit || count >= maxFiles {
			if hit || count >= maxFiles {
				return filepath.SkipAll
			}
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		count++
		info, _ := d.Info()
		if info != nil && info.Size() > maxBytes {
			return nil
		}
		data, _ := os.ReadFile(path)
		lower := strings.ToLower(string(data))
		for _, n := range needles {
			if strings.Contains(lower, strings.ToLower(n)) {
				hit = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	return hit
}
