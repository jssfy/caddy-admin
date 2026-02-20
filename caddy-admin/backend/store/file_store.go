package store

import (
	"caddy-admin/caddy"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// FileStore persists ServiceConfig entries to a JSON file.
type FileStore struct {
	mu   sync.RWMutex
	path string
}

// NewFileStore creates a FileStore at the given path.
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

// Load returns all stored services. File-not-found returns empty slice.
func (fs *FileStore) Load() ([]caddy.ServiceConfig, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.unsafeLoad()
}

// Upsert adds or updates a service by name.
func (fs *FileStore) Upsert(svc caddy.ServiceConfig) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	services, err := fs.unsafeLoad()
	if err != nil {
		return err
	}

	found := false
	for i, s := range services {
		if s.Name == svc.Name {
			services[i] = svc
			found = true
			break
		}
	}
	if !found {
		services = append(services, svc)
	}
	return fs.unsafeSave(services)
}

// Delete removes a service by name. No error if not found.
func (fs *FileStore) Delete(name string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	services, err := fs.unsafeLoad()
	if err != nil {
		return err
	}

	filtered := services[:0]
	for _, s := range services {
		if s.Name != name {
			filtered = append(filtered, s)
		}
	}
	return fs.unsafeSave(filtered)
}

func (fs *FileStore) unsafeLoad() ([]caddy.ServiceConfig, error) {
	data, err := os.ReadFile(fs.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []caddy.ServiceConfig{}, nil
		}
		return nil, err
	}
	var services []caddy.ServiceConfig
	if err := json.Unmarshal(data, &services); err != nil {
		return nil, err
	}
	return services, nil
}

func (fs *FileStore) unsafeSave(services []caddy.ServiceConfig) error {
	data, err := json.MarshalIndent(services, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(fs.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmp := fs.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, fs.path)
}
