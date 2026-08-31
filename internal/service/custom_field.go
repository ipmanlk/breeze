package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
)

type CustomFieldService struct {
	fieldRepo port.CustomFieldRepository
	projRepo  port.ProjectRepository
	access    port.AccessChecker
}

var _ port.CustomFieldService = (*CustomFieldService)(nil)

func NewCustomFieldService(fieldRepo port.CustomFieldRepository, projRepo port.ProjectRepository, access port.AccessChecker) *CustomFieldService {
	return &CustomFieldService{fieldRepo: fieldRepo, projRepo: projRepo, access: access}
}

var validFieldTypes = map[string]bool{
	domain.CustomFieldText:   true,
	domain.CustomFieldNumber: true,
	domain.CustomFieldSelect: true,
	domain.CustomFieldDate:   true,
}

func (s *CustomFieldService) List(ctx context.Context, orgID, projectID string) ([]*domain.CustomField, error) {
	return s.fieldRepo.ListByProject(ctx, orgID, projectID)
}

func (s *CustomFieldService) Create(ctx context.Context, userID string, p domain.CreateCustomFieldParams) (*domain.CustomField, error) {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return nil, apperr.InvalidInput("name is required")
	}
	if len(p.Name) > 64 {
		return nil, apperr.InvalidInput("name must be at most 64 characters")
	}
	if !validFieldTypes[p.FieldType] {
		return nil, apperr.InvalidInput("invalid field type")
	}
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, userID, p.OrgID, p.ProjectID, domain.PermProjectManage); err != nil {
			return nil, err
		}
	}

	// Validate project belongs to org
	proj, err := s.projRepo.GetByID(ctx, p.OrgID, p.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	if proj.OrgID != p.OrgID {
		return nil, apperr.InvalidInput("project does not belong to organization")
	}

	// For select type, options are required
	if p.FieldType == domain.CustomFieldSelect && len(p.Options) == 0 {
		return nil, apperr.InvalidInput("select fields require at least one option")
	}

	f := &domain.CustomField{
		ID:        uuid.New().String(),
		OrgID:     p.OrgID,
		ProjectID: p.ProjectID,
		Name:      p.Name,
		FieldType: p.FieldType,
		Options:   p.Options,
		Position:  p.Position,
	}

	if err := s.fieldRepo.Create(ctx, f); err != nil {
		return nil, fmt.Errorf("create custom field: %w", err)
	}
	return f, nil
}

func (s *CustomFieldService) Update(ctx context.Context, userID string, p domain.UpdateCustomFieldParams) (*domain.CustomField, error) {
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, userID, p.OrgID, p.ProjectID, domain.PermProjectManage); err != nil {
			return nil, err
		}
	}
	existing, err := s.fieldRepo.GetByID(ctx, p.OrgID, p.ID)
	if err != nil {
		return nil, apperr.NotFound("custom field", err)
	}
	// Prevent cross-project IDOR: the field must belong to the project in the
	// request URL. A user with PermProjectManage on project A must not be able
	// to mutate a field from project B by swapping the fieldId.
	if existing.ProjectID != p.ProjectID {
		return nil, apperr.ErrForbidden
	}

	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return nil, apperr.InvalidInput("name is required")
	}
	if existing.FieldType == domain.CustomFieldSelect && len(p.Options) == 0 {
		return nil, apperr.InvalidInput("select fields require at least one option")
	}

	existing.Name = p.Name
	existing.Options = p.Options
	existing.Position = p.Position

	if err := s.fieldRepo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("update custom field: %w", err)
	}
	return existing, nil
}

func (s *CustomFieldService) Delete(ctx context.Context, userID, orgID, projectID, id string) error {
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, userID, orgID, projectID, domain.PermProjectManage); err != nil {
			return err
		}
	}
	// Fetch first so we can verify the field belongs to the URL's project. A
	// plain org-scoped delete would allow cross-project deletion by ID.
	existing, err := s.fieldRepo.GetByID(ctx, orgID, id)
	if err != nil {
		return apperr.NotFound("custom field", err)
	}
	if existing.ProjectID != projectID {
		return apperr.ErrForbidden
	}
	return s.fieldRepo.Delete(ctx, orgID, id)
}

func (s *CustomFieldService) GetTaskValues(ctx context.Context, userID, orgID, taskID string) (map[string]string, error) {
	// Read path must enforce the same task-level access as the write path:
	// resolve the caller's project role before returning any field values.
	if s.access != nil {
		if err := s.access.RequireTaskAccess(ctx, userID, orgID, taskID, domain.PermTaskView); err != nil {
			return nil, err
		}
	}
	return s.fieldRepo.GetValuesByTask(ctx, taskID)
}

func (s *CustomFieldService) SetTaskValue(ctx context.Context, userID, orgID, taskID, fieldID, value string) error {
	if s.access != nil {
		if err := s.access.RequireTaskAccess(ctx, userID, orgID, taskID, domain.PermTaskEdit); err != nil {
			return err
		}
	}
	// Validate the field exists and belongs to org
	field, err := s.fieldRepo.GetByID(ctx, orgID, fieldID)
	if err != nil {
		return apperr.NotFound("custom field", err)
	}

	// Validate value against field type
	if field.FieldType == domain.CustomFieldSelect {
		valid := false
		for _, opt := range field.Options {
			if opt == value {
				valid = true
				break
			}
		}
		if !valid && value != "" {
			return apperr.InvalidInput("value is not a valid option")
		}
	}

	return s.fieldRepo.SetValue(ctx, taskID, fieldID, value)
}
