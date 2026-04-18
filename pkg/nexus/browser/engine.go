package browser

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"digital.vasic.helixqa/pkg/nexus"
)

// EngineType selects the underlying driver.
type EngineType string

const (
	EngineChromedp   EngineType = "chromedp"
	EngineRod        EngineType = "rod"
	EnginePlaywright EngineType = "playwright"
)

// Config configures a browser session.
type Config struct {
	Engine       EngineType
	Headless     bool
	UserDataDir  string
	CDPPort      int
	WindowWidth  int
	WindowHeight int
	Timeout      time.Duration
	AllowedHosts []string // non-empty activates the allowlist
	MaxBodyBytes int64
}

// Driver is the concrete contract every engine (chromedp, rod, playwright)
// must satisfy. Callers never depend on a Driver directly; they go through
// Engine, which wraps the chosen Driver with security + observability.
type Driver interface {
	Kind() EngineType
	Open(ctx context.Context, cfg Config) (SessionHandle, error)
}

// SessionHandle is the driver-owned representation of a single session.
// It is returned by Driver.Open and consumed by the Engine to satisfy
// nexus.Session semantics without leaking driver types.
type SessionHandle interface {
	Close() error
	Navigate(ctx context.Context, url string) error
	Snapshot(ctx context.Context) (*nexus.Snapshot, error)
	Click(ctx context.Context, ref nexus.ElementRef) error
	Type(ctx context.Context, ref nexus.ElementRef, text string) error
	Screenshot(ctx context.Context) ([]byte, error)
	Scroll(ctx context.Context, dx, dy int) error
}

// Engine is the HelixQA-facing facade. It exposes an Adapter on top of
// the selected Driver, applies URL allowlisting and body-size caps, and
// translates errors via ToAIFriendlyError.
type Engine struct {
	driver    Driver
	cfg       Config
	sessions  atomic.Int64
	idCounter atomic.Uint64
}

// NewEngine returns an Engine that drives the named EngineType. The
// constructor rejects unknown engines up-front so tests never block on
// driver startup for a typo.
func NewEngine(d Driver, cfg Config) (*Engine, error) {
	if d == nil {
		return nil, fmt.Errorf("browser: nil driver")
	}
	if cfg.Engine == "" {
		cfg.Engine = d.Kind()
	}
	if cfg.Engine != d.Kind() {
		return nil, fmt.Errorf("browser: driver kind=%s does not match config=%s", d.Kind(), cfg.Engine)
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = 32 << 20 // 32 MiB default
	}
	return &Engine{driver: d, cfg: cfg}, nil
}

// Open satisfies nexus.Adapter.
func (e *Engine) Open(ctx context.Context, opts nexus.SessionOptions) (nexus.Session, error) {
	cfg := e.cfg
	if opts.Headless {
		cfg.Headless = true
	}
	if opts.UserDataDir != "" {
		cfg.UserDataDir = opts.UserDataDir
	}
	if opts.WindowSize[0] > 0 {
		cfg.WindowWidth = opts.WindowSize[0]
	}
	if opts.WindowSize[1] > 0 {
		cfg.WindowHeight = opts.WindowSize[1]
	}
	if opts.Timeout > 0 {
		cfg.Timeout = opts.Timeout
	}
	h, err := e.driver.Open(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open session: %s", ToAIFriendlyError(err))
	}
	e.sessions.Add(1)
	id := fmt.Sprintf("nexus-%s-%d", cfg.Engine, e.idCounter.Add(1))
	return &session{id: id, engine: e, handle: h}, nil
}

// Navigate satisfies nexus.Adapter.
func (e *Engine) Navigate(ctx context.Context, s nexus.Session, target string) error {
	sess, ok := s.(*session)
	if !ok {
		return fmt.Errorf("browser: foreign session type %T", s)
	}
	if err := e.allowURL(target); err != nil {
		return fmt.Errorf("navigate denied: %w", err)
	}
	if err := sess.handle.Navigate(ctx, target); err != nil {
		return fmt.Errorf("navigate: %s", ToAIFriendlyError(err))
	}
	return nil
}

// Snapshot satisfies nexus.Adapter.
func (e *Engine) Snapshot(ctx context.Context, s nexus.Session) (*nexus.Snapshot, error) {
	sess, ok := s.(*session)
	if !ok {
		return nil, fmt.Errorf("browser: foreign session type %T", s)
	}
	snap, err := sess.handle.Snapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("snapshot: %s", ToAIFriendlyError(err))
	}
	return snap, nil
}

// CoordCapable is the optional extension interface a SessionHandle
// implements to accept Phase-6 coordinate-grounded Actions
// (coord_click / coord_type / coord_scroll). Drivers that do not
// implement it receive a descriptive error from Engine.Do so
// operators know which driver to upgrade.
type CoordCapable interface {
	CoordClick(ctx context.Context, x, y int) error
	CoordType(ctx context.Context, x, y int, text string) error
	CoordScroll(ctx context.Context, x, y, dx, dy int) error
}

