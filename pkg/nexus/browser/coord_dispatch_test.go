// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package browser

import (
	"context"
	"strings"
	"sync"
	"testing"

	"digital.vasic.helixqa/pkg/nexus"
)

// Phase-6 coord dispatch — regression suite for the browser
// Engine's coord_click / coord_type / coord_scroll routing.

// coordHandle is a SessionHandle that also implements CoordCapable.
// Records every call so tests assert on the exact arguments.
type coordHandle struct {
	mu       sync.Mutex
	clicks   []clickArgs
	types    []typeArgs
	scrolls  []scrollArgs
}

type clickArgs struct{ X, Y int }
type typeArgs struct {
	X, Y int
	Text string
}
type scrollArgs struct{ X, Y, DX, DY int }

func (c *coordHandle) Close() error                                               { return nil }
func (c *coordHandle) Navigate(_ context.Context, _ string) error                 { return nil }
func (c *coordHandle) Snapshot(_ context.Context) (*nexus.Snapshot, error)        { return &nexus.Snapshot{}, nil }
func (c *coordHandle) Click(_ context.Context, _ nexus.ElementRef) error          { return nil }
func (c *coordHandle) Type(_ context.Context, _ nexus.ElementRef, _ string) error { return nil }
func (c *coordHandle) Screenshot(_ context.Context) ([]byte, error)               { return []byte{}, nil }
func (c *coordHandle) Scroll(_ context.Context, _, _ int) error                   { return nil }

func (c *coordHandle) CoordClick(_ context.Context, x, y int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clicks = append(c.clicks, clickArgs{X: x, Y: y})
	return nil
}

func (c *coordHandle) CoordType(_ context.Context, x, y int, text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.types = append(c.types, typeArgs{X: x, Y: y, Text: text})
	return nil
}

func (c *coordHandle) CoordScroll(_ context.Context, x, y, dx, dy int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.scrolls = append(c.scrolls, scrollArgs{X: x, Y: y, DX: dx, DY: dy})
	return nil
}

// coordDriver produces coordHandle on Open.
type coordDriver struct{ handle *coordHandle }

func (d *coordDriver) Kind() EngineType { return EngineChromedp }
func (d *coordDriver) Open(_ context.Context, _ Config) (SessionHandle, error) {
	return d.handle, nil
}

// TestEngine_Do_CoordClick_RoutesToCoordCapable proves that when the
// driver implements CoordCapable, Engine.Do forwards coord_click
// with the X/Y pair from the Action.
func TestEngine_Do_CoordClick_RoutesToCoordCapable(t *testing.T) {
	h := &coordHandle{}
	eng, _ := NewEngine(&coordDriver{handle: h}, Config{Engine: EngineChromedp})
	sess, err := eng.Open(context.Background(), nexus.SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	err = eng.Do(context.Background(), sess, nexus.Action{Kind: "coord_click", X: 960, Y: 540})
	if err != nil {
		t.Fatalf("coord_click: %v", err)
	}
	if len(h.clicks) != 1 {
		t.Fatalf("expected 1 coord click, got %d", len(h.clicks))
	}
	if h.clicks[0].X != 960 || h.clicks[0].Y != 540 {
		t.Errorf("click args = %+v", h.clicks[0])
	}
}

func TestEngine_Do_CoordType_ForwardsText(t *testing.T) {
	h := &coordHandle{}
	eng, _ := NewEngine(&coordDriver{handle: h}, Config{Engine: EngineChromedp})
	sess, _ := eng.Open(context.Background(), nexus.SessionOptions{})
	err := eng.Do(context.Background(), sess, nexus.Action{
		Kind: "coord_type", X: 100, Y: 200, Text: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.types) != 1 || h.types[0].Text != "hello" {
		t.Errorf("coord_type wrong: %+v", h.types)
	}
}

func TestEngine_Do_CoordScroll_ReadsDeltasFromParams(t *testing.T) {
	h := &coordHandle{}
	eng, _ := NewEngine(&coordDriver{handle: h}, Config{Engine: EngineChromedp})
	sess, _ := eng.Open(context.Background(), nexus.SessionOptions{})
	err := eng.Do(context.Background(), sess, nexus.Action{
		Kind: "coord_scroll", X: 50, Y: 60,
		Params: map[string]any{"dx": 0, "dy": -320},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.scrolls) != 1 {
		t.Fatalf("expected 1 scroll, got %d", len(h.scrolls))
	}
	s := h.scrolls[0]
	if s.X != 50 || s.Y != 60 || s.DX != 0 || s.DY != -320 {
		t.Errorf("scroll args = %+v", s)
	}
}

// legacyHandle implements only SessionHandle — not CoordCapable.
// Used to prove the dispatch error is descriptive when the driver
// hasn't been upgraded yet.
type legacyHandle struct{ *coordHandle }

// CoordCapable explicitly NOT implemented — drop the embed's
// methods to simulate a pre-Phase-6 driver.
type legacyHandle2 struct{}

func (legacyHandle2) Close() error                                               { return nil }
func (legacyHandle2) Navigate(_ context.Context, _ string) error                 { return nil }
func (legacyHandle2) Snapshot(_ context.Context) (*nexus.Snapshot, error)        { return &nexus.Snapshot{}, nil }
func (legacyHandle2) Click(_ context.Context, _ nexus.ElementRef) error          { return nil }
func (legacyHandle2) Type(_ context.Context, _ nexus.ElementRef, _ string) error { return nil }
func (legacyHandle2) Screenshot(_ context.Context) ([]byte, error)               { return []byte{}, nil }
func (legacyHandle2) Scroll(_ context.Context, _, _ int) error                   { return nil }

type legacyDriver struct{}

func (*legacyDriver) Kind() EngineType { return EngineChromedp }
func (*legacyDriver) Open(_ context.Context, _ Config) (SessionHandle, error) {
	return legacyHandle2{}, nil
}

func TestEngine_Do_CoordClick_OnLegacyDriverReturnsDescriptiveError(t *testing.T) {
	eng, _ := NewEngine(&legacyDriver{}, Config{Engine: EngineChromedp})
	sess, _ := eng.Open(context.Background(), nexus.SessionOptions{})
	err := eng.Do(context.Background(), sess, nexus.Action{Kind: "coord_click", X: 0, Y: 0})
	if err == nil {
		t.Fatal("expected error on legacy driver")
	}
	if !strings.Contains(err.Error(), "CoordCapable") {
		t.Errorf("error should mention CoordCapable, got %v", err)
	}
}

func TestEngine_Do_UnknownActionKindStillRejected(t *testing.T) {
	h := &coordHandle{}
	eng, _ := NewEngine(&coordDriver{handle: h}, Config{Engine: EngineChromedp})
	sess, _ := eng.Open(context.Background(), nexus.SessionOptions{})
	err := eng.Do(context.Background(), sess, nexus.Action{Kind: "nope"})
	if err == nil || !strings.Contains(err.Error(), "unsupported action kind") {
		t.Errorf("unexpected error: %v", err)
	}
}
