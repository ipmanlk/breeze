package service

import (
	"context"
	"errors"
	"testing"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
)

func seedLabel(t *testing.T, repo *mockLabelRepo, orgID, id, name, color string) *domain.Label {
	t.Helper()
	l := &domain.Label{ID: id, OrgID: orgID, Name: name, Color: color}
	if err := repo.Create(context.Background(), l); err != nil {
		t.Fatalf("seed label: %v", err)
	}
	return l
}

func TestLabelService_Create(t *testing.T) {
	repo := newMockLabelRepo()
	svc := NewLabelService(repo, nil, nil, nil, nil)

	label, err := svc.Create(context.Background(), "", "org-1", "Bug", "#ef4444")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if label.Name != "Bug" {
		t.Errorf("Name = %q, want Bug", label.Name)
	}
	if label.Color != "#ef4444" {
		t.Errorf("Color = %q, want #ef4444", label.Color)
	}
	if label.ID == "" {
		t.Error("ID is empty")
	}
}

func TestLabelService_Create_DefaultColor(t *testing.T) {
	repo := newMockLabelRepo()
	svc := NewLabelService(repo, nil, nil, nil, nil)

	label, err := svc.Create(context.Background(), "", "org-1", "No Color", "not-a-color")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if label.Color != "#6366f1" {
		t.Errorf("Color = %q, want default #6366f1", label.Color)
	}
}

func TestLabelService_Create_InvalidName(t *testing.T) {
	repo := newMockLabelRepo()
	svc := NewLabelService(repo, nil, nil, nil, nil)

	if _, err := svc.Create(context.Background(), "", "org-1", "", "#fff"); !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("Create() empty name error = %v, want ErrInvalidInput", err)
	}
	longName := make([]byte, 33)
	for i := range longName {
		longName[i] = 'x'
	}
	if _, err := svc.Create(context.Background(), "", "org-1", string(longName), "#fff"); !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("Create() long name error = %v, want ErrInvalidInput", err)
	}
}

func TestLabelService_List(t *testing.T) {
	repo := newMockLabelRepo()
	svc := NewLabelService(repo, nil, nil, nil, nil)
	seedLabel(t, repo, "org-1", "l-1", "Bug", "#ef4444")
	seedLabel(t, repo, "org-1", "l-2", "Feature", "#22c55e")
	seedLabel(t, repo, "org-2", "l-3", "Other Org", "#000")

	labels, err := svc.List(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(labels) != 2 {
		t.Errorf("List() returned %d labels, want 2", len(labels))
	}
}

func TestLabelService_Update(t *testing.T) {
	repo := newMockLabelRepo()
	svc := NewLabelService(repo, nil, nil, nil, nil)
	seedLabel(t, repo, "org-1", "l-1", "Bug", "#ef4444")

	updated, err := svc.Update(context.Background(), "", "org-1", "l-1", "Critical", "#f97316")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != "Critical" {
		t.Errorf("Name = %q, want Critical", updated.Name)
	}
	if updated.Color != "#f97316" {
		t.Errorf("Color = %q, want #f97316", updated.Color)
	}
}

func TestLabelService_Update_NotFound(t *testing.T) {
	repo := newMockLabelRepo()
	svc := NewLabelService(repo, nil, nil, nil, nil)

	_, err := svc.Update(context.Background(), "", "org-1", "missing", "Name", "#fff")
	if err == nil {
		t.Error("Update() expected error for missing label")
	}
}

func TestLabelService_Delete(t *testing.T) {
	repo := newMockLabelRepo()
	svc := NewLabelService(repo, nil, nil, nil, nil)
	seedLabel(t, repo, "org-1", "l-1", "Bug", "#ef4444")

	if err := svc.Delete(context.Background(), "", "org-1", "l-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := svc.List(context.Background(), "org-1"); err == nil {
		// List still returns but label should be gone from byID
	}
	if _, err := repo.GetByID(context.Background(), "org-1", "l-1"); err == nil {
		t.Error("label still exists after Delete")
	}
}

func TestLabelService_Delete_NotFound(t *testing.T) {
	repo := newMockLabelRepo()
	svc := NewLabelService(repo, nil, nil, nil, nil)

	if err := svc.Delete(context.Background(), "", "org-1", "missing"); err == nil {
		t.Error("Delete() expected error for missing label")
	}
}

func TestLabelService_SetTaskLabels_RejectsForeignOrgLabel(t *testing.T) {
	repo := newMockLabelRepo()
	svc := NewLabelService(repo, nil, nil, nil, nil)
	// Label belongs to org-2.
	seedLabel(t, repo, "org-2", "l-foreign", "Foreign", "#fff")

	// Attempt to attach it to a task in org-1.
	err := svc.SetTaskLabels(context.Background(), "", "org-1", "task-1", []string{"l-foreign"})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("SetTaskLabels() foreign label error = %v, want ErrNotFound", err)
	}
	// Nothing should have been cleared/added.
	if len(repo.cleared) != 0 {
		t.Errorf("expected no ClearTaskLabels, got %d", len(repo.cleared))
	}
}

func TestLabelService_SetTaskLabels_Success(t *testing.T) {
	repo := newMockLabelRepo()
	svc := NewLabelService(repo, nil, nil, nil, nil)
	seedLabel(t, repo, "org-1", "l-1", "Bug", "#ef4444")
	seedLabel(t, repo, "org-1", "l-2", "Feature", "#22c55e")

	if err := svc.SetTaskLabels(context.Background(), "", "org-1", "task-1", []string{"l-1", "l-2"}); err != nil {
		t.Fatalf("SetTaskLabels() error = %v", err)
	}
	labels, err := svc.GetTaskLabels(context.Background(), "", "org-1", "task-1")
	if err != nil {
		t.Fatalf("GetTaskLabels() error = %v", err)
	}
	if len(labels) != 2 {
		t.Errorf("GetTaskLabels() returned %d, want 2", len(labels))
	}
}

func TestLabelService_SetTaskLabels_ReplacesExisting(t *testing.T) {
	repo := newMockLabelRepo()
	svc := NewLabelService(repo, nil, nil, nil, nil)
	seedLabel(t, repo, "org-1", "l-1", "Bug", "#ef4444")
	seedLabel(t, repo, "org-1", "l-2", "Feature", "#22c55e")

	_ = svc.SetTaskLabels(context.Background(), "", "org-1", "task-1", []string{"l-1"})
	// Replace with a different set.
	if err := svc.SetTaskLabels(context.Background(), "", "org-1", "task-1", []string{"l-2"}); err != nil {
		t.Fatalf("SetTaskLabels() replace error = %v", err)
	}
	labels, _ := svc.GetTaskLabels(context.Background(), "", "org-1", "task-1")
	if len(labels) != 1 || labels[0].ID != "l-2" {
		t.Errorf("after replace, got %v, want [l-2]", labels)
	}
}
