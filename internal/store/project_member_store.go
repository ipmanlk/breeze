package store

import (
	"context"
	"database/sql"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

type ProjectMemberStore struct {
	q  *sqlc.Queries
	db *sql.DB
}

func NewProjectMemberStore(q *sqlc.Queries, db *sql.DB) *ProjectMemberStore {
	return &ProjectMemberStore{q: q, db: db}
}

var _ port.ProjectMemberRepository = (*ProjectMemberStore)(nil)

func (s *ProjectMemberStore) List(ctx context.Context, orgID, projectID string, filter domain.UserFilter) (*domain.ProjectMemberListResult, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}

	var cursorName, cursorID string
	if filter.Cursor != "" {
		var err error
		cursorName, cursorID, err = decodeCursor(filter.Cursor)
		if err != nil {
			return nil, err
		}
	}

	rows, err := s.q.ListProjectMembers(ctx, sqlc.ListProjectMembersParams{
		ProjectID:  projectID,
		OrgID:      orgID,
		Search:     nilIfEmpty(filter.Search),
		CursorName: cursorName,
		CursorID:   cursorID,
		LimitVal:   int64(limit + 1),
	})
	if err != nil {
		return nil, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	members := make([]*domain.ProjectMember, len(rows))
	for i, row := range rows {
		members[i] = &domain.ProjectMember{
			ProjectID: projectID,
			UserID:    row.ID,
			Role:      domain.Role(row.ProjectRole),
			User: &domain.User{
				ID:        row.ID,
				Name:      row.Name,
				Email:     row.Email,
				AvatarURL: row.AvatarUrl,
				Role:      domain.Role(row.OrgRole),
			},
		}
	}

	result := &domain.ProjectMemberListResult{
		Members: members,
		HasMore: hasMore,
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		result.NextCursor = encodeCursor(last.Name, last.ID)
	}

	return result, nil
}

func (s *ProjectMemberStore) Get(ctx context.Context, orgID, projectID, userID string) (*domain.ProjectMember, error) {
	row, err := s.q.GetProjectMember(ctx, sqlc.GetProjectMemberParams{
		ProjectID: projectID,
		UserID:    userID,
		OrgID:     orgID,
	})
	if err != nil {
		return nil, err
	}
	// No project_members row means this user has no explicit membership.
	// This path is only reached for viewer/guest users who MUST have a
	// project_members entry to access the project.
	if row.ProjectRole == "" {
		return nil, nil
	}
	pm := &domain.ProjectMember{
		ProjectID: projectID,
		UserID:    row.ID,
		Role:      domain.Role(row.ProjectRole),
		User: &domain.User{
			ID:        row.ID,
			Name:      row.Name,
			Email:     row.Email,
			AvatarURL: row.AvatarUrl,
			Role:      domain.Role(row.Role),
		},
	}
	return pm, nil
}

func (s *ProjectMemberStore) Add(ctx context.Context, projectID, userID string, role domain.Role) error {
	return s.q.AddProjectMember(ctx, sqlc.AddProjectMemberParams{
		ProjectID: projectID,
		Role:      string(role),
		ID:        projectID,
		ID_2:      userID,
	})
}

func (s *ProjectMemberStore) Remove(ctx context.Context, projectID, userID string) error {
	return s.q.RemoveProjectMember(ctx, sqlc.RemoveProjectMemberParams{
		ProjectID: projectID,
		UserID:    userID,
	})
}

func (s *ProjectMemberStore) UpdateRole(ctx context.Context, projectID, userID string, role domain.Role) error {
	return s.q.UpdateProjectMemberRole(ctx, sqlc.UpdateProjectMemberRoleParams{
		ProjectID: projectID,
		UserID:    userID,
		Role:      string(role),
	})
}

func (s *ProjectMemberStore) SetMemberships(ctx context.Context, orgID, userID string, assignments []domain.ProjectAssignment) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := s.q.WithTx(tx)

	// Read current memberships for this user.
	currentRows, err := q.ListUserProjectMemberships(ctx, sqlc.ListUserProjectMembershipsParams{
		UserID: userID,
		OrgID:  orgID,
	})
	if err != nil {
		return err
	}

	currentMap := make(map[string]domain.Role)
	for _, row := range currentRows {
		currentMap[row.ProjectID] = domain.Role(row.Role)
	}

	desiredMap := make(map[string]domain.Role)
	for _, a := range assignments {
		desiredMap[a.ProjectID] = a.Role
	}

	// Add or update memberships.
	for projectID, role := range desiredMap {
		if existingRole, exists := currentMap[projectID]; exists {
			if existingRole != role {
				if err := q.UpdateProjectMemberRole(ctx, sqlc.UpdateProjectMemberRoleParams{
					ProjectID: projectID,
					UserID:    userID,
					Role:      string(role),
				}); err != nil {
					return err
				}
			}
		} else {
			if err := q.AddProjectMember(ctx, sqlc.AddProjectMemberParams{
				ProjectID: projectID,
				Role:      string(role),
				ID:        projectID,
				ID_2:      userID,
			}); err != nil {
				return err
			}
		}
	}

	// Remove memberships not in the desired set.
	for projectID := range currentMap {
		if _, keep := desiredMap[projectID]; !keep {
			if err := q.RemoveProjectMember(ctx, sqlc.RemoveProjectMemberParams{
				ProjectID: projectID,
				UserID:    userID,
			}); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (s *ProjectMemberStore) ListByUser(ctx context.Context, orgID, userID string) ([]*domain.UserProjectMembership, error) {
	rows, err := s.q.ListUserProjectMemberships(ctx, sqlc.ListUserProjectMembershipsParams{
		UserID: userID,
		OrgID:  orgID,
	})
	if err != nil {
		return nil, err
	}
	memberships := make([]*domain.UserProjectMembership, len(rows))
	for i, row := range rows {
		memberships[i] = &domain.UserProjectMembership{
			ProjectID: row.ProjectID,
			Name:      row.Name,
			Color:     row.Color,
			Icon:      row.Icon,
			Role:      domain.Role(row.Role),
		}
	}
	return memberships, nil
}
