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

// RetentionPolicy configures FileEvidenceStore.Sweep. A zero value
// means "never touch anything". Each non-zero field contributes an
// independent eviction rule; Sweep keeps an artefact when every
// configured rule says "keep". P4 fix
// (docs/nexus/remaining-work.md): the store used to grow unbounded;
// operators now call Sweep periodically to age evidence out.
type RetentionPolicy struct {
	// MaxAge drops artefacts older than this duration. Zero = keep.
	MaxAge time.Duration
	// MaxBytes drops oldest-first until the total footprint is at or
	// below this budget. Zero = keep.
	MaxBytes int64
	// MaxItems drops oldest-first until at most N items remain. Zero
	// = keep.
	MaxItems int
}

// SweepResult reports what Sweep removed.
type SweepResult struct {
	Deleted      []string
	DeletedBytes int64
}

// Sweep applies the retention policy to the evidence root and
// removes any artefacts that fall outside it. The oldest files are
// evicted first (by mtime). Directories that become empty after a
// sweep are pruned so the on-disk layout stays tidy.
func (s *FileEvidenceStore) Sweep(policy RetentionPolicy) (SweepResult, error) {
	items, err := s.List()
	if err != nil {
		return SweepResult{}, err
	}
	// Sort oldest-first so we can evict from the head.
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].CreatedAt.Before(items[j-1].CreatedAt); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var totalBytes int64
	for _, it := range items {
		totalBytes += it.Size
	}

	result := SweepResult{}
	now := time.Now()
	// Pre-compute which items violate MaxAge — those are always
	// evicted first regardless of MaxBytes / MaxItems.
	keep := make([]bool, len(items))
	for i, it := range items {
		keep[i] = true
		if policy.MaxAge > 0 && now.Sub(it.CreatedAt) > policy.MaxAge {
			keep[i] = false
		}
	}
	// Apply MaxItems: trim oldest remaining items until the kept
	// count is at or below MaxItems.
	if policy.MaxItems > 0 {
		kept := 0
		for i := len(items) - 1; i >= 0; i-- {
			if keep[i] {
				kept++
				if kept > policy.MaxItems {
					keep[i] = false
				}
			}
		}
	}
	// Apply MaxBytes: evict oldest kept items until footprint fits.
	if policy.MaxBytes > 0 {
		runningBytes := int64(0)
		for _, it := range items {
			_ = it // placeholder so gofmt stays happy if we tweak logic
		}
		for i := len(items) - 1; i >= 0; i-- {
			if !keep[i] {
				continue
			}
			if runningBytes+items[i].Size > policy.MaxBytes {
				keep[i] = false
				continue
			}
			runningBytes += items[i].Size
		}
	}

	for i, it := range items {
		if keep[i] {
			continue
		}
		if err := os.Remove(it.URL); err != nil && !os.IsNotExist(err) {
			return result, fmt.Errorf("evidence sweep: remove %s: %w", it.URL, err)
		}
		result.Deleted = append(result.Deleted, it.Name)
		result.DeletedBytes += it.Size
	}

	// Prune empty directories left behind by the sweep (except the
	// root itself). Walk depth-first.
	_ = filepath.Walk(s.Root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || !info.IsDir() || path == s.Root {
			return nil
		}
		entries, _ := os.ReadDir(path)
		if len(entries) == 0 {
			_ = os.Remove(path)
		}
		return nil
	})

	return result, nil
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
