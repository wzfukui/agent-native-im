package filestore

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type FileStore interface {
	Save(filename string, r io.Reader) (url string, err error)
	ServePath() string
}

type LocalStore struct {
	dir      string
	urlBase  string
}

func NewLocalStore(dir, urlBase string) (*LocalStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}
	return &LocalStore{dir: dir, urlBase: urlBase}, nil
}

func (s *LocalStore) Save(filename string, r io.Reader) (string, error) {
	// Prefix with timestamp to avoid collisions
	stored := fmt.Sprintf("%d-%s", time.Now().UnixMilli(), filepath.Base(filename))
	dst := filepath.Join(s.dir, stored)

	f, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return s.urlBase + "/" + stored, nil
}

func (s *LocalStore) ServePath() string {
	return s.dir
}
