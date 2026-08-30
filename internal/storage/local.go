package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var ErrPathTraversal = errors.New("storage path escapes base directory")

type Local struct {
	basePath string
}

func NewLocal(basePath string) *Local {
	return &Local{basePath: basePath}
}

// resolvePath joins basePath with path and guarantees the result stays within
// basePath. This prevents path traversal (../../etc/passwd) even if a future
// caller passes user-controlled input; defense in depth.
func (s *Local) resolvePath(path string) (string, error) {
	cleanBase := filepath.Clean(s.basePath)
	full := filepath.Join(cleanBase, path)
	rel, err := filepath.Rel(cleanBase, full)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", ErrPathTraversal
	}
	return full, nil
}

func (s *Local) Save(ctx context.Context, path string, reader io.Reader) error {
	fullPath, err := s.resolvePath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	file, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, reader)
	return err
}

func (s *Local) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath, err := s.resolvePath(path)
	if err != nil {
		return nil, err
	}
	return os.Open(fullPath)
}

func (s *Local) Delete(ctx context.Context, path string) error {
	fullPath, err := s.resolvePath(path)
	if err != nil {
		return err
	}
	return os.Remove(fullPath)
}

func (s *Local) URL(ctx context.Context, path string) (string, error) {
	return "/uploads/" + path, nil
}
