package service

import (
	"context"
	"testing"

	"ipmanlk/breeze/internal/domain"
)

func TestProjectService_Create_Success(t *testing.T) {
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	viewRepo := newMockViewRepo()
	svc := NewProjectService(projRepo, statusRepo, viewRepo)

	project, err := svc.Create(context.Background(), "org-1", "My Project", "user-1", nil, false, domain.CycleHandlingNextCycle, nil, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if project == nil {
		t.Fatal("Create() project is nil")
	}
	if project.Name != "My Project" {
		t.Errorf("Name = %q, want %q", project.Name, "My Project")
	}
	if project.Slug != "my-project" {
		t.Errorf("Slug = %q, want %q", project.Slug, "my-project")
	}
	if project.OrgID != "org-1" {
		t.Errorf("OrgID = %q, want %q", project.OrgID, "org-1")
	}
}

func TestProjectService_Create_Slugifies(t *testing.T) {
	tests := []struct {
		name     string
		projName string
		wantSlug string
	}{
		{name: "simple", projName: "My Project", wantSlug: "my-project"},
		{name: "multiple spaces", projName: "Hello   World", wantSlug: "hello-world"},
		{name: "special chars", projName: "Hello! World?", wantSlug: "hello-world"},
		{name: "numbers", projName: "Sprint 42", wantSlug: "sprint-42"},
		{name: "underscores", projName: "my_project_name", wantSlug: "my_project_name"},
		{name: "mixed case", projName: "MyNew Project", wantSlug: "mynew-project"},
		{name: "only special chars", projName: "@@@!!!", wantSlug: "project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projRepo := newMockProjectRepo()
			svc := NewProjectService(projRepo, newMockTaskStatusRepo(), newMockViewRepo())
			project, err := svc.Create(context.Background(), "org-1", tt.projName, "user-1", nil, false, domain.CycleHandlingNextCycle, nil, nil)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if project.Slug != tt.wantSlug {
				t.Errorf("Slug = %q, want %q", project.Slug, tt.wantSlug)
			}
		})
	}
}

func TestProjectService_List(t *testing.T) {
	projRepo := newMockProjectRepo()
	svc := NewProjectService(projRepo, newMockTaskStatusRepo(), newMockViewRepo())

	projRepo.projectsByID["p1"] = &domain.Project{ID: "p1", OrgID: "org-1", Name: "Alpha", Slug: "alpha"}
	projRepo.projectsByID["p2"] = &domain.Project{ID: "p2", OrgID: "org-1", Name: "Beta", Slug: "beta"}
	projRepo.projectsByID["p3"] = &domain.Project{ID: "p3", OrgID: "org-2", Name: "Other", Slug: "other"}

	projects, err := svc.List(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(projects) != 2 {
		t.Errorf("got %d projects, want 2", len(projects))
	}
}

func TestProjectService_Delete(t *testing.T) {
	projRepo := newMockProjectRepo()
	svc := NewProjectService(projRepo, newMockTaskStatusRepo(), newMockViewRepo())

	projRepo.projectsByID["p1"] = &domain.Project{ID: "p1", OrgID: "org-1", Name: "Alpha", Slug: "alpha"}
	if err := svc.Delete(context.Background(), "org-1", "p1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := projRepo.GetByID(context.Background(), "org-1", "p1"); err == nil {
		t.Error("expected project to be deleted")
	}
}

func TestProjectService_Create_HandlesSlugCollision(t *testing.T) {
	repo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	viewRepo := newMockViewRepo()

	repo.projectsByID["p1"] = &domain.Project{
		ID: "p1", OrgID: "org-1", Name: "Test Project", Slug: "test-project",
	}

	svc := NewProjectService(repo, statusRepo, viewRepo)
	p, err := svc.Create(context.Background(), "org-1", "Test Project", "user-1", nil, false, domain.CycleHandlingNextCycle, nil, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if p.Slug == "test-project" {
		t.Fatalf("expected a deduplicated slug, got %q", p.Slug)
	}
	if p.Slug != "test-project-2" {
		t.Errorf("slug = %q, want test-project-2", p.Slug)
	}
}

func TestProjectService_Update_HandlesSlugCollision(t *testing.T) {
	repo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	viewRepo := newMockViewRepo()

	repo.projectsByID["p1"] = &domain.Project{ID: "p1", OrgID: "org-1", Name: "Alpha", Slug: "alpha"}
	repo.projectsByID["p2"] = &domain.Project{ID: "p2", OrgID: "org-1", Name: "Beta", Slug: "beta"}

	svc := NewProjectService(repo, statusRepo, viewRepo)
	err := svc.Update(context.Background(), &domain.Project{
		ID: "p2", OrgID: "org-1", Name: "Alpha", Slug: "alpha",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updated := repo.projectsByID["p2"]
	if updated.Slug != "alpha-2" {
		t.Errorf("slug = %q, want alpha-2", updated.Slug)
	}
}

func TestProjectService_ArchiveAndUnarchive(t *testing.T) {
	repo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	viewRepo := newMockViewRepo()

	repo.projectsByID["p1"] = &domain.Project{ID: "p1", OrgID: "org-1", Name: "Active", Slug: "active"}

	svc := NewProjectService(repo, statusRepo, viewRepo)

	if err := svc.Archive(context.Background(), "org-1", "p1"); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if !repo.projectsByID["p1"].IsArchived {
		t.Error("expected IsArchived=true after Archive")
	}
	// Archived project should be filtered out of the default List.
	active, err := svc.List(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(active) != 0 {
		t.Errorf("expected 0 active projects, got %d", len(active))
	}

	if err := svc.Unarchive(context.Background(), "org-1", "p1"); err != nil {
		t.Fatalf("Unarchive() error = %v", err)
	}
	if repo.projectsByID["p1"].IsArchived {
		t.Error("expected IsArchived=false after Unarchive")
	}
	active, _ = svc.List(context.Background(), "org-1")
	if len(active) != 1 {
		t.Errorf("expected 1 active project after unarchive, got %d", len(active))
	}
}
