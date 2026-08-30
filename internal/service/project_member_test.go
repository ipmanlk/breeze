package service

import (
	"context"
	"testing"

	"ipmanlk/breeze/internal/domain"
)

func TestProjectMemberService_ListByUser(t *testing.T) {
	pmRepo := newMockProjectMemberRepo()
	svc := NewProjectMemberService(pmRepo, newMockUserRepo())

	// Seed memberships
	pmRepo.memberships["user-1"] = []*domain.UserProjectMembership{
		{ProjectID: "proj-1", Name: "Project A", Color: "#ff0000", Role: domain.RoleMember},
		{ProjectID: "proj-2", Name: "Project B", Color: "#00ff00", Role: domain.RoleAdmin},
	}

	memberships, err := svc.ListByUser(context.Background(), "org-1", "user-1")
	if err != nil {
		t.Fatalf("ListByUser() err = %v", err)
	}

	if len(memberships) != 2 {
		t.Fatalf("expected 2 memberships, got %d", len(memberships))
	}

	if memberships[0].ProjectID != "proj-1" {
		t.Errorf("expected proj-1, got %s", memberships[0].ProjectID)
	}
	if memberships[0].Role != domain.RoleMember {
		t.Errorf("expected member role, got %s", memberships[0].Role)
	}

	// User with no memberships returns empty slice
	empty, err := svc.ListByUser(context.Background(), "org-1", "user-empty")
	if err != nil {
		t.Fatalf("ListByUser() empty err = %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected 0 memberships, got %d", len(empty))
	}
}

func TestProjectMemberService_SetMemberships(t *testing.T) {
	pmRepo := newMockProjectMemberRepo()
	userRepo := newMockUserRepo()
	seedUser(userRepo, "user-1", "org-1")
	svc := NewProjectMemberService(pmRepo, userRepo)

	// Seed existing memberships
	pmRepo.memberships["user-1"] = []*domain.UserProjectMembership{
		{ProjectID: "keep-me", Role: domain.RoleMember},
		{ProjectID: "remove-me", Role: domain.RoleViewer},
	}

	// Set new memberships: keep keep-me with different role, add a new one
	err := svc.SetMemberships(context.Background(), "org-1", "user-1", []domain.ProjectAssignment{
		{ProjectID: "keep-me", Role: domain.RoleAdmin},
		{ProjectID: "add-me", Role: domain.RoleMember},
	})
	if err != nil {
		t.Fatalf("SetMemberships() err = %v", err)
	}

	// Check result via ListByUser
	memberships, err := svc.ListByUser(context.Background(), "org-1", "user-1")
	if err != nil {
		t.Fatalf("ListByUser() after set err = %v", err)
	}

	if len(memberships) != 2 {
		t.Fatalf("expected 2 memberships, got %d", len(memberships))
	}

	// "keep-me" should still be there with updated role
	keepFound := false
	for _, m := range memberships {
		if m.ProjectID == "keep-me" {
			keepFound = true
			if m.Role != domain.RoleAdmin {
				t.Errorf("keep-me role = %s, want admin", m.Role)
			}
		}
		if m.ProjectID == "add-me" {
			if m.Role != domain.RoleMember {
				t.Errorf("add-me role = %s, want member", m.Role)
			}
		}
		if m.ProjectID == "remove-me" {
			t.Error("remove-me should have been removed")
		}
	}
	if !keepFound {
		t.Error("keep-me not found after SetMemberships")
	}
}

func TestProjectMemberService_SetMemberships_InvalidRole(t *testing.T) {
	pmRepo := newMockProjectMemberRepo()
	userRepo := newMockUserRepo()
	seedUser(userRepo, "user-1", "org-1")
	svc := NewProjectMemberService(pmRepo, userRepo)

	err := svc.SetMemberships(context.Background(), "org-1", "user-1", []domain.ProjectAssignment{
		{ProjectID: "proj-1", Role: domain.Role("invalid")},
	})
	if err == nil {
		t.Fatal("SetMemberships with invalid role should return error")
	}
}

func TestProjectMemberService_SetMemberships_InvalidUser_ReturnsError(t *testing.T) {
	pmRepo := newMockProjectMemberRepo()
	userRepo := newMockUserRepo()
	// user-99 is NOT seeded, so GetByID will return not-found
	seedUser(userRepo, "user-1", "org-1")
	svc := NewProjectMemberService(pmRepo, userRepo)

	err := svc.SetMemberships(context.Background(), "org-1", "user-99", []domain.ProjectAssignment{
		{ProjectID: "proj-1", Role: domain.RoleMember},
	})
	if err == nil {
		t.Fatal("SetMemberships with user not in org should return error")
	}

	// Verify no memberships were written for the invalid user
	memberships, err := svc.ListByUser(context.Background(), "org-1", "user-99")
	if err != nil {
		t.Fatalf("ListByUser() err = %v", err)
	}
	if len(memberships) != 0 {
		t.Errorf("expected 0 memberships for invalid user, got %d", len(memberships))
	}
}
