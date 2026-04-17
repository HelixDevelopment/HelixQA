package orchestrator

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EvidenceStore is the pluggable sink for screenshots, videos, logs,
// and arbitrary artefacts. Implementations must be safe for concurrent
// use.
type EvidenceStore interface {
	Put(name string, data []byte) (string, error)       // returns the stored-at URL/path
	PutStream(name string, r io.Reader) (string, error) // streaming upload
	List() ([]EvidenceItem, error)
}

// EvidenceItem describes one stored artefact.
type EvidenceItem struct {
	Name      string
	URL       string
	Size      int64
	CreatedAt time.Time
}

// FileEvidenceStore is a disk-backed default implementation suitable
// for developer workstations and single-host deployments.
type FileEvidenceStore struct {
	Root string

	mu sync.Mutex
}

// NewFileEvidenceStore returns a store rooted at path. The directory
// is created if missing.
func NewFileEvidenceStore(root string) (*FileEvidenceStore, error) {
	if root == "" {
		return nil, errors.New("evidence: root is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("evidence: mkdir: %w", err)
	}
	return &FileEvidenceStore{Root: root}, nil
}

// Put writes data to root/name.
func (s *FileEvidenceStore) Put(name string, data []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.Root, filepath.Clean(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// PutStream streams r into root/name.
func (s *FileEvidenceStore) PutStream(name string, r io.Reader) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.Root, filepath.Clean(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return path, nil
}

// List enumerates stored artefacts.
func (s *FileEvidenceStore) List() ([]EvidenceItem, error) {
	out := []EvidenceItem{}
	err := filepath.Walk(s.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return err
		}
		rel, relErr := filepath.Rel(s.Root, path)
		if relErr != nil {
			return relErr
		}
		out = append(out, EvidenceItem{
			Name:      rel,
			URL:       path,
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
		return nil
	})
	return out, err
}

// Evidence bundles a store with type-specific helpers used by the
// orchestrator. A nil store still accepts writes but drops them
// silently — useful for fast-path test runs.
type Evidence struct {
	Store EvidenceStore
}

// NewEvidence returns an Evidence wrapper with no configured backend.
// Callers attach a real store via SetStore.
func NewEvidence() *Evidence { return &Evidence{} }

// SetStore attaches a backend.
func (e *Evidence) SetStore(s EvidenceStore) { e.Store = s }

// Screenshot records a PNG buffer under sessions/<session>/<step>/frame.png.
func (e *Evidence) Screenshot(session, step string, png []byte) (string, error) {
	if e.Store == nil {
		return "", nil
	}
	return e.Store.Put(fmt.Sprintf("sessions/%s/%s/frame.png", session, step), png)
}

// Log records a text log fragment.
func (e *Evidence) Log(session, step, msg string) (string, error) {
	if e.Store == nil {
		return "", nil
	}
	name := fmt.Sprintf("sessions/%s/%s/log.txt", session, step)
	return e.Store.Put(name, []byte(msg))
}
