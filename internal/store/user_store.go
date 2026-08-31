package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

type UserStore struct {
	q  *sqlc.Queries
	db *sql.DB
}

func NewUserStore(q *sqlc.Queries, db *sql.DB) *UserStore {
	return &UserStore{q: q, db: db}
}

// RunInTransaction runs fn against a transaction-scoped UserRepository.
// Membership mutations that must re-check owner counts (last-owner guard,
// owner cap) use this so the check and the write commit atomically.
func (s *UserStore) RunInTransaction(ctx context.Context, fn func(port.UserRepository) error) error {
	if s.db == nil {
		return fn(s)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	txStore := &UserStore{q: s.q.WithTx(tx), db: nil}
	if err := fn(txStore); err != nil {
		return err
	}
	return tx.Commit()
}

var _ port.UserRepository = (*UserStore)(nil)

type cursorData struct {
	N string `json:"n"`
	I string `json:"i"`
}

func encodeCursor(name, id string) string {
	data, _ := json.Marshal(cursorData{N: name, I: id})
	return base64.StdEncoding.EncodeToString(data)
}

func decodeCursor(cursor string) (name, id string, err error) {
	data, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", fmt.Errorf("invalid cursor: %w", err)
	}
	var c cursorData
	if err := json.Unmarshal(data, &c); err != nil {
		return "", "", fmt.Errorf("invalid cursor: %w", err)
	}
	return c.N, c.I, nil
}

func userRowToDomain(
	id, accountID, orgID, email, name, role string,
	avatarURL *string,
	isActive int64,
	createdAt, updatedAt string,
) domain.User {
	return domain.User{
		ID:        id,
		AccountID: accountID,
		OrgID:     orgID,
		Email:     email,
		Name:      name,
		Role:      domain.Role(role),
		AvatarURL: avatarURL,
		IsActive:  isActive != 0,
		CreatedAt: parseTime(createdAt),
		UpdatedAt: parseTime(updatedAt),
	}
}

func (s *UserStore) GetByID(ctx context.Context, orgID, id string) (*domain.User, error) {
	u, err := s.q.GetUserByID(ctx, sqlc.GetUserByIDParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, mapScanErr(err)
	}
	d := userRowToDomain(u.ID, derefStr(u.AccountID), u.OrgID, u.Email, u.Name, u.Role, u.AvatarUrl, u.IsActive, u.CreatedAt, u.UpdatedAt)
	return &d, nil
}

func (s *UserStore) GetByEmail(ctx context.Context, orgID, email string) (*domain.User, error) {
	u, err := s.q.GetUserByEmail(ctx, sqlc.GetUserByEmailParams{OrgID: orgID, Email: email})
	if err != nil {
		return nil, mapScanErr(err)
	}
	d := userRowToDomain(u.ID, derefStr(u.AccountID), u.OrgID, u.Email, u.Name, u.Role, u.AvatarUrl, u.IsActive, u.CreatedAt, u.UpdatedAt)
	return &d, nil
}

func (s *UserStore) ListByAccount(ctx context.Context, accountID string) ([]*domain.User, error) {
	rows, err := s.q.ListUsersByAccount(ctx, &accountID)
	if err != nil {
		return nil, err
	}
	users := make([]*domain.User, len(rows))
	for i, u := range rows {
		d := userRowToDomain(u.ID, derefStr(u.AccountID), u.OrgID, u.Email, u.Name, u.Role, u.AvatarUrl, u.IsActive, u.CreatedAt, u.UpdatedAt)
		users[i] = &d
	}
	return users, nil
}

func (s *UserStore) GetByOrgAndAccount(ctx context.Context, orgID, accountID string) (*domain.User, error) {
	u, err := s.q.GetByOrgAndAccount(ctx, sqlc.GetByOrgAndAccountParams{OrgID: orgID, AccountID: &accountID})
	if err != nil {
		return nil, mapScanErr(err)
	}
	d := userRowToDomain(u.ID, derefStr(u.AccountID), u.OrgID, u.Email, u.Name, u.Role, u.AvatarUrl, u.IsActive, u.CreatedAt, u.UpdatedAt)
	return &d, nil
}

