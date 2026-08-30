package storage

import (
	"context"
	"io"
)

type Storage interface {
	Save(ctx context.Context, path string, reader io.Reader) error
	Get(ctx context.Context, path string) (io.ReadCloser, error)
	Delete(ctx context.Context, path string) error
	URL(ctx context.Context, path string) (string, error)
}
