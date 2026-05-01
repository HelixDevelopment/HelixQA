package screenshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Storage decouples persistence from capture.
type Storage interface {
	Save(name string, data []byte, meta Result) (string, error)
	Load(id string) ([]byte, error)
	List(sessionID string) []string
	Thumbnail(id string) ([]byte, error)
}

// fsStorage is the default filesystem-backed storage.
type fsStorage struct {
	baseDir string
	mu      sync.RWMutex
	index   map[string]indexEntry
}

type indexEntry struct {
	Path     string `json:"path"`
	Platform string `json:"platform"`
	Format   string `json:"format"`
	Size     int    `json:"size"`
}

// NewFSStorage creates a filesystem storage backend.
func NewFSStorage(baseDir string) Storage {
	_ = os.MkdirAll(baseDir, 0755)
	return &fsStorage{
		baseDir: baseDir,
		index:   make(map[string]indexEntry),
	}
}

func (s *fsStorage) Save(name string, data []byte, meta Result) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	platformDir := filepath.Join(s.baseDir, string(meta.Platform))
	_ = os.MkdirAll(platformDir, 0755)

	path := filepath.Join(platformDir, name+".png")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}

	s.index[name] = indexEntry{
		Path:     path,
		Platform: string(meta.Platform),
		Format:   meta.Format,
		Size:     len(data),
	}

	// Write index
	indexPath := filepath.Join(s.baseDir, "screenshots.json")
	indexData, _ := json.MarshalIndent(s.index, "", "  ")
	_ = os.WriteFile(indexPath, indexData, 0644)

	return path, nil
}

func (s *fsStorage) Load(id string) ([]byte, error) {
	s.mu.RLock()
	entry, ok := s.index[id]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("screenshot not found: %s", id)
	}
	return os.ReadFile(entry.Path)
}

func (s *fsStorage) List(sessionID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []string
	for id, entry := range s.index {
		if sessionID == "" || filepath.Base(filepath.Dir(entry.Path)) == sessionID {
			out = append(out, id)
		}
	}
	return out
}

func (s *fsStorage) Thumbnail(id string) ([]byte, error) {
	// For simplicity, return the full image as thumbnail.
	// In production, this would generate a 480px-wide preview.
	return s.Load(id)
}
