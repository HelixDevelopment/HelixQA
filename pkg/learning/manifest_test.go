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

// TestProjectManifest_DecoupledFromAnySpecificProject is the
// decoupling regression gate. It proves that HelixQA's learning
// pipeline works against a project whose directory names have
// absolutely nothing in common with Catalogizer — no "catalog-*",
// no "catalogizer-*", no installer-wizard — just generic names
// plus the standard marker files. If any hardcoded directory name
// slips back into the production code, this test will fail.
func TestProjectManifest_DecoupledFromAnySpecificProject(t *testing.T) {
	root := t.TempDir()

	// An API component called "alphaservice" — detected via go.mod + Gin.
	apiDir := filepath.Join(root, "alphaservice")
	require.NoError(t, os.MkdirAll(apiDir, 0o755))
	write(t, filepath.Join(apiDir, "go.mod"), `module alpha

go 1.21

require github.com/gin-gonic/gin v1.9.1
`)
	write(t, filepath.Join(apiDir, "main.go"), `package main

import "github.com/gin-gonic/gin"

func main() {
	router := gin.Default()
	router.GET("/alpha/health", nil)
	router.POST("/alpha/login", nil)
}
`)

	// A web component called "betafrontend".
	webDir := filepath.Join(root, "betafrontend", "src")
	require.NoError(t, os.MkdirAll(webDir, 0o755))
	write(t, filepath.Join(filepath.Dir(webDir), "package.json"),
		`{"name":"beta","dependencies":{"react":"18.3.0"}}`)
	write(t, filepath.Join(webDir, "App.tsx"), `import { Route } from 'react-router-dom';
export default () => (
  <Route path="/beta" element={<BetaHome />} />
);
`)

	// An Android-TV component called "gamma-tv".
	gammaRoot := filepath.Join(root, "gamma-tv")
	gammaSrc := filepath.Join(gammaRoot, "app", "src", "main")
	require.NoError(t, os.MkdirAll(gammaSrc, 0o755))
	write(t, filepath.Join(gammaSrc, "AndroidManifest.xml"),
		`<?xml version="1.0"?><manifest package="com.gamma.tv"><uses-feature android:name="android.software.leanback"/></manifest>`)
	write(t, filepath.Join(gammaSrc, "TvNav.kt"), `package com.gamma.tv
@Composable
fun TvNav() {
    NavHost {
        composable("gamma_home") { GammaHomeScreen(nav = nav) }
    }
}
`)

	kb, err := learning.BuildKnowledgeBase(root, nil)
	require.NoError(t, err)
	require.NotNil(t, kb)

	assert.Contains(t, kb.Components, "alphaservice",
		"generic Go-API component must be auto-discovered")
	assert.Contains(t, kb.Components, "betafrontend",
		"generic React-web component must be auto-discovered")
	assert.Contains(t, kb.Components, "gamma-tv",
		"generic Android-TV component must be auto-discovered")

	// No Catalogizer-specific names must leak through.
	for _, c := range kb.Components {
		assert.NotContains(t, c, "catalog-",
			"HelixQA must not invent catalog-* components from thin air")
		assert.NotContains(t, c, "catalogizer",
			"HelixQA must not invent catalogizer* components from thin air")
	}

	assert.GreaterOrEqual(t, len(kb.APIEndpoints), 2,
		"both /alpha/health and /alpha/login should be extracted")
	assert.GreaterOrEqual(t, len(kb.Screens), 2,
		"beta React route + gamma Kotlin composable should be extracted")
}

// TestProjectManifest_ExplicitComponents verifies the caller-supplied
// manifest path — the most reliable way for integrators to pin their
// project layout without relying on auto-discovery.
func TestProjectManifest_ExplicitComponents(t *testing.T) {
	root := t.TempDir()

	apiDir := filepath.Join(root, "custom-name-api")
	require.NoError(t, os.MkdirAll(apiDir, 0o755))
	// Deliberately omit go.mod so auto-discovery would skip it — we
	// want to prove that an explicit manifest still wires the scanner.
	write(t, filepath.Join(apiDir, "routes.go"), `package routes

func register(router *gin.Engine) {
	router.GET("/custom/ping", nil)
}
`)

	manifest := learning.ProjectManifest{
		Components: []learning.Component{
			{Name: "custom-name-api", Dir: apiDir, Type: learning.ComponentGoAPI},
		},
	}
	mapper := learning.NewCodebaseMapper(root, learning.WithManifest(manifest))

	endpoints, err := mapper.ExtractAPIEndpoints()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(endpoints), 1,
		"explicit manifest should drive the scanner even without marker files")
}