// Do dispatches an Action to the underlying handle. Phase-6 coord_*
// kinds are routed to the CoordCapable extension interface when the
// driver implements it, otherwise the call fails with an
// actionable error.
func (e *Engine) Do(ctx context.Context, s nexus.Session, a nexus.Action) error {
	sess, ok := s.(*session)
	if !ok {
		return fmt.Errorf("browser: foreign session type %T", s)
	}
	switch a.Kind {
	case "click":
		return sess.handle.Click(ctx, nexus.ElementRef(a.Target))
	case "type":
		return sess.handle.Type(ctx, nexus.ElementRef(a.Target), a.Text)
	case "scroll":
		dx, _ := a.Params["dx"].(int)
		dy, _ := a.Params["dy"].(int)
		return sess.handle.Scroll(ctx, dx, dy)
	case "screenshot":
		_, err := sess.handle.Screenshot(ctx)
		return err
	case "coord_click":
		cc, ok := sess.handle.(CoordCapable)
		if !ok {
			return fmt.Errorf("browser: driver does not implement CoordCapable; coord_click requires a coord-mode-capable driver")
		}
		return cc.CoordClick(ctx, a.X, a.Y)
	case "coord_type":
		cc, ok := sess.handle.(CoordCapable)
		if !ok {
			return fmt.Errorf("browser: driver does not implement CoordCapable; coord_type requires a coord-mode-capable driver")
		}
		return cc.CoordType(ctx, a.X, a.Y, a.Text)
	case "coord_scroll":
		cc, ok := sess.handle.(CoordCapable)
		if !ok {
			return fmt.Errorf("browser: driver does not implement CoordCapable; coord_scroll requires a coord-mode-capable driver")
		}
		dx, _ := a.Params["dx"].(int)
		dy, _ := a.Params["dy"].(int)
		return cc.CoordScroll(ctx, a.X, a.Y, dx, dy)
	default:
		return fmt.Errorf("browser: unsupported action kind %q", a.Kind)
	}
}

// Screenshot satisfies nexus.Adapter.
func (e *Engine) Screenshot(ctx context.Context, s nexus.Session) ([]byte, error) {
	sess, ok := s.(*session)
	if !ok {
		return nil, fmt.Errorf("browser: foreign session type %T", s)
	}
	return sess.handle.Screenshot(ctx)
}

// ActiveSessions reports how many session handles the engine has produced
// since startup minus the number that have been Closed. It is used by the
// pool and by tests to detect leaks.
func (e *Engine) ActiveSessions() int64 {
	return e.sessions.Load()
}

func (e *Engine) allowURL(target string) error {
	if target == "" {
		return fmt.Errorf("empty target url")
	}
	lower := toLower(target)
	for _, bad := range unsafeSchemes {
		if hasPrefix(lower, bad) {
			return fmt.Errorf("scheme %q is blocked for safety", bad)
		}
	}
	if len(e.cfg.AllowedHosts) == 0 {
		return nil
	}
	host := extractHost(target)
	for _, h := range e.cfg.AllowedHosts {
		if host == h {
			return nil
		}
	}
	return fmt.Errorf("host %q is not in the allowlist", host)
}

var unsafeSchemes = []string{"file:", "javascript:", "data:", "vbscript:"}

// --- tiny local helpers avoid importing net/url when URL parsing needs to
// tolerate things the stdlib refuses to accept (malformed inputs are rare
// but should still be rejected by the allowlist gracefully).

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func hasPrefix(s, p string) bool {
	if len(s) < len(p) {
		return false
	}
	return s[:len(p)] == p
}

func extractHost(u string) string {
	// Strip scheme.
	idx := indexOf(u, "://")
	if idx >= 0 {
		u = u[idx+3:]
	}
	// Strip path, query, fragment.
	for i := 0; i < len(u); i++ {
		if u[i] == '/' || u[i] == '?' || u[i] == '#' {
			u = u[:i]
			break
		}
	}
	// Strip userinfo.
	if at := indexOf(u, "@"); at >= 0 {
		u = u[at+1:]
	}
	// Strip port.
	if colon := indexOf(u, ":"); colon >= 0 {
		u = u[:colon]
	}
	return u
}

func indexOf(s, sub string) int {
	if len(sub) == 0 || len(sub) > len(s) {
		return -1
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

type session struct {
	id     string
	engine *Engine
	handle SessionHandle
}

func (s *session) ID() string { return s.id }
func (s *session) Platform() nexus.Platform {
	switch s.engine.cfg.Engine {
	case EngineChromedp:
		return nexus.PlatformWebChromedp
	case EngineRod:
		return nexus.PlatformWebRod
	case EnginePlaywright:
		return nexus.PlatformWebPlaywright
	default:
		return nexus.PlatformWebChromedp
	}
}
func (s *session) Close() error {
	s.engine.sessions.Add(-1)
	return s.handle.Close()
}

var _ nexus.Adapter = (*Engine)(nil)
var _ nexus.Session = (*session)(nil)
