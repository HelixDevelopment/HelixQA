package browser

import (
	"context"
	"errors"
	"sync"

	"digital.vasic.helixqa/pkg/nexus"
)

// Pool caps the number of simultaneous browser sessions to avoid
// exceeding the host resource budget (4 CPU / 8 GB enforced by the
// project CLAUDE.md). Callers Acquire a session, use it, and Release
// when done. Released sessions are closed — the pool does not recycle
// handles because many browser adapters leak state across navigations.
type Pool struct {
	adapter nexus.Adapter
	size    int

	mu     sync.Mutex
	active int
	wake   chan struct{}
	closed bool
}

// NewPool returns a Pool that caps concurrent sessions at size. A size
// of zero or less is rejected so misconfiguration is caught up-front.
func NewPool(adapter nexus.Adapter, size int) (*Pool, error) {
	if adapter == nil {
		return nil, errors.New("browser: pool requires an Adapter")
	}
	if size <= 0 {
		return nil, errors.New("browser: pool size must be > 0")
	}
	return &Pool{
		adapter: adapter,
		size:    size,
		wake:    make(chan struct{}, 1),
	}, nil
}

// Acquire blocks until a slot is available or ctx is done. On success
// it returns a live Session that the caller must Release.
func (p *Pool) Acquire(ctx context.Context, opts nexus.SessionOptions) (nexus.Session, error) {
	for {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return nil, errors.New("browser: pool is closed")
		}
		if p.active < p.size {
			p.active++
			p.mu.Unlock()
			sess, err := p.adapter.Open(ctx, opts)
			if err != nil {
				p.mu.Lock()
				p.active--
				p.signalWake()
				p.mu.Unlock()
				return nil, err
			}
			return sess, nil
		}
		p.mu.Unlock()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-p.wake:
		}
	}
}

// Release closes sess and frees its slot. Passing nil is safe.
func (p *Pool) Release(sess nexus.Session) {
	if sess == nil {
		return
	}
	_ = sess.Close()
	p.mu.Lock()
	if p.active > 0 {
		p.active--
	}
	p.signalWake()
	p.mu.Unlock()
}

// Close marks the pool closed; in-flight Acquire calls will observe
// the state on their next iteration.
func (p *Pool) Close() {
	p.mu.Lock()
	p.closed = true
	p.signalWake()
	p.mu.Unlock()
}

// Active reports the number of outstanding sessions for tests and
// observability.
func (p *Pool) Active() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.active
}

// signalWake pokes one waiter. Must be called with p.mu held.
func (p *Pool) signalWake() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}
