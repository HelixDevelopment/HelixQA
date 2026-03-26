// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package learning_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/learning"
)

// setupCodebaseProject creates a temporary directory tree with fake Go routes,
// React routes, and Kotlin composables used to test CodebaseMapper.
//
// Layout:
//
//	<root>/
//	  catalog-api/
//	    main.go                   (3 Gin routes)
//	    handlers/health.go        (1 Gin route)
//	  catalog-web/
//	    src/App.tsx                (2 React <Route …> elements)
//	    src/pages/Dashboard.tsx    (1 React <Route …> element + 1 more)
//	  catalogizer-android/
//	    app/src/main/MainNav.kt    (2 Kotlin composable destinations)
//	  catalogizer-androidtv/
//	    app/src/main/TvNav.kt      (1 Kotlin composable destination)
func setupCodebaseProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	// ── catalog-api ─────────────────────────────────────────────────────────
	apiDir := filepath.Join(root, "catalog-api")
	require.NoError(t, os.MkdirAll(apiDir, 0o755))
	write(t, filepath.Join(apiDir, "main.go"), `package main

import "github.com/gin-gonic/gin"

func main() {
	router := gin.Default()
	router.GET("/api/v1/health", healthHandler)
	router.POST("/api/v1/login", loginHandler)
	router.GET("/api/v1/media", mediaHandler)
}
`)

	handlersDir := filepath.Join(apiDir, "handlers")
	require.NoError(t, os.MkdirAll(handlersDir, 0o755))
	write(t, filepath.Join(handlersDir, "health.go"), `package handlers

func Register(router *gin.Engine) {
	router.GET("/api/v1/status", statusHandler)
}
`)

	// ── catalog-web ─────────────────────────────────────────────────────────
	webSrcDir := filepath.Join(root, "catalog-web", "src")
	require.NoError(t, os.MkdirAll(webSrcDir, 0o755))
	write(t, filepath.Join(webSrcDir, "App.tsx"), `import React from 'react';
import { Route } from 'react-router-dom';

function App() {
  return (
    <>
      <Route path="/dashboard" element={<DashboardPage />} />
      <Route path="/settings" element={<SettingsPage />} />
    </>
  );
}
`)

	pagesDir := filepath.Join(webSrcDir, "pages")
	require.NoError(t, os.MkdirAll(pagesDir, 0o755))
	write(t, filepath.Join(pagesDir, "Dashboard.tsx"), `import React from 'react';
import { Route } from 'react-router-dom';

export function DashboardRoutes() {
  return (
    <>
      <Route path="/browse" element={<BrowsePage />} />
      <Route path="/library" element={<LibraryPage />} />
    </>
  );
}
`)

	// ── catalogizer-android ─────────────────────────────────────────────────
	androidDir := filepath.Join(root, "catalogizer-android", "app", "src", "main")
	require.NoError(t, os.MkdirAll(androidDir, 0o755))
	write(t, filepath.Join(androidDir, "MainNav.kt"), `package com.example.catalogizer

@Composable
fun MainNavGraph(navController: NavHostController) {
    NavHost(navController = navController) {
        composable("home") {
            HomeScreen(navController = navController)
        }
        composable("detail") {
            DetailScreen(navController = navController)
        }
    }
}
`)

	// ── catalogizer-androidtv ───────────────────────────────────────────────
	androidTvDir := filepath.Join(root, "catalogizer-androidtv", "app", "src", "main")
	require.NoError(t, os.MkdirAll(androidTvDir, 0o755))
	write(t, filepath.Join(androidTvDir, "TvNav.kt"), `package com.example.catalogizer.tv

@Composable
fun TvNavGraph(navController: NavHostController) {
    NavHost(navController = navController) {
        composable("tv_home") {
            TvHomeScreen(navController = navController)
        }
    }
}
`)

	return root
}

