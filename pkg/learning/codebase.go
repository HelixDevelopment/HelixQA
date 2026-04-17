// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package learning provides types for capturing and querying structured
// knowledge about a project.
package learning

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ── regex patterns ────────────────────────────────────────────────────────────

// ginRouteRegex matches Gin router method calls such as:
//
//	router.GET("/api/v1/health", handler)
var ginRouteRegex = regexp.MustCompile(
	`(?i)router\.(GET|POST|PUT|DELETE|PATCH)\s*\(\s*"([^"]+)"`,
)

// reactRouteRegex matches JSX <Route> elements such as:
//
//	<Route path="/dashboard" element={<DashboardPage />} />
var reactRouteRegex = regexp.MustCompile(
	`<Route\s+path="([^"]+)"\s+element=\{<(\w+)`,
)

// composeNavRegex matches Kotlin Compose navigation destinations such as:
//
//	composable("home") {
//	    HomeScreen(…)
//	}
var composeNavRegex = regexp.MustCompile(
	`composable\(\s*"([^"]+)"\s*\)\s*\{[^}]*?(\w+Screen)\s*\(`,
)

// ── CodebaseMapper ────────────────────────────────────────────────────────────

// CodebaseMapper walks the project source tree and extracts API endpoints,
// web screens, and Android/TV composable destinations from source files.
// The mapper is driven by a ProjectManifest so HelixQA stays decoupled
// from any specific project layout — callers pass their own component
// list, or rely on the zero-value manifest's auto-discovery.
type CodebaseMapper struct {
	root     string
	manifest ProjectManifest
}

// MapperOption configures a CodebaseMapper at construction time.
type MapperOption func(*CodebaseMapper)

// WithManifest overrides the project manifest the mapper consults to
// decide which directories (and therefore which scanner passes) to
// apply. When omitted, the mapper resolves an empty manifest — which
// triggers on-the-fly auto-discovery via marker files.
func WithManifest(m ProjectManifest) MapperOption {
	return func(c *CodebaseMapper) { c.manifest = m }
}

// NewCodebaseMapper returns a CodebaseMapper that works relative to
// root. By default it auto-discovers component directories via marker
// files (go.mod, package.json with "react", AndroidManifest.xml, etc.)
// — supply WithManifest to pin a deterministic layout.
func NewCodebaseMapper(root string, opts ...MapperOption) *CodebaseMapper {
	m := &CodebaseMapper{root: root}
	for _, opt := range opts {
		opt(m)
	}
	m.manifest = m.manifest.Resolve(root)
	return m
}

// ExtractAPIEndpoints walks every Go-API component declared in the
// manifest and returns every Gin route found (GET/POST/PUT/DELETE/PATCH).
func (m *CodebaseMapper) ExtractAPIEndpoints() ([]APIEndpoint, error) {
	endpoints := []APIEndpoint{}
	for _, comp := range m.manifest.ComponentsByType(ComponentGoAPI) {
		if _, err := os.Stat(comp.Dir); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(comp.Dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".go") {
				return nil
			}

			raw, err := os.ReadFile(path)
			if err != nil {
				return nil // skip unreadable files
			}

			for _, match := range ginRouteRegex.FindAllStringSubmatch(string(raw), -1) {
				endpoints = append(endpoints, APIEndpoint{
					Method:     strings.ToUpper(match[1]),
					Path:       match[2],
					SourceFile: path,
				})
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return endpoints, nil
}

// ExtractWebScreens walks every React-web component declared in the
// manifest and returns every React <Route> element found, with
// Platform set to "web".
func (m *CodebaseMapper) ExtractWebScreens() ([]Screen, error) {
	screens := []Screen{}
	for _, comp := range m.manifest.ComponentsByType(ComponentReactWeb) {
		if _, err := os.Stat(comp.Dir); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(comp.Dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			name := strings.ToLower(d.Name())
			if !strings.HasSuffix(name, ".tsx") &&
				!strings.HasSuffix(name, ".jsx") &&
				!strings.HasSuffix(name, ".ts") {
				return nil
			}

			raw, err := os.ReadFile(path)
			if err != nil {
				return nil // skip unreadable files
			}

			for _, match := range reactRouteRegex.FindAllStringSubmatch(string(raw), -1) {
				route := match[1]
				component := match[2]
				screens = append(screens, Screen{
					Name:       component,
					Platform:   "web",
					Route:      route,
					Component:  component,
					SourceFile: path,
				})
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return screens, nil
}

// ExtractAndroidScreens walks every Android/Android-TV component
// declared in the manifest and returns every Compose navigation
// destination found. Platform is "android" for phone components and
// "androidtv" for TV components.
func (m *CodebaseMapper) ExtractAndroidScreens() ([]Screen, error) {
	screens := []Screen{}
	for _, comp := range m.manifest.ComponentsByType(ComponentAndroid, ComponentAndroidTV) {
		if _, err := os.Stat(comp.Dir); os.IsNotExist(err) {
			continue
		}
		platform := "android"
		if comp.Type == ComponentAndroidTV {
			platform = "androidtv"
		}
		err := filepath.WalkDir(comp.Dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(d.Name()), ".kt") {
				return nil
			}

			raw, err := os.ReadFile(path)
			if err != nil {
				return nil // skip unreadable files
			}

			for _, match := range composeNavRegex.FindAllStringSubmatch(string(raw), -1) {
				route := match[1]
				component := match[2]
				screens = append(screens, Screen{
					Name:       component,
					Platform:   platform,
					Route:      route,
					Component:  component,
					SourceFile: path,
				})
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return screens, nil
}

// DiscoverComponents returns the names of every Component in the
// resolved manifest whose directory exists on disk.
func (m *CodebaseMapper) DiscoverComponents() []string {
	var found []string
	for _, c := range m.manifest.Components {
		if info, err := os.Stat(c.Dir); err == nil && info.IsDir() {
			found = append(found, c.Name)
		}
	}
	return found
}

// Manifest returns the resolved manifest the mapper is using. Callers
// can forward this to the platform-feature detector or the credential
// reader so every stage shares the same component topology.
func (m *CodebaseMapper) Manifest() ProjectManifest { return m.manifest }
