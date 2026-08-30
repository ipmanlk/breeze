package service

import (
	"context"
	"testing"

	"ipmanlk/breeze/internal/domain"
)

func TestViewService_Create(t *testing.T) {
	repo := newMockViewRepo()
	svc := NewViewService(repo, nil)

	view, err := svc.Create(context.Background(), domain.CreateViewParams{
		OrgID:     "org-1",
		Name:      "All Tasks",
		Layout:    domain.ViewLayoutBoard,
		Filters:   domain.ViewFilters{Search: "bug"},
		CreatedBy: "user-1",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if view == nil {
		t.Fatal("Create() view is nil")
	}
	if view.Name != "All Tasks" {
		t.Errorf("Name = %q, want %q", view.Name, "All Tasks")
	}
	if view.Layout != domain.ViewLayoutBoard {
		t.Errorf("Layout = %q, want %q", view.Layout, domain.ViewLayoutBoard)
	}
	if view.OrgID != "org-1" {
		t.Errorf("OrgID = %q, want %q", view.OrgID, "org-1")
	}
	if view.Filters.Search != "bug" {
		t.Errorf("Filters.Search = %q, want %q", view.Filters.Search, "bug")
	}
}

func TestViewService_Create_ProjectView(t *testing.T) {
	repo := newMockViewRepo()
	svc := NewViewService(repo, nil)

	pid := "proj-1"
	view, err := svc.Create(context.Background(), domain.CreateViewParams{
		OrgID:     "org-1",
		ProjectID: &pid,
		Name:      "High Priority",
		Layout:    domain.ViewLayoutList,
		Filters:   domain.ViewFilters{Priority: "high"},
		CreatedBy: "user-1",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if view.ProjectID == nil || *view.ProjectID != pid {
		t.Errorf("ProjectID = %v, want %q", view.ProjectID, pid)
	}
	if view.Filters.Priority != "high" {
		t.Errorf("Filters.Priority = %q, want %q", view.Filters.Priority, "high")
	}
}

func TestViewService_GetByID(t *testing.T) {
	repo := newMockViewRepo()
	svc := NewViewService(repo, nil)

	repo.viewsByID["v1"] = &domain.View{ID: "v1", OrgID: "org-1", Name: "Board", Layout: domain.ViewLayoutBoard}

	view, err := svc.GetByID(context.Background(), "org-1", "v1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if view.Name != "Board" {
		t.Errorf("Name = %q, want %q", view.Name, "Board")
	}

	_, err = svc.GetByID(context.Background(), "org-2", "v1")
	if err == nil {
		t.Error("expected error for wrong orgID")
	}

	_, err = svc.GetByID(context.Background(), "org-1", "missing")
	if err == nil {
		t.Error("expected error for missing view")
	}
}

func TestViewService_Update(t *testing.T) {
	repo := newMockViewRepo()
	svc := NewViewService(repo, nil)

	repo.viewsByID["v1"] = &domain.View{ID: "v1", OrgID: "org-1", Name: "Board", Layout: domain.ViewLayoutBoard, Filters: domain.ViewFilters{}}

	name := "Updated Board"
	layout := domain.ViewLayoutList
	filters := domain.ViewFilters{Priority: "urgent"}
	view, err := svc.Update(context.Background(), "", domain.UpdateViewParams{
		ID:      "v1",
		OrgID:   "org-1",
		Name:    &name,
		Layout:  &layout,
		Filters: &filters,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if view.Name != "Updated Board" {
		t.Errorf("Name = %q, want %q", view.Name, "Updated Board")
	}
	if view.Layout != domain.ViewLayoutList {
		t.Errorf("Layout = %q, want %q", view.Layout, domain.ViewLayoutList)
	}
	if view.Filters.Priority != "urgent" {
		t.Errorf("Filters.Priority = %q, want %q", view.Filters.Priority, "urgent")
	}
}

func TestViewService_Update_NotFound(t *testing.T) {
	repo := newMockViewRepo()
	svc := NewViewService(repo, nil)

	name := "Updated"
	_, err := svc.Update(context.Background(), "", domain.UpdateViewParams{
		ID:    "missing",
		OrgID: "org-1",
		Name:  &name,
	})
	if err == nil {
		t.Error("expected error updating non-existent view")
	}
}

func TestViewService_Delete(t *testing.T) {
	repo := newMockViewRepo()
	svc := NewViewService(repo, nil)

	repo.viewsByID["v1"] = &domain.View{ID: "v1", OrgID: "org-1", Name: "Custom", Layout: domain.ViewLayoutBoard}

	if err := svc.Delete(context.Background(), "", "org-1", "v1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, ok := repo.viewsByID["v1"]; ok {
		t.Error("expected view to be deleted")
	}
}

func TestViewService_PinUnpin(t *testing.T) {
	repo := newMockViewRepo()
	svc := NewViewService(repo, nil)

	repo.viewsByID["v1"] = &domain.View{ID: "v1", OrgID: "org-1", Name: "Board", Layout: domain.ViewLayoutBoard}

	if err := svc.Pin(context.Background(), "org-1", "v1", "user-1"); err != nil {
		t.Fatalf("Pin() error = %v", err)
	}

	pinned, err := svc.ListPinned(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ListPinned() error = %v", err)
	}
	if len(pinned) != 1 {
		t.Errorf("got %d pinned views, want 1", len(pinned))
	}

	if err := svc.Unpin(context.Background(), "org-1", "v1", "user-1"); err != nil {
		t.Fatalf("Unpin() error = %v", err)
	}

	pinned, err = svc.ListPinned(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ListPinned() after unpin error = %v", err)
	}
	if len(pinned) != 0 {
		t.Errorf("got %d pinned views after unpin, want 0", len(pinned))
	}
}

func TestViewService_ListByProject(t *testing.T) {
	repo := newMockViewRepo()
	svc := NewViewService(repo, nil)

	pid := "proj-1"
	repo.viewsByID["v1"] = &domain.View{ID: "v1", OrgID: "org-1", ProjectID: &pid, Name: "Board", Layout: domain.ViewLayoutBoard}
	repo.viewsByID["v2"] = &domain.View{ID: "v2", OrgID: "org-1", ProjectID: &pid, Name: "List", Layout: domain.ViewLayoutList}
	repo.viewsByID["v3"] = &domain.View{ID: "v3", OrgID: "org-1", ProjectID: nil, Name: "Global", Layout: domain.ViewLayoutBoard}

	views, err := svc.ListByProject(context.Background(), "org-1", pid)
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}
	if len(views) != 2 {
		t.Errorf("got %d views, want 2", len(views))
	}
}

func TestViewService_ListGlobal(t *testing.T) {
	repo := newMockViewRepo()
	svc := NewViewService(repo, nil)

	pid := "proj-1"
	repo.viewsByID["v1"] = &domain.View{ID: "v1", OrgID: "org-1", ProjectID: &pid, Name: "Board", Layout: domain.ViewLayoutBoard}
	repo.viewsByID["v2"] = &domain.View{ID: "v2", OrgID: "org-1", ProjectID: nil, Name: "Global", Layout: domain.ViewLayoutList}

	views, err := svc.ListGlobal(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("ListGlobal() error = %v", err)
	}
	if len(views) != 1 {
		t.Errorf("got %d views, want 1", len(views))
	}
	if views[0].Name != "Global" {
		t.Errorf("Name = %q, want %q", views[0].Name, "Global")
	}
}
