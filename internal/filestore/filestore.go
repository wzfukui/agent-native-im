package filestore

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// generateSafeFilename creates a URL-safe filename
func generateSafeFilename(original string) string {
	// Get file extension
	ext := filepath.Ext(original)
	if ext == "" {
		ext = ".bin"
	}

	// Generate random ID
	b := make([]byte, 8)
	rand.Read(b)
	randomID := hex.EncodeToString(b)

	// Create timestamp
	timestamp := time.Now().Format("20060102_150405")

	// Return safe filename: timestamp_randomID.ext
	return fmt.Sprintf("%s_%s%s", timestamp, randomID, strings.ToLower(ext))
}

func (s *LocalStore) Save(filename string, r io.Reader) (string, error) {
	// Generate safe filename to avoid encoding issues
	stored := generateSafeFilename(filename)
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
