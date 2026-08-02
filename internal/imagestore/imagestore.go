package imagestore

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ImageStore interface {
	Save(key string, reader io.Reader) error
	URL(key string) string
}

type LocalStore struct {
	BasePath string
	BaseURL  string
}

func NewLocalStore(basePath, baseURL string) *LocalStore {
	return &LocalStore{BasePath: basePath, BaseURL: baseURL}
}

func (s *LocalStore) Save(key string, reader io.Reader) error {
	fullPath := filepath.Join(s.BasePath, key)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}

func (s *LocalStore) URL(key string) string {
	return strings.TrimRight(s.BaseURL, "/") + "/" + strings.TrimLeft(key, "/")
}