// TestCodebaseMapper_ExtractAPIEndpoints verifies that at least 3 Gin routes
// are extracted from .go files under catalog-api/.
func TestCodebaseMapper_ExtractAPIEndpoints(t *testing.T) {
	root := setupCodebaseProject(t)
	m := learning.NewCodebaseMapper(root)

	endpoints, err := m.ExtractAPIEndpoints()
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(endpoints), 3,
		"should extract at least 3 Gin API endpoints")

	// All entries must have a non-empty method and path.
	for _, ep := range endpoints {
		assert.NotEmpty(t, ep.Method, "endpoint method should not be empty")
		assert.NotEmpty(t, ep.Path, "endpoint path should not be empty")
		assert.NotEmpty(t, ep.SourceFile, "endpoint source file should not be empty")
	}
}

// TestCodebaseMapper_ExtractWebScreens verifies that at least 4 React <Route>
// elements are extracted from .tsx/.jsx files under catalog-web/.
func TestCodebaseMapper_ExtractWebScreens(t *testing.T) {
	root := setupCodebaseProject(t)
	m := learning.NewCodebaseMapper(root)

	screens, err := m.ExtractWebScreens()
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(screens), 4,
		"should extract at least 4 React web screens")

	for _, s := range screens {
		assert.Equal(t, "web", s.Platform, "all web screens should have platform=web")
		assert.NotEmpty(t, s.Route, "web screen route should not be empty")
		assert.NotEmpty(t, s.Component, "web screen component should not be empty")
		assert.NotEmpty(t, s.SourceFile, "web screen source file should not be empty")
	}
}

// TestCodebaseMapper_ExtractAndroidScreens verifies that at least 2 Kotlin
// composable destinations are extracted across android + androidtv.
func TestCodebaseMapper_ExtractAndroidScreens(t *testing.T) {
	root := setupCodebaseProject(t)
	m := learning.NewCodebaseMapper(root)

	screens, err := m.ExtractAndroidScreens()
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(screens), 2,
		"should extract at least 2 Android/TV composable screens")

	for _, s := range screens {
		assert.NotEmpty(t, s.Platform, "android screen platform should not be empty")
		assert.NotEmpty(t, s.Route, "android screen route should not be empty")
		assert.NotEmpty(t, s.SourceFile, "android screen source file should not be empty")
	}
}

// TestCodebaseMapper_DiscoverComponents verifies that components matching
// known project directories are detected.
func TestCodebaseMapper_DiscoverComponents(t *testing.T) {
	root := setupCodebaseProject(t)
	m := learning.NewCodebaseMapper(root)

	components := m.DiscoverComponents()

	// The fake project has catalog-api, catalog-web, catalogizer-android,
	// catalogizer-androidtv — all four must be detected.
	assert.GreaterOrEqual(t, len(components), 4,
		"should detect at least 4 known components")

	found := make(map[string]bool)
	for _, c := range components {
		found[c] = true
	}
	assert.True(t, found["catalog-api"], "catalog-api should be discovered")
	assert.True(t, found["catalog-web"], "catalog-web should be discovered")
	assert.True(t, found["catalogizer-android"], "catalogizer-android should be discovered")
	assert.True(t, found["catalogizer-androidtv"], "catalogizer-androidtv should be discovered")
}

// TestCodebaseMapper_EmptyProject verifies that all extractors return empty
// (zero-length, no error) slices when the project has no relevant files.
func TestCodebaseMapper_EmptyProject(t *testing.T) {
	root := t.TempDir()
	m := learning.NewCodebaseMapper(root)

	endpoints, err := m.ExtractAPIEndpoints()
	require.NoError(t, err)
	assert.Len(t, endpoints, 0, "empty project: no API endpoints expected")

	webScreens, err := m.ExtractWebScreens()
	require.NoError(t, err)
	assert.Len(t, webScreens, 0, "empty project: no web screens expected")

	androidScreens, err := m.ExtractAndroidScreens()
	require.NoError(t, err)
	assert.Len(t, androidScreens, 0, "empty project: no android screens expected")

	components := m.DiscoverComponents()
	assert.Len(t, components, 0, "empty project: no components expected")
}
