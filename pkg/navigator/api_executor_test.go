// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- API Executor Tests ---

func TestAPIExecutor_Type_SendsPOST(t *testing.T) {
	var gotMethod, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	runner := newMockRunner()
	exec := NewAPIExecutor(srv.URL, runner)
	err := exec.Type(context.Background(), `{"key":"value"}`)
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, `{"key":"value"}`, gotBody)
}

func TestAPIExecutor_Type_StoresResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":42}`))
	}))
	defer srv.Close()

	runner := newMockRunner()
	exec := NewAPIExecutor(srv.URL, runner)
	err := exec.Type(context.Background(), `{"name":"test"}`)
	require.NoError(t, err)

	assert.Equal(t, `{"id":42}`, string(exec.LastResponse()))
}

func TestAPIExecutor_KeyPress_SendsGET(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("endpoint response"))
	}))
	defer srv.Close()

	runner := newMockRunner()
	exec := NewAPIExecutor(srv.URL, runner)
	err := exec.KeyPress(context.Background(), "status")
	require.NoError(t, err)

	assert.Equal(t, http.MethodGet, gotMethod)
	assert.Equal(t, "/status", gotPath)
}

func TestAPIExecutor_KeyPress_StoresResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
	}))
	defer srv.Close()

	runner := newMockRunner()
	exec := NewAPIExecutor(srv.URL, runner)
	err := exec.KeyPress(context.Background(), "ping")
	require.NoError(t, err)

	assert.Equal(t, "pong", string(exec.LastResponse()))
}

func TestAPIExecutor_Screenshot_GetsHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	runner := newMockRunner()
	exec := NewAPIExecutor(srv.URL, runner)
	data, err := exec.Screenshot(context.Background())
	require.NoError(t, err)
	assert.Equal(t, `{"status":"ok"}`, string(data))
}

func TestAPIExecutor_Screenshot_Error(t *testing.T) {
	runner := newMockRunner()
	// Use an invalid URL so the HTTP call fails
	exec := NewAPIExecutor("http://127.0.0.1:1", runner)
	_, err := exec.Screenshot(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api screenshot")
}

func TestAPIExecutor_Click_ReturnsNil(t *testing.T) {
	runner := newMockRunner()
	exec := NewAPIExecutor("http://localhost:8080", runner)
	err := exec.Click(context.Background(), 0, 0)
	assert.NoError(t, err)
}

func TestAPIExecutor_Scroll_ReturnsNil(t *testing.T) {
	runner := newMockRunner()
	exec := NewAPIExecutor("http://localhost:8080", runner)
	err := exec.Scroll(context.Background(), "up", 3)
	assert.NoError(t, err)
}

func TestAPIExecutor_LongPress_ReturnsNil(t *testing.T) {
	runner := newMockRunner()
	exec := NewAPIExecutor("http://localhost:8080", runner)
	err := exec.LongPress(context.Background(), 0, 0)
	assert.NoError(t, err)
}

func TestAPIExecutor_Swipe_ReturnsNil(t *testing.T) {
	runner := newMockRunner()
	exec := NewAPIExecutor("http://localhost:8080", runner)
	err := exec.Swipe(context.Background(), 0, 0, 10, 10)
	assert.NoError(t, err)
}

func TestAPIExecutor_Back_ReturnsNil(t *testing.T) {
	runner := newMockRunner()
	exec := NewAPIExecutor("http://localhost:8080", runner)
	err := exec.Back(context.Background())
	assert.NoError(t, err)
}

func TestAPIExecutor_Home_ReturnsNil(t *testing.T) {
	runner := newMockRunner()
	exec := NewAPIExecutor("http://localhost:8080", runner)
	err := exec.Home(context.Background())
	assert.NoError(t, err)
}

func TestAPIExecutor_Type_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	runner := newMockRunner()
	exec := NewAPIExecutor(srv.URL, runner)
	// Server errors (5xx) are still valid HTTP responses — no transport error.
	err := exec.Type(context.Background(), `{}`)
	assert.NoError(t, err)
}

func TestAPIExecutor_Type_ContentTypeJSON(t *testing.T) {
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	runner := newMockRunner()
	exec := NewAPIExecutor(srv.URL, runner)
	err := exec.Type(context.Background(), `{"x":1}`)
	require.NoError(t, err)

	assert.True(t,
		strings.HasPrefix(gotContentType, "application/json"),
		"expected Content-Type application/json, got %q", gotContentType,
	)
}

func TestAPIExecutor_Interface(t *testing.T) {
	// Verify APIExecutor satisfies ActionExecutor.
	var _ ActionExecutor = &APIExecutor{}
}