func (s *UserStore) ListUsers(ctx context.Context, orgID string, filter domain.UserFilter) (*domain.UserListResult, error) {
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

	var rolePtr, searchPtr *string
	if filter.Role != "" {
		rolePtr = &filter.Role
	}
	if filter.Search != "" {
		searchPtr = nilIfEmpty(filter.Search)
	}

	rows, err := s.q.ListUsersPaginated(ctx, sqlc.ListUsersPaginatedParams{
		OrgID:           orgID,
		Search:          searchPtr,
		Role:            rolePtr,
		IncludeInactive: boolToInt64(filter.IncludeInactive),
		CursorName:      cursorName,
		CursorID:        cursorID,
		LimitVal:        int64(limit + 1),
	})
	if err != nil {
		return nil, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	users := make([]*domain.User, len(rows))
	for i, row := range rows {
		d := userRowToDomain(row.ID, derefStr(row.AccountID), row.OrgID, row.Email, row.Name, row.Role, row.AvatarUrl, row.IsActive, row.CreatedAt, row.UpdatedAt)
		users[i] = &d
	}

	result := &domain.UserListResult{
		Users:   users,
		HasMore: hasMore,
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		result.NextCursor = encodeCursor(last.Name, last.ID)
	}

	return result, nil
}

func (s *UserStore) Create(ctx context.Context, user *domain.User) error {
	var isActive int64
	if user.IsActive {
		isActive = 1
	}
	return s.q.CreateUser(ctx, sqlc.CreateUserParams{
		ID:        user.ID,
		AccountID: &user.AccountID,
		OrgID:     user.OrgID,
		Email:     user.Email,
		Name:      user.Name,
		Role:      string(user.Role),
		AvatarUrl: user.AvatarURL,
		IsActive:  isActive,
	})
}

func (s *UserStore) Update(ctx context.Context, user *domain.User) error {
	var isActive int64
	if user.IsActive {
		isActive = 1
	}
	return s.q.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:        user.ID,
		OrgID:     user.OrgID,
		Name:      user.Name,
		Email:     user.Email,
		Role:      string(user.Role),
		AvatarUrl: user.AvatarURL,
		IsActive:  isActive,
	})
}

func (s *UserStore) UpdateProfileByAccount(ctx context.Context, accountID, name string, avatarURL *string) error {
	return s.q.UpdateAccountProfile(ctx, sqlc.UpdateAccountProfileParams{
		Name:      name,
		AvatarUrl: avatarURL,
		AccountID: &accountID,
	})
}

func (s *UserStore) UpdateRole(ctx context.Context, orgID, id string, role domain.Role) error {
	return s.q.UpdateUserRole(ctx, sqlc.UpdateUserRoleParams{
		ID:    id,
		Role:  string(role),
		OrgID: orgID,
	})
}

func (s *UserStore) UpdateActive(ctx context.Context, orgID, id string, active bool) error {
	var isActive int64
	if active {
		isActive = 1
	}
	return s.q.UpdateUserActive(ctx, sqlc.UpdateUserActiveParams{
		IsActive: isActive,
		ID:       id,
		OrgID:    orgID,
	})
}

func (s *UserStore) ListByIDs(ctx context.Context, ids []string) ([]*domain.User, error) {
	rows, err := s.q.ListUsersByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	users := make([]*domain.User, len(rows))
	for i, u := range rows {
		d := userRowToDomain(u.ID, derefStr(u.AccountID), u.OrgID, u.Email, u.Name, u.Role, u.AvatarUrl, u.IsActive, u.CreatedAt, u.UpdatedAt)
		users[i] = &d
	}
	return users, nil
}

func (s *UserStore) CountOwners(ctx context.Context, orgID string) (int, error) {
	count, err := s.q.CountOwners(ctx, orgID)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}
