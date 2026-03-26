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

// ── known component directories ───────────────────────────────────────────────

var knownComponents = []string{
	"catalog-api",
	"catalog-web",
	"catalogizer-android",
	"catalogizer-androidtv",
	"catalogizer-desktop",
	"installer-wizard",
}

// ── CodebaseMapper ────────────────────────────────────────────────────────────

// CodebaseMapper walks the project source tree and extracts API endpoints,
// web screens, and Android/TV composable destinations from source files.
type CodebaseMapper struct {
	root string
}

// NewCodebaseMapper returns a CodebaseMapper that works relative to root.
func NewCodebaseMapper(root string) *CodebaseMapper {
	return &CodebaseMapper{root: root}
}

// ExtractAPIEndpoints walks catalog-api/ for .go files and returns every
// Gin route found (GET/POST/PUT/DELETE/PATCH).
func (m *CodebaseMapper) ExtractAPIEndpoints() ([]APIEndpoint, error) {
	apiDir := filepath.Join(m.root, "catalog-api")

	if _, err := os.Stat(apiDir); os.IsNotExist(err) {
		return []APIEndpoint{}, nil
	}

	var endpoints []APIEndpoint

	err := filepath.WalkDir(apiDir, func(path string, d fs.DirEntry, walkErr error) error {
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

		for _, m := range ginRouteRegex.FindAllStringSubmatch(string(raw), -1) {
			endpoints = append(endpoints, APIEndpoint{
				Method:     strings.ToUpper(m[1]),
				Path:       m[2],
				SourceFile: path,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return endpoints, nil
}

// ExtractWebScreens walks catalog-web/ for .tsx, .jsx, and .ts files and
// returns every React <Route> element found, with Platform set to "web".
func (m *CodebaseMapper) ExtractWebScreens() ([]Screen, error) {
	webDir := filepath.Join(m.root, "catalog-web")

	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		return []Screen{}, nil
	}

	var screens []Screen

	err := filepath.WalkDir(webDir, func(path string, d fs.DirEntry, walkErr error) error {
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

	return screens, nil
}

// ExtractAndroidScreens walks catalogizer-android/ and catalogizer-androidtv/
// for .kt files and returns every Compose navigation destination found.
// Platform is set to "android" for phone and "androidtv" for TV.
func (m *CodebaseMapper) ExtractAndroidScreens() ([]Screen, error) {
	type dirPlatform struct {
		dir      string
		platform string
	}

	dirs := []dirPlatform{
		{filepath.Join(m.root, "catalogizer-android"), "android"},
		{filepath.Join(m.root, "catalogizer-androidtv"), "androidtv"},
	}

	var screens []Screen

	for _, dp := range dirs {
		if _, err := os.Stat(dp.dir); os.IsNotExist(err) {
			continue
		}

		err := filepath.WalkDir(dp.dir, func(path string, d fs.DirEntry, walkErr error) error {
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
					Platform:   dp.platform,
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

// DiscoverComponents checks which known project component directories exist
// under root and returns their names.
func (m *CodebaseMapper) DiscoverComponents() []string {
	var found []string
	for _, name := range knownComponents {
		dir := filepath.Join(m.root, name)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			found = append(found, name)
		}
	}
	return found
}
