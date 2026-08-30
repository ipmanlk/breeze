package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocal_RejectsPathTraversal(t *testing.T) {
	tmp := t.TempDir()
	s := NewLocal(tmp)

	// A traversal payload must not escape the base directory.
	if _, err := s.Get(context.Background(), "../../etc/passwd"); err != ErrPathTraversal {
		t.Errorf("Get traversal: err = %v, want ErrPathTraversal", err)
	}
	if err := s.Delete(context.Background(), "../../etc/passwd"); err != ErrPathTraversal {
		t.Errorf("Delete traversal: err = %v, want ErrPathTraversal", err)
	}
	if err := s.Save(context.Background(), "../escape.bin", bytes.NewReader([]byte("x"))); err != ErrPathTraversal {
		t.Errorf("Save traversal: err = %v, want ErrPathTraversal", err)
	}

	// Ensure nothing was written outside basePath.
	if _, err := os.Stat(filepath.Join(tmp, "..", "escape.bin")); !os.IsNotExist(err) {
		t.Errorf("traversal Save wrote outside base: %v", err)
	}
}

func TestLocal_SaveGet_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	s := NewLocal(tmp)

	payload := []byte("hello breeze")
	if err := s.Save(context.Background(), "tasks/t1/att.bin", bytes.NewReader(payload)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	r, err := s.Get(context.Background(), "tasks/t1/att.bin")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer r.Close()

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("round-trip = %q, want %q", got, payload)
	}

	if err := s.Delete(context.Background(), "tasks/t1/att.bin"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}
