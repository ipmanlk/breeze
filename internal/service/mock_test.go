package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/lexorank"
	"ipmanlk/plume/internal/port"
)

type mockOrgRepo struct {
	exists   bool
	called   int
	orgsByID map[string]*domain.Organization
	// userRepo is set by tests that need userRepo to reflect org+user creation.
	userRepo *mockUserRepo
}

func newMockOrgRepo(exists bool) *mockOrgRepo {
	return &mockOrgRepo{
		exists:   exists,
		orgsByID: make(map[string]*domain.Organization),
	}
}

func (m *mockOrgRepo) GetByID(ctx context.Context, id string) (*domain.Organization, error) {
	m.called++
	if org, ok := m.orgsByID[id]; ok {
		return org, nil
	}
	return nil, errors.New("not found")
}

func (m *mockOrgRepo) GetBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	for _, org := range m.orgsByID {
		if org.Slug == slug {
			return org, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockOrgRepo) Exists(ctx context.Context) (bool, error) {
	m.called++
	return m.exists, nil
}

func (m *mockOrgRepo) Count(ctx context.Context) (int64, error) {
	m.called++
	if m.exists {
		return 1, nil
	}
	return 0, nil
}

func (m *mockOrgRepo) Create(ctx context.Context, org *domain.Organization) error {
	m.called++
	m.orgsByID[org.ID] = org
	m.exists = true
	return nil
}

func (m *mockOrgRepo) Update(ctx context.Context, org *domain.Organization) error {
	m.called++
	if existing, ok := m.orgsByID[org.ID]; ok {
		*existing = *org
	} else {
		m.orgsByID[org.ID] = org
	}
	return nil
}

func (m *mockOrgRepo) Delete(ctx context.Context, id string) error {
	m.called++
	delete(m.orgsByID, id)
	return nil
}

func (m *mockOrgRepo) ListForAccount(ctx context.Context, accountID string) ([]*domain.Workspace, error) {
	var out []*domain.Workspace
	for _, org := range m.orgsByID {
		out = append(out, &domain.Workspace{Organization: *org, Role: domain.RoleOwner, IsOwner: true})
	}
	return out, nil
}

func (m *mockOrgRepo) CreateOrgWithAccountAndUser(ctx context.Context, org *domain.Organization, accountID, userID, passwordHash, adminEmail, adminName string) error {
	m.called++
	m.orgsByID[org.ID] = org
	m.exists = true
	if m.userRepo != nil {
		_ = m.userRepo.Create(ctx, &domain.User{
			ID:        userID,
			AccountID: accountID,
			OrgID:     org.ID,
			Email:     adminEmail,
			Name:      adminName,
			Role:      domain.RoleOwner,
			IsActive:  true,
		})
	}
	return nil
}

func (m *mockOrgRepo) CreateOrgWithUser(ctx context.Context, org *domain.Organization, userID, accountID, displayName, email string, avatarURL *string) error {
	m.called++
	m.orgsByID[org.ID] = org
	m.exists = true
	if m.userRepo != nil {
		_ = m.userRepo.Create(ctx, &domain.User{
			ID:        userID,
			AccountID: accountID,
			OrgID:     org.ID,
			Email:     email,
			Name:      displayName,
			Role:      domain.RoleOwner,
			AvatarURL: avatarURL,
			IsActive:  true,
		})
	}
	return nil
}

type mockUserRepo struct {
	usersByID    map[string]*domain.User
	usersByEmail map[string]*domain.User
	createdUser  *domain.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		usersByID:    make(map[string]*domain.User),
		usersByEmail: make(map[string]*domain.User),
	}
}

// seedUser inserts a user into the mock repo for a given org. Used by
// conversation/DM tests that now require org-membership validation.
func seedUser(repo *mockUserRepo, id, orgID string) {
	_ = repo.Create(context.Background(), &domain.User{ID: id, OrgID: orgID, Email: id + "@test.com", Name: id, IsActive: true})
}

func (m *mockUserRepo) GetByID(ctx context.Context, orgID, id string) (*domain.User, error) {
	u, ok := m.usersByID[id]
	if !ok || u.OrgID != orgID {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, orgID, email string) (*domain.User, error) {
	u, ok := m.usersByEmail[email]
	if !ok || u.OrgID != orgID {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (m *mockUserRepo) ListByAccount(ctx context.Context, accountID string) ([]*domain.User, error) {
	var out []*domain.User
	for _, u := range m.usersByID {
		if u.AccountID == accountID {
			out = append(out, u)
		}
	}
	return out, nil
}

func (m *mockUserRepo) GetByOrgAndAccount(ctx context.Context, orgID, accountID string) (*domain.User, error) {
	for _, u := range m.usersByID {
		if u.OrgID == orgID && u.AccountID == accountID {
			return u, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockUserRepo) ListUsers(ctx context.Context, orgID string, filter domain.UserFilter) (*domain.UserListResult, error) {
	var users []*domain.User
	for _, u := range m.usersByID {
		if u.OrgID == orgID {
			if filter.Search != "" && !strings.Contains(strings.ToLower(u.Name), strings.ToLower(filter.Search)) {
				continue
			}
			users = append(users, u)
		}
	}
	return &domain.UserListResult{Users: users, HasMore: false}, nil
}

func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error {
	u := *user
	m.usersByID[u.ID] = &u
	m.usersByEmail[u.Email] = &u
	m.createdUser = &u
	return nil
}

func (m *mockUserRepo) Update(ctx context.Context, user *domain.User) error {
	return nil
}

func (m *mockUserRepo) UpdateProfileByAccount(ctx context.Context, accountID, name string, avatarURL *string) error {
	for _, u := range m.usersByID {
		if u.AccountID == accountID {
			u.Name = name
			u.AvatarURL = avatarURL
		}
	}
	return nil
}

func (m *mockUserRepo) UpdateRole(ctx context.Context, orgID, id string, role domain.Role) error {
	u, ok := m.usersByID[id]
	if !ok {
		return errors.New("not found")
	}
	u.Role = role
	return nil
}

func (m *mockUserRepo) UpdateActive(ctx context.Context, orgID, id string, active bool) error {
	u, ok := m.usersByID[id]
	if !ok {
		return errors.New("not found")
	}
	u.IsActive = active
	return nil
}

func (m *mockUserRepo) RunInTransaction(ctx context.Context, fn func(port.UserRepository) error) error {
	return fn(m)
}

func (m *mockUserRepo) CountOwners(ctx context.Context, orgID string) (int, error) {
	count := 0
	for _, u := range m.usersByID {
		if u.OrgID == orgID && u.Role == domain.RoleOwner && u.IsActive {
			count++
		}
	}
	return count, nil
}

func (m *mockUserRepo) ListByIDs(ctx context.Context, ids []string) ([]*domain.User, error) {
	var result []*domain.User
	for _, id := range ids {
		if u, ok := m.usersByID[id]; ok {
			result = append(result, u)
		}
	}
	return result, nil
}

// mockAccountRepo is a port.AccountRepository for service tests.
type mockAccountRepo struct {
	accountsByEmail   map[string]*domain.Account
	accountsByID      map[string]*domain.Account
	created           *domain.Account
	updatePasswordErr error // when non-nil, UpdatePassword returns this error
}

func newMockAccountRepo() *mockAccountRepo {
	return &mockAccountRepo{
		accountsByEmail: make(map[string]*domain.Account),
		accountsByID:    make(map[string]*domain.Account),
	}
}

func (m *mockAccountRepo) GetByEmail(ctx context.Context, email string) (*domain.Account, error) {
	a, ok := m.accountsByEmail[email]
	if !ok {
		return nil, errors.New("not found")
	}
	return a, nil
}

func (m *mockAccountRepo) GetByID(ctx context.Context, id string) (*domain.Account, error) {
	a, ok := m.accountsByID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return a, nil
}

func (m *mockAccountRepo) Create(ctx context.Context, account *domain.Account) error {
	a := *account
	m.accountsByEmail[a.Email] = &a
	m.accountsByID[a.ID] = &a
	m.created = &a
	return nil
}

func (m *mockAccountRepo) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	if m.updatePasswordErr != nil {
		return m.updatePasswordErr
	}
	a, ok := m.accountsByID[id]
	if !ok {
		return errors.New("not found")
	}
	a.PasswordHash = passwordHash
	return nil
}

func (m *mockAccountRepo) Exists(ctx context.Context) (bool, error) {
	return len(m.accountsByID) > 0, nil
}

type mockProjectRepo struct {
	projectsByID map[string]*domain.Project
	createdIDs   []string
}

func newMockProjectRepo() *mockProjectRepo {
	return &mockProjectRepo{projectsByID: make(map[string]*domain.Project)}
}

func (m *mockProjectRepo) List(ctx context.Context, orgID string) ([]*domain.Project, error) {
	var projects []*domain.Project
	for _, p := range m.projectsByID {
		if p.OrgID == orgID && !p.IsArchived {
			projects = append(projects, p)
		}
	}
	return projects, nil
}

func (m *mockProjectRepo) ListIncludingArchived(ctx context.Context, orgID string) ([]*domain.Project, error) {
	var projects []*domain.Project
	for _, p := range m.projectsByID {
		if p.OrgID == orgID {
			projects = append(projects, p)
		}
	}
	return projects, nil
}

func (m *mockProjectRepo) ListForUser(ctx context.Context, orgID, userID string) ([]*domain.Project, error) {
	return m.List(ctx, orgID)
}

func (m *mockProjectRepo) ListForUserIncludingArchived(ctx context.Context, orgID, userID string) ([]*domain.Project, error) {
	return m.ListIncludingArchived(ctx, orgID)
}

func (m *mockProjectRepo) GetByID(ctx context.Context, orgID, id string) (*domain.Project, error) {
	if p, ok := m.projectsByID[id]; ok && p.OrgID == orgID {
		return p, nil
	}
	return nil, errors.New("not found")
}

func (m *mockProjectRepo) GetBySlug(ctx context.Context, orgID, slug string) (*domain.Project, error) {
	for _, p := range m.projectsByID {
		if p.OrgID == orgID && p.Slug == slug {
			return p, nil
		}
	}
	return nil, apperr.ErrNotFound
}

func (m *mockProjectRepo) Create(ctx context.Context, p *domain.Project) error {
	m.projectsByID[p.ID] = p
	m.createdIDs = append(m.createdIDs, p.ID)
	return nil
}

func (m *mockProjectRepo) Update(ctx context.Context, p *domain.Project) error {
	m.projectsByID[p.ID] = p
	return nil
}

func (m *mockProjectRepo) ListByIDs(ctx context.Context, orgID string, ids []string) ([]*domain.Project, error) {
	var result []*domain.Project
	for _, id := range ids {
		if p, ok := m.projectsByID[id]; ok && p.OrgID == orgID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockProjectRepo) Delete(ctx context.Context, orgID, id string) error {
	delete(m.projectsByID, id)
	return nil
}

func (m *mockProjectRepo) SetArchived(_ context.Context, orgID, id string, archived bool) error {
	if p, ok := m.projectsByID[id]; ok && p.OrgID == orgID {
		p.IsArchived = archived
	}
	return nil
}

func (m *mockProjectRepo) CreateWithStatuses(_ context.Context, project *domain.Project, statuses []*domain.TaskStatus) error {
	for _, st := range statuses {
		st.ProjectID = project.ID
	}
	m.projectsByID[project.ID] = project
	return nil
}

type mockTaskStatusRepo struct {
	statusesByID      map[string]*domain.TaskStatus
	projectIDs        map[string]string
	taskCountByStatus map[string]int64 // statusID -> count
}

func newMockTaskStatusRepo() *mockTaskStatusRepo {
	return &mockTaskStatusRepo{
		statusesByID:      make(map[string]*domain.TaskStatus),
		projectIDs:        make(map[string]string),
		taskCountByStatus: make(map[string]int64),
	}
}

func (m *mockTaskStatusRepo) ListByProject(ctx context.Context, projectID string) ([]*domain.TaskStatus, error) {
	var statuses []*domain.TaskStatus
	for _, s := range m.statusesByID {
		if s.ProjectID == projectID {
			statuses = append(statuses, s)
		}
	}
	return statuses, nil
}

func (m *mockTaskStatusRepo) GetByID(ctx context.Context, id string) (*domain.TaskStatus, error) {
	if s, ok := m.statusesByID[id]; ok {
		return s, nil
	}
	return nil, errors.New("not found")
}

func (m *mockTaskStatusRepo) Create(ctx context.Context, s *domain.TaskStatus) error {
	m.statusesByID[s.ID] = s
	m.projectIDs[s.ID] = s.ProjectID
	return nil
}

func (m *mockTaskStatusRepo) Update(ctx context.Context, s *domain.TaskStatus) error {
	m.statusesByID[s.ID] = s
	return nil
}

func (m *mockTaskStatusRepo) CountTasksByStatus(ctx context.Context, statusID, projectID string) (int64, error) {
	return m.taskCountByStatus[statusID], nil
}

func (m *mockTaskStatusRepo) Delete(ctx context.Context, id, projectID string) error {
	delete(m.statusesByID, id)
	return nil
}

func (m *mockTaskStatusRepo) ReassignTasks(ctx context.Context, toStatusID, fromStatusID, projectID string) error {
	return nil
}

type mockCycleRepo struct {
	cyclesByID map[string]*domain.Cycle
	taskRepo   *mockTaskRepo // optional; set by tests that verify task movement in CompleteCycle
}

func newMockCycleRepo() *mockCycleRepo {
	return &mockCycleRepo{cyclesByID: make(map[string]*domain.Cycle)}
}

func (m *mockCycleRepo) ListByProject(ctx context.Context, projectID string) ([]*domain.Cycle, error) {
	var cycles []*domain.Cycle
	for _, c := range m.cyclesByID {
		if c.ProjectID == projectID {
			cycles = append(cycles, c)
		}
	}
	return cycles, nil
}

func (m *mockCycleRepo) GetByID(ctx context.Context, id, projectID string) (*domain.Cycle, error) {
	if c, ok := m.cyclesByID[id]; ok && c.ProjectID == projectID {
		return c, nil
	}
	return nil, errors.New("not found")
}

func (m *mockCycleRepo) Create(ctx context.Context, c *domain.Cycle) error {
	m.cyclesByID[c.ID] = c
	return nil
}

func (m *mockCycleRepo) Update(ctx context.Context, c *domain.Cycle) error {
	m.cyclesByID[c.ID] = c
	return nil
}

func (m *mockCycleRepo) Delete(ctx context.Context, id, projectID string) error {
	delete(m.cyclesByID, id)
	return nil
}

func (m *mockCycleRepo) GetActiveByProject(ctx context.Context, projectID string) (*domain.Cycle, error) {
	for _, c := range m.cyclesByID {
		if c.ProjectID == projectID && c.IsActive {
			return c, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockCycleRepo) DeactivateAll(ctx context.Context, projectID string) error {
	for _, c := range m.cyclesByID {
		if c.ProjectID == projectID {
			c.IsActive = false
		}
	}
	return nil
}

func (m *mockCycleRepo) SetActive(ctx context.Context, id, projectID string) error {
	if c, ok := m.cyclesByID[id]; ok && c.ProjectID == projectID {
		c.IsActive = true
		return nil
	}
	return errors.New("not found")
}

func (m *mockCycleRepo) ActivateCycle(ctx context.Context, id, projectID string) error {
	if err := m.DeactivateAll(ctx, projectID); err != nil {
		return err
	}
	return m.SetActive(ctx, id, projectID)
}

func (m *mockCycleRepo) CountTasksByCycle(ctx context.Context, id string) (total, completed int64, err error) {
	return 0, 0, nil
}

func (m *mockCycleRepo) CountTasksByCycles(ctx context.Context, projectID string) (map[string]domain.CycleTaskCount, error) {
	return map[string]domain.CycleTaskCount{}, nil
}

func (m *mockCycleRepo) CompleteCycle(ctx context.Context, plan domain.CycleCompletionPlan) error {
	// Create the new auto-generated cycle if one is provided.
	if plan.NewCycle != nil {
		m.cyclesByID[plan.NewCycle.ID] = plan.NewCycle
	}

	// Deactivate all cycles for this project.
	for _, c := range m.cyclesByID {
		if c.ProjectID == plan.ProjectID {
			c.IsActive = false
		}
	}

	// Move incomplete tasks to target cycle, or unassign them.
	if m.taskRepo != nil {
		for _, t := range m.taskRepo.tasksByID {
			if t.OrgID == plan.OrgID && t.CycleID != nil && *t.CycleID == plan.CompletedCycleID {
				if t.CompletedAt == nil {
					// Incomplete task: move or unassign
					if plan.MoveTargetCycleID != "" {
						t.CycleID = &plan.MoveTargetCycleID
					} else {
						t.CycleID = nil
					}
				}
			}
		}
	}

	// Set the target cycle as active.
	if plan.SetActiveCycleID != "" {
		if c, ok := m.cyclesByID[plan.SetActiveCycleID]; ok && c.ProjectID == plan.ProjectID {
			c.IsActive = true
		}
	}

	// Update the completed cycle.
	if c, ok := m.cyclesByID[plan.CompletedCycleID]; ok && c.ProjectID == plan.ProjectID {
		c.IsCompleted = plan.CompletedCycle.IsCompleted
		c.IsActive = plan.CompletedCycle.IsActive
		c.UpdatedAt = plan.CompletedCycle.UpdatedAt
	}

	return nil
}

type mockTaskRepo struct {
	tasksByID map[string]*domain.Task
}

func newMockTaskRepo() *mockTaskRepo {
	return &mockTaskRepo{tasksByID: make(map[string]*domain.Task)}
}

func (m *mockTaskRepo) ListByProject(ctx context.Context, orgID, projectID string, filter domain.TaskFilter) ([]*domain.Task, error) {
	var tasks []*domain.Task
	for _, t := range m.tasksByID {
		if t.ProjectID == projectID && t.OrgID == orgID {
			if filter.StatusID != nil && t.StatusID != *filter.StatusID {
				continue
			}
			tasks = append(tasks, t)
		}
	}
	return tasks, nil
}

func (m *mockTaskRepo) GetByID(ctx context.Context, orgID, id, projectID string) (*domain.Task, error) {
	if t, ok := m.tasksByID[id]; ok && t.ProjectID == projectID && t.OrgID == orgID {
		return t, nil
	}
	return nil, errors.New("not found")
}

func (m *mockTaskRepo) GetByIDAndOrg(ctx context.Context, orgID, id string) (*domain.Task, error) {
	if t, ok := m.tasksByID[id]; ok && t.OrgID == orgID {
		return t, nil
	}
	return nil, errors.New("not found")
}

func (m *mockTaskRepo) ListSubtasks(_ context.Context, orgID, parentID string) ([]*domain.Task, error) {
	var out []*domain.Task
	for _, t := range m.tasksByID {
		if t.OrgID == orgID && t.ParentID != nil && *t.ParentID == parentID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *mockTaskRepo) Create(ctx context.Context, t *domain.Task) error {
	m.tasksByID[t.ID] = t
	return nil
}

func (m *mockTaskRepo) Update(ctx context.Context, t *domain.Task) error {
	m.tasksByID[t.ID] = t
	return nil
}

func (m *mockTaskRepo) Move(ctx context.Context, orgID, id, projectID, statusID, positionKey string) error {
	t, ok := m.tasksByID[id]
	if !ok {
		return errors.New("not found")
	}
	t.StatusID = statusID
	t.PositionKey = positionKey
	return nil
}

func (m *mockTaskRepo) MoveToProject(ctx context.Context, orgID, id, fromProjectID, toProjectID, toStatusID, positionKey string, completedAt *time.Time) error {
	t, ok := m.tasksByID[id]
	if !ok {
		return errors.New("not found")
	}
	t.ProjectID = toProjectID
	t.StatusID = toStatusID
	t.PositionKey = positionKey
	t.CycleID = nil
	t.ParentID = nil
	t.CompletedAt = completedAt
	return nil
}

func (m *mockTaskRepo) GetLastPositionKey(ctx context.Context, orgID, projectID, statusID string) (string, error) {
	var lastKey string
	for _, t := range m.tasksByID {
		if t.ProjectID == projectID && t.StatusID == statusID {
			if t.PositionKey > lastKey {
				lastKey = t.PositionKey
			}
		}
	}
	return lastKey, nil
}

func (m *mockTaskRepo) GeneratePositionKey(ctx context.Context, orgID, projectID, statusID string) (string, error) {
	lastKey, err := m.GetLastPositionKey(ctx, orgID, projectID, statusID)
	if err != nil {
		return "", err
	}
	return lexorank.GenerateKeyBetween(lastKey, "")
}

func (m *mockTaskRepo) GetPositionKeyNeighbors(ctx context.Context, orgID, taskID string) (prev, next string, err error) {
	t, ok := m.tasksByID[taskID]
	if !ok {
		return "", "", errors.New("not found")
	}
	var prevKey, nextKey string
	for _, other := range m.tasksByID {
		if other.ProjectID != t.ProjectID || other.StatusID != t.StatusID {
			continue
		}
		if other.PositionKey < t.PositionKey && other.PositionKey > prevKey {
			prevKey = other.PositionKey
		}
		if other.PositionKey > t.PositionKey && (nextKey == "" || other.PositionKey < nextKey) {
			nextKey = other.PositionKey
		}
	}
	return prevKey, nextKey, nil
}

func (m *mockTaskRepo) ListByIDs(ctx context.Context, orgID string, ids []string) ([]*domain.Task, error) {
	var result []*domain.Task
	for _, id := range ids {
		if t, ok := m.tasksByID[id]; ok && t.OrgID == orgID {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockTaskRepo) ListByIDsFull(ctx context.Context, orgID string, ids []string) ([]*domain.Task, error) {
	return m.ListByIDs(ctx, orgID, ids)
}

func (m *mockTaskRepo) Delete(ctx context.Context, orgID, id, projectID string) error {
	delete(m.tasksByID, id)
	return nil
}

func (m *mockTaskRepo) DeleteSubtasks(_ context.Context, orgID, parentID string) error {
	for id, t := range m.tasksByID {
		if t.OrgID == orgID && t.ParentID != nil && *t.ParentID == parentID {
			delete(m.tasksByID, id)
		}
	}
	return nil
}

func (m *mockTaskRepo) PromoteSubtasks(_ context.Context, orgID, parentID string) error {
	for _, t := range m.tasksByID {
		if t.OrgID == orgID && t.ParentID != nil && *t.ParentID == parentID {
			t.ParentID = nil
		}
	}
	return nil
}

func (m *mockTaskRepo) CountSubtasks(_ context.Context, orgID, parentID string) (int64, error) {
	var n int64
	for _, t := range m.tasksByID {
		if t.OrgID == orgID && t.ParentID != nil && *t.ParentID == parentID {
			n++
		}
	}
	return n, nil
}

func (m *mockTaskRepo) GetLastSubtaskPosition(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func (m *mockTaskRepo) GenerateSubtaskPositionKey(_ context.Context, _, _ string) (string, error) {
	return lexorank.GenerateKeyBetween("", "")
}

// RunInTransaction runs fn against the mock itself (no real transaction;
// tests don't need rollback semantics, just the call-through behavior).
func (m *mockTaskRepo) RunInTransaction(_ context.Context, fn func(port.TaskRepository) error) error {
	return fn(m)
}

func (m *mockTaskRepo) ReorderSubtasks(_ context.Context, _, _ string, _ []domain.ReorderOp) error {
	return nil
}

func (m *mockTaskRepo) SetAssignees(ctx context.Context, taskID string, userIDs []string) error {
	return nil
}

func (m *mockTaskRepo) ListAssigneesByTaskIDs(ctx context.Context, taskIDs []string) (map[string][]domain.TaskAssignee, error) {
	return nil, nil
}

func (m *mockTaskRepo) UnassignCycleFromTasks(ctx context.Context, orgID, cycleID string) error {
	for _, t := range m.tasksByID {
		if t.CycleID != nil && *t.CycleID == cycleID {
			t.CycleID = nil
		}
	}
	return nil
}

func (m *mockTaskRepo) MoveIncompleteTasksToCycle(ctx context.Context, orgID, fromCycleID, toCycleID string) error {
	for _, t := range m.tasksByID {
		if t.CycleID != nil && *t.CycleID == fromCycleID && t.CompletedAt == nil {
			t.CycleID = &toCycleID
		}
	}
	return nil
}

func (m *mockTaskRepo) UnassignCycleFromIncompleteTasks(ctx context.Context, orgID, cycleID string) error {
	for _, t := range m.tasksByID {
		if t.CycleID != nil && *t.CycleID == cycleID && t.CompletedAt == nil {
			t.CycleID = nil
		}
	}
	return nil
}

func (m *mockTaskRepo) MoveTasksToCycle(ctx context.Context, orgID, fromCycleID, toCycleID string) error {
	for _, t := range m.tasksByID {
		if t.CycleID != nil && *t.CycleID == fromCycleID {
			t.CycleID = &toCycleID
		}
	}
	return nil
}

func (m *mockTaskRepo) ListByCycle(ctx context.Context, orgID, cycleID string) ([]*domain.Task, error) {
	var tasks []*domain.Task
	for _, t := range m.tasksByID {
		if t.CycleID != nil && *t.CycleID == cycleID {
			tasks = append(tasks, t)
		}
	}
	return tasks, nil
}

func (m *mockTaskRepo) ListByUser(ctx context.Context, orgID, userID string, filter domain.TaskListFilter) (*domain.TaskListResult, error) {
	return &domain.TaskListResult{}, nil
}

type mockNotificationService struct {
	notifications []*domain.Notification
	preferences   map[domain.NotificationType]bool
	dueRows       []domain.DueTaskRow
	notifyFn      func(ctx context.Context, orgID, recipientID string, notifType domain.NotificationType, title, body, link, entityType, entityID, actorID string) error
	processFn     func(ctx context.Context) error
}

func newMockNotificationService() *mockNotificationService {
	return &mockNotificationService{
		preferences: make(map[domain.NotificationType]bool),
	}
}

func (m *mockNotificationService) List(ctx context.Context, orgID, userID string, filter domain.NotificationFilter) (*domain.NotificationListResult, error) {
	return &domain.NotificationListResult{}, nil
}

func (m *mockNotificationService) CountUnread(ctx context.Context, userID string) (int, error) {
	return 0, nil
}

func (m *mockNotificationService) MarkRead(ctx context.Context, id, userID string) error {
	return nil
}

func (m *mockNotificationService) MarkAllRead(ctx context.Context, userID string) error {
	return nil
}

func (m *mockNotificationService) GetPreferences(ctx context.Context, userID string) ([]*domain.NotificationPreference, error) {
	return nil, nil
}

func (m *mockNotificationService) SetPreference(ctx context.Context, userID string, notifType domain.NotificationType, enabled bool) error {
	return nil
}

func (m *mockNotificationService) Notify(ctx context.Context, orgID, recipientID string, notifType domain.NotificationType, title, body, link, entityType, entityID, actorID string) error {
	if m.notifyFn != nil {
		return m.notifyFn(ctx, orgID, recipientID, notifType, title, body, link, entityType, entityID, actorID)
	}
	m.notifications = append(m.notifications, &domain.Notification{
		OrgID:      orgID,
		UserID:     recipientID,
		Type:       notifType,
		Title:      title,
		Body:       body,
		Link:       link,
		EntityType: entityType,
		EntityID:   entityID,
		ActorID:    actorID,
	})
	return nil
}

func (m *mockNotificationService) ProcessDueNotifications(ctx context.Context) error {
	if m.processFn != nil {
		return m.processFn(ctx)
	}
	return nil
}

type mockBroadcaster struct {
	messages []struct {
		roomKey, eventType string
		payload            any
	}
}

func newMockBroadcaster() *mockBroadcaster {
	return &mockBroadcaster{}
}

func (m *mockBroadcaster) Broadcast(roomKey string, eventType string, payload any) error {
	m.messages = append(m.messages, struct {
		roomKey, eventType string
		payload            any
	}{roomKey, eventType, payload})
	return nil
}

// broadcastCount returns the number of broadcast calls since the last reset.
func (m *mockBroadcaster) broadcastCount() int { return len(m.messages) }

// reset clears recorded broadcasts.
func (m *mockBroadcaster) reset() { m.messages = nil }

type mockNotifRepo struct {
	notifications []*domain.Notification
}

func newMockNotifRepo() *mockNotifRepo {
	return &mockNotifRepo{}
}

func (m *mockNotifRepo) Create(ctx context.Context, n *domain.Notification) error {
	m.notifications = append(m.notifications, n)
	return nil
}

func (m *mockNotifRepo) List(ctx context.Context, orgID, userID string, filter domain.NotificationFilter) (*domain.NotificationListResult, error) {
	return &domain.NotificationListResult{}, nil
}

func (m *mockNotifRepo) GetByID(ctx context.Context, id, userID string) (*domain.Notification, error) {
	return nil, nil
}

func (m *mockNotifRepo) MarkRead(ctx context.Context, id, userID string) error {
	return nil
}

func (m *mockNotifRepo) MarkAllRead(ctx context.Context, userID string) error {
	return nil
}

func (m *mockNotifRepo) CountUnread(ctx context.Context, userID string) (int64, error) {
	return 0, nil
}

type mockNotifPrefRepo struct {
	prefs       map[string]bool
	dueTaskRows []domain.DueTaskRow
}

func newMockNotifPrefRepo() *mockNotifPrefRepo {
	return &mockNotifPrefRepo{prefs: make(map[string]bool)}
}

func (m *mockNotifPrefRepo) List(ctx context.Context, userID string) ([]*domain.NotificationPreference, error) {
	var prefs []*domain.NotificationPreference
	for t := range m.prefs {
		prefs = append(prefs, &domain.NotificationPreference{Type: domain.NotificationType(t), Enabled: m.prefs[t]})
	}
	return prefs, nil
}

func (m *mockNotifPrefRepo) GetByType(ctx context.Context, userID string, notifType domain.NotificationType) (*domain.NotificationPreference, error) {
	enabled, ok := m.prefs[string(notifType)]
	if !ok {
		return nil, apperr.ErrNotFound
	}
	return &domain.NotificationPreference{Type: notifType, Enabled: enabled}, nil
}

func (m *mockNotifPrefRepo) Set(ctx context.Context, userID, notifType string, enabled bool) error {
	m.prefs[notifType] = enabled
	return nil
}

func (m *mockNotifPrefRepo) FindDueNotifications(ctx context.Context, nowMinus1h, now, nowPlus24h time.Time, dueSoonType, overdueType string) ([]domain.DueTaskRow, error) {
	if m.dueTaskRows != nil {
		return m.dueTaskRows, nil
	}
	return nil, nil
}

type mockConversationRepo struct {
	convsByID map[string]*domain.Conversation
	members   map[string][]*domain.ConversationMember
}

func newMockConversationRepo() *mockConversationRepo {
	return &mockConversationRepo{
		convsByID: make(map[string]*domain.Conversation),
		members:   make(map[string][]*domain.ConversationMember),
	}
}

func (m *mockConversationRepo) ListByUser(ctx context.Context, orgID, userID string, filter domain.ConversationFilter) (*domain.ConversationListResult, error) {
	var items []*domain.Conversation
	for _, c := range m.convsByID {
		if c.OrgID != orgID {
			continue
		}
		for _, mem := range m.members[c.ID] {
			if mem.UserID == userID {
				items = append(items, c)
				break
			}
		}
	}
	return &domain.ConversationListResult{Items: items}, nil
}

func (m *mockConversationRepo) ListByParent(ctx context.Context, orgID, parentID, userID string, includeProjectLinked bool) ([]*domain.Conversation, error) {
	var items []*domain.Conversation
	for _, c := range m.convsByID {
		if c.OrgID == orgID && c.ParentID != nil && *c.ParentID == parentID {
			items = append(items, c)
		}
	}
	return items, nil
}

func (m *mockConversationRepo) GetByID(ctx context.Context, orgID, id string) (*domain.Conversation, error) {
	c, ok := m.convsByID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	if orgID != "" && c.OrgID != orgID {
		return nil, errors.New("not found")
	}
	return c, nil
}

func (m *mockConversationRepo) GetByIDWithMember(ctx context.Context, orgID, id, userID string) (*domain.Conversation, error) {
	c, ok := m.convsByID[id]
	if !ok || c.OrgID != orgID {
		return nil, errors.New("not found")
	}
	for _, mem := range m.members[c.ID] {
		if mem.UserID == userID {
			return c, nil
		}
	}
	return nil, errors.New("not a member")
}

func (m *mockConversationRepo) GetDMByUsers(ctx context.Context, orgID, requesterID, recipientID string) (*domain.Conversation, error) {
	for _, c := range m.convsByID {
		if c.OrgID != orgID || c.Type != domain.ConvDirect {
			continue
		}
		users := make(map[string]bool)
		for _, mem := range m.members[c.ID] {
			users[mem.UserID] = true
		}
		if len(users) == 2 && users[requesterID] && users[recipientID] {
			return c, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockConversationRepo) Create(ctx context.Context, conv *domain.Conversation) error {
	cp := *conv
	m.convsByID[cp.ID] = &cp
	return nil
}

func (m *mockConversationRepo) Update(ctx context.Context, conv *domain.Conversation) error {
	m.convsByID[conv.ID] = conv
	return nil
}

func (m *mockConversationRepo) UpdateParent(ctx context.Context, orgID, id string, parentID *string, positionKey string) error {
	if c, ok := m.convsByID[id]; ok {
		c.ParentID = parentID
		c.PositionKey = positionKey
	}
	return nil
}

func (m *mockConversationRepo) UpdatePositionKey(ctx context.Context, orgID, id string, positionKey string) error {
	if c, ok := m.convsByID[id]; ok {
		c.PositionKey = positionKey
	}
	return nil
}

func (m *mockConversationRepo) ListCategories(ctx context.Context, orgID string) ([]*domain.Conversation, error) {
	var cats []*domain.Conversation
	for _, c := range m.convsByID {
		if c.OrgID == orgID && c.Type == domain.ConvCategory && c.DeletedAt == nil {
			cats = append(cats, c)
		}
	}
	slices.SortFunc(cats, func(a, b *domain.Conversation) int {
		if a.PositionKey < b.PositionKey {
			return -1
		}
		if a.PositionKey > b.PositionKey {
			return 1
		}
		return 0
	})
	return cats, nil
}

func (m *mockConversationRepo) ListSiblingPositionKeys(ctx context.Context, orgID string, parentID *string) ([]string, error) {
	if parentID == nil {
		cats, _ := m.ListCategories(ctx, orgID)
		keys := make([]string, 0, len(cats))
		for _, c := range cats {
			keys = append(keys, c.PositionKey)
		}
		return keys, nil
	}
	var keys []string
	for _, c := range m.convsByID {
		if c.OrgID == orgID && c.ParentID != nil && *c.ParentID == *parentID &&
			(c.Type == domain.ConvChannel || c.Type == domain.ConvVoice) && c.DeletedAt == nil {
			keys = append(keys, c.PositionKey)
		}
	}
	slices.Sort(keys)
	return keys, nil
}

func (m *mockConversationRepo) Delete(ctx context.Context, orgID, id string) error {
	now := time.Now()
	if c, ok := m.convsByID[id]; ok {
		c.DeletedAt = &now
	}
	return nil
}

func (m *mockConversationRepo) SoftDeleteByParent(ctx context.Context, orgID, parentID string) error {
	now := time.Now()
	for _, c := range m.convsByID {
		if c.ParentID != nil && *c.ParentID == parentID && c.OrgID == orgID {
			c.DeletedAt = &now
		}
	}
	return nil
}

func (m *mockConversationRepo) AddMember(ctx context.Context, orgID, convID, userID string) error {
	m.members[convID] = append(m.members[convID], &domain.ConversationMember{
		ConversationID: convID,
		UserID:         userID,
		OrgID:          orgID,
	})
	return nil
}

func (m *mockConversationRepo) RemoveMember(ctx context.Context, convID, userID string) error {
	var updated []*domain.ConversationMember
	for _, mem := range m.members[convID] {
		if mem.UserID != userID {
			updated = append(updated, mem)
		}
	}
	m.members[convID] = updated
	return nil
}

func (m *mockConversationRepo) GetMembers(ctx context.Context, convID string) ([]*domain.ConversationMember, error) {
	members := m.members[convID]
	var result []*domain.ConversationMember
	for _, mem := range members {
		cp := *mem
		result = append(result, &cp)
	}
	return result, nil
}

func (m *mockConversationRepo) IsMember(ctx context.Context, convID, userID string) (bool, error) {
	for _, mem := range m.members[convID] {
		if mem.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockConversationRepo) UpdateReadState(ctx context.Context, convID, userID string) error {
	return nil
}

func (m *mockConversationRepo) UnreadCount(ctx context.Context, convID, userID string) (int, error) {
	return 0, nil
}

func (m *mockConversationRepo) UnreadCounts(ctx context.Context, userID string, convIDs []string) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *mockConversationRepo) GetLastMessage(ctx context.Context, convID string) (*domain.Message, error) {
	return nil, errors.New("no messages")
}

func (m *mockConversationRepo) ListPinnedMessages(ctx context.Context, convID string, limit int) ([]*domain.Message, error) {
	return nil, nil
}

func (m *mockConversationRepo) CountMembers(ctx context.Context, convID string) (int, error) {
	return len(m.members[convID]), nil
}

func (m *mockConversationRepo) ListByIDs(ctx context.Context, orgID string, ids []string) ([]*domain.Conversation, error) {
	var result []*domain.Conversation
	for _, id := range ids {
		if c, ok := m.convsByID[id]; ok && c.OrgID == orgID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockConversationRepo) CreateWithMembers(_ context.Context, conv *domain.Conversation, memberIDs []string) error {
	if err := m.Create(context.Background(), conv); err != nil {
		return err
	}
	for _, memberID := range memberIDs {
		if err := m.AddMember(context.Background(), conv.OrgID, conv.ID, memberID); err != nil {
			return err
		}
	}
	return nil
}

type mockMessageRepo struct {
	msgsByID  map[string]*domain.Message
	searchErr error
}

func newMockMessageRepo() *mockMessageRepo {
	return &mockMessageRepo{msgsByID: make(map[string]*domain.Message)}
}

func (m *mockMessageRepo) ListByConversation(ctx context.Context, orgID, convID string, filter domain.MessageFilter) (*domain.MessageListResult, error) {
	var items []*domain.Message
	for _, msg := range m.msgsByID {
		if msg.ConversationID == convID && msg.DeletedAt == nil {
			items = append(items, msg)
		}
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return &domain.MessageListResult{Items: items}, nil
}

func (m *mockMessageRepo) ListReplies(ctx context.Context, orgID, convID, parentID string, filter domain.MessageFilter) (*domain.MessageListResult, error) {
	var items []*domain.Message
	for _, msg := range m.msgsByID {
		if msg.ConversationID == convID && msg.ParentID != nil && *msg.ParentID == parentID && msg.DeletedAt == nil {
			items = append(items, msg)
		}
	}
	return &domain.MessageListResult{Items: items}, nil
}

func (m *mockMessageRepo) GetByID(ctx context.Context, id, convID string) (*domain.Message, error) {
	msg, ok := m.msgsByID[id]
	if !ok || msg.ConversationID != convID || msg.DeletedAt != nil {
		return nil, errors.New("not found")
	}
	return msg, nil
}

func (m *mockMessageRepo) GetByIDAnyConv(ctx context.Context, id string) (*domain.Message, error) {
	msg, ok := m.msgsByID[id]
	if !ok || msg.DeletedAt != nil {
		return nil, errors.New("not found")
	}
	return msg, nil
}

func (m *mockMessageRepo) Create(ctx context.Context, msg *domain.Message) error {
	cp := *msg
	m.msgsByID[cp.ID] = &cp
	return nil
}

func (m *mockMessageRepo) Update(ctx context.Context, msg *domain.Message) error {
	m.msgsByID[msg.ID] = msg
	return nil
}

func (m *mockMessageRepo) SoftDelete(ctx context.Context, id, convID string) error {
	msg, ok := m.msgsByID[id]
	if ok {
		now := time.Now()
		msg.DeletedAt = &now
	}
	return nil
}

func (m *mockMessageRepo) Pin(ctx context.Context, id, convID, pinnedBy string) error {
	msg, ok := m.msgsByID[id]
	if ok {
		now := time.Now()
		msg.Pinned = true
		msg.PinnedAt = &now
		msg.PinnedBy = &pinnedBy
	}
	return nil
}

func (m *mockMessageRepo) Unpin(ctx context.Context, id, convID string) error {
	msg, ok := m.msgsByID[id]
	if ok {
		msg.Pinned = false
		msg.PinnedAt = nil
		msg.PinnedBy = nil
	}
	return nil
}

func (m *mockMessageRepo) SearchMessages(ctx context.Context, orgID, userID string, filter domain.MessageSearchFilter) (*domain.MessageSearchListResult, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	var items []*domain.MessageSearchResult
	for _, msg := range m.msgsByID {
		if msg.DeletedAt == nil {
			items = append(items, &domain.MessageSearchResult{Message: msg})
		}
	}
	return &domain.MessageSearchListResult{Items: items}, nil
}

func (m *mockMessageRepo) GetConversationLastMessage(ctx context.Context, convID string) (*domain.Message, error) {
	for _, msg := range m.msgsByID {
		if msg.ConversationID == convID && msg.DeletedAt == nil {
			return msg, nil
		}
	}
	return nil, errors.New("no messages")
}

func (m *mockMessageRepo) GetLastMessagesForConversations(ctx context.Context, convIDs []string) (map[string]*domain.Message, error) {
	return map[string]*domain.Message{}, nil
}

func (m *mockMessageRepo) Count(ctx context.Context, convID string) (int, error) {
	count := 0
	for _, msg := range m.msgsByID {
		if msg.ConversationID == convID && msg.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

type mockPendingAttachmentRepo struct {
	attsByID map[string]*domain.PendingAttachment
}

func newMockPendingAttachmentRepo() *mockPendingAttachmentRepo {
	return &mockPendingAttachmentRepo{attsByID: make(map[string]*domain.PendingAttachment)}
}

func (m *mockPendingAttachmentRepo) Create(ctx context.Context, att *domain.PendingAttachment) error {
	cp := *att
	m.attsByID[cp.ID] = &cp
	return nil
}

func (m *mockPendingAttachmentRepo) GetByID(ctx context.Context, id, uploadedBy string) (*domain.PendingAttachment, error) {
	a, ok := m.attsByID[id]
	if !ok || a.UploadedBy != uploadedBy {
		return nil, errors.New("not found")
	}
	return a, nil
}

func (m *mockPendingAttachmentRepo) Delete(ctx context.Context, id string) error {
	delete(m.attsByID, id)
	return nil
}

func (m *mockPendingAttachmentRepo) DeleteOlderThan(ctx context.Context, before time.Time) ([]*domain.PendingAttachment, error) {
	var stale []*domain.PendingAttachment
	for _, a := range m.attsByID {
		if a.CreatedAt.Before(before) {
			stale = append(stale, a)
		}
	}
	return stale, nil
}

type mockMessageAttachmentRepo struct {
	attsByID map[string]*domain.MessageAttachment
}

func newMockMessageAttachmentRepo() *mockMessageAttachmentRepo {
	return &mockMessageAttachmentRepo{attsByID: make(map[string]*domain.MessageAttachment)}
}

func (m *mockMessageAttachmentRepo) Create(ctx context.Context, att *domain.MessageAttachment) error {
	cp := *att
	m.attsByID[cp.ID] = &cp
	return nil
}

func (m *mockMessageAttachmentRepo) ListByMessage(ctx context.Context, messageID string) ([]*domain.MessageAttachment, error) {
	var items []*domain.MessageAttachment
	for _, a := range m.attsByID {
		if a.MessageID == messageID {
			items = append(items, a)
		}
	}
	return items, nil
}

func (m *mockMessageAttachmentRepo) GetByID(ctx context.Context, id string) (*domain.MessageAttachment, error) {
	a, ok := m.attsByID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return a, nil
}

// GetByIDAndConversation mirrors the store's conversation-scoped lookup used to
// prevent cross-conversation IDOR on attachment download.
func (m *mockMessageAttachmentRepo) GetByIDAndConversation(ctx context.Context, id, conversationID string) (*domain.MessageAttachment, error) {
	a, ok := m.attsByID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return a, nil
}

func (m *mockMessageAttachmentRepo) Delete(ctx context.Context, id string) error {
	delete(m.attsByID, id)
	return nil
}

func (m *mockMessageAttachmentRepo) UpdateMessageID(ctx context.Context, id, messageID string) error {
	a, ok := m.attsByID[id]
	if ok {
		a.MessageID = messageID
	}
	return nil
}

func (m *mockMessageAttachmentRepo) ListByMessages(ctx context.Context, messageIDs []string) (map[string][]*domain.MessageAttachment, error) {
	out := map[string][]*domain.MessageAttachment{}
	for _, id := range messageIDs {
		out[id] = nil
	}
	for _, a := range m.attsByID {
		for _, mid := range messageIDs {
			if a.MessageID == mid {
				out[mid] = append(out[mid], a)
			}
		}
	}
	return out, nil
}

type mockReactionRepo struct {
	reactions map[string]map[string]string // messageID -> userID -> emoji
}

func newMockReactionRepo() *mockReactionRepo {
	return &mockReactionRepo{reactions: make(map[string]map[string]string)}
}

func (m *mockReactionRepo) Add(ctx context.Context, orgID, messageID, userID, emoji string) error {
	if m.reactions[messageID] == nil {
		m.reactions[messageID] = make(map[string]string)
	}
	m.reactions[messageID][userID] = emoji
	return nil
}

func (m *mockReactionRepo) Remove(ctx context.Context, messageID, userID, emoji string) (bool, error) {
	if r, ok := m.reactions[messageID]; ok {
		if _, existed := r[userID]; existed {
			delete(r, userID)
			return true, nil
		}
	}
	return false, nil
}

func (m *mockReactionRepo) ListForMessages(ctx context.Context, messageIDs []string) ([]*domain.Reaction, error) {
	var out []*domain.Reaction
	for _, id := range messageIDs {
		for uid, emoji := range m.reactions[id] {
			out = append(out, &domain.Reaction{MessageID: id, UserID: uid, Emoji: emoji})
		}
	}
	return out, nil
}

type mockUserPrefRepo struct {
	prefs map[string]*domain.UserChannelPreference
}

func newMockUserPrefRepo() *mockUserPrefRepo {
	return &mockUserPrefRepo{prefs: make(map[string]*domain.UserChannelPreference)}
}

func (m *mockUserPrefRepo) Upsert(ctx context.Context, pref *domain.UserChannelPreference) error {
	m.prefs[pref.UserID+":"+pref.ConversationID] = pref
	return nil
}

func (m *mockUserPrefRepo) SetMuted(ctx context.Context, userID, convID, orgID string, muted bool) error {
	key := userID + ":" + convID
	if _, ok := m.prefs[key]; !ok {
		m.prefs[key] = &domain.UserChannelPreference{UserID: userID, ConversationID: convID, OrgID: orgID}
	}
	m.prefs[key].Muted = muted
	return nil
}

func (m *mockUserPrefRepo) SetNotificationLevel(ctx context.Context, userID, convID, orgID string, level domain.NotificationLevel) error {
	key := userID + ":" + convID
	if _, ok := m.prefs[key]; !ok {
		m.prefs[key] = &domain.UserChannelPreference{UserID: userID, ConversationID: convID, OrgID: orgID, NotificationLevel: level}
	} else {
		m.prefs[key].NotificationLevel = level
	}
	return nil
}

func (m *mockUserPrefRepo) Get(ctx context.Context, userID, convID string) (*domain.UserChannelPreference, error) {
	pref, ok := m.prefs[userID+":"+convID]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *pref
	return &cp, nil
}

func (m *mockUserPrefRepo) UpdateLastRead(ctx context.Context, userID, convID string) error {
	return nil
}

type mockPresenceRepo struct {
	statuses map[string]*domain.UserPresence
}

func newMockPresenceRepo() *mockPresenceRepo {
	return &mockPresenceRepo{statuses: make(map[string]*domain.UserPresence)}
}

func (m *mockPresenceRepo) Upsert(ctx context.Context, orgID, userID string, status domain.PresenceStatus) error {
	m.statuses[userID] = &domain.UserPresence{UserID: userID, OrgID: orgID, Status: status}
	return nil
}

func (m *mockPresenceRepo) Get(ctx context.Context, userID string) (*domain.UserPresence, error) {
	p, ok := m.statuses[userID]
	if !ok {
		return nil, errors.New("not found")
	}
	return p, nil
}

func (m *mockPresenceRepo) ListForOrg(ctx context.Context, orgID string) ([]*domain.UserPresence, error) {
	var out []*domain.UserPresence
	for _, p := range m.statuses {
		out = append(out, p)
	}
	return out, nil
}

type mockPasswordResetRepo struct {
	tokens map[string]*domain.PasswordReset
}

func newMockPasswordResetRepo() *mockPasswordResetRepo {
	return &mockPasswordResetRepo{tokens: make(map[string]*domain.PasswordReset)}
}

func (m *mockPasswordResetRepo) Create(ctx context.Context, reset *domain.PasswordReset) error {
	m.tokens[reset.TokenHash] = reset
	return nil
}

func (m *mockPasswordResetRepo) GetByTokenHash(ctx context.Context, hash string) (*domain.PasswordReset, error) {
	r, ok := m.tokens[hash]
	if !ok {
		return nil, errors.New("not found")
	}
	return r, nil
}

func (m *mockPasswordResetRepo) MarkUsed(ctx context.Context, id string) (bool, error) {
	for _, r := range m.tokens {
		if r.ID == id {
			if r.UsedAt != nil {
				return false, nil
			}
			now := time.Now()
			r.UsedAt = &now
			return true, nil
		}
	}
	return false, errors.New("not found")
}

func (m *mockPasswordResetRepo) DeleteExpired(ctx context.Context) error {
	return nil
}

type mockLinkRepo struct {
	links map[string][]string // channelID -> []projectIDs
	users map[string][]*domain.User
}

func newMockLinkRepo() *mockLinkRepo {
	return &mockLinkRepo{links: make(map[string][]string), users: make(map[string][]*domain.User)}
}

func (m *mockLinkRepo) Create(ctx context.Context, channelID, projectID string) error {
	m.links[channelID] = append(m.links[channelID], projectID)
	return nil
}

func (m *mockLinkRepo) Delete(ctx context.Context, channelID, projectID string) error {
	var updated []string
	for _, pid := range m.links[channelID] {
		if pid != projectID {
			updated = append(updated, pid)
		}
	}
	m.links[channelID] = updated
	return nil
}

func (m *mockLinkRepo) DeleteByChannel(ctx context.Context, channelID string) error {
	delete(m.links, channelID)
	return nil
}

func (m *mockLinkRepo) GetByChannel(ctx context.Context, channelID string) ([]string, error) {
	return m.links[channelID], nil
}

func (m *mockLinkRepo) GetByProject(ctx context.Context, projectID string) ([]string, error) {
	var channels []string
	for chID, pids := range m.links {
		for _, pid := range pids {
			if pid == projectID {
				channels = append(channels, chID)
				break
			}
		}
	}
	return channels, nil
}

func (m *mockLinkRepo) UserHasProjectAccess(ctx context.Context, projectID, userID string, userRole domain.Role) (bool, error) {
	return true, nil
}

func (m *mockLinkRepo) SetProjectLinks(ctx context.Context, channelID string, projectIDs []string) error {
	m.links[channelID] = projectIDs
	return nil
}

func (m *mockLinkRepo) GetUsersWithProjectAccess(ctx context.Context, orgID, projectID string) ([]*domain.User, error) {
	return m.users[projectID], nil
}

type mockViewRepo struct {
	viewsByID map[string]*domain.View
	pins      map[string]map[string]bool // userID -> viewID -> bool
}

func newMockViewRepo() *mockViewRepo {
	return &mockViewRepo{
		viewsByID: make(map[string]*domain.View),
		pins:      make(map[string]map[string]bool),
	}
}

func (m *mockViewRepo) Create(ctx context.Context, v *domain.View) error {
	m.viewsByID[v.ID] = v
	return nil
}

func (m *mockViewRepo) Update(ctx context.Context, v *domain.View) error {
	m.viewsByID[v.ID] = v
	return nil
}

func (m *mockViewRepo) Delete(ctx context.Context, orgID, id string) error {
	delete(m.viewsByID, id)
	return nil
}

func (m *mockViewRepo) GetByID(ctx context.Context, orgID, id string) (*domain.View, error) {
	v, ok := m.viewsByID[id]
	if !ok || v.OrgID != orgID {
		return nil, errors.New("not found")
	}
	cp := *v
	return &cp, nil
}

func (m *mockViewRepo) ListByProject(ctx context.Context, orgID, projectID string) ([]*domain.View, error) {
	var views []*domain.View
	for _, v := range m.viewsByID {
		if v.OrgID == orgID && v.ProjectID != nil && *v.ProjectID == projectID {
			views = append(views, v)
		}
	}
	return views, nil
}

func (m *mockViewRepo) ListGlobal(ctx context.Context, orgID string) ([]*domain.View, error) {
	var views []*domain.View
	for _, v := range m.viewsByID {
		if v.OrgID == orgID && v.ProjectID == nil {
			views = append(views, v)
		}
	}
	return views, nil
}

func (m *mockViewRepo) ListPinned(ctx context.Context, userID string) ([]*domain.View, error) {
	var views []*domain.View
	for vid := range m.pins[userID] {
		if v, ok := m.viewsByID[vid]; ok {
			views = append(views, v)
		}
	}
	return views, nil
}

func (m *mockViewRepo) Pin(ctx context.Context, viewID, userID string) error {
	if m.pins[userID] == nil {
		m.pins[userID] = make(map[string]bool)
	}
	m.pins[userID][viewID] = true
	return nil
}

func (m *mockViewRepo) Unpin(ctx context.Context, viewID, userID string) error {
	if m.pins[userID] != nil {
		delete(m.pins[userID], viewID)
	}
	return nil
}

type mockPermRepo struct {
	perms     map[string][]*domain.PermissionRule
	overrides map[string][]*domain.UserPermissionOverride
}

func newMockPermRepo() *mockPermRepo {
	return &mockPermRepo{
		perms:     make(map[string][]*domain.PermissionRule),
		overrides: make(map[string][]*domain.UserPermissionOverride),
	}
}

func (m *mockPermRepo) GetPermissions(ctx context.Context, channelID string) ([]*domain.PermissionRule, error) {
	return m.perms[channelID], nil
}

func (m *mockPermRepo) SetPermissions(ctx context.Context, channelID string, rules []*domain.PermissionRule) error {
	m.perms[channelID] = rules
	return nil
}

func (m *mockPermRepo) GetUserOverrides(ctx context.Context, channelID string) ([]*domain.UserPermissionOverride, error) {
	return m.overrides[channelID], nil
}

func (m *mockPermRepo) SetUserOverrides(ctx context.Context, channelID string, overrides []*domain.UserPermissionOverride) error {
	m.overrides[channelID] = overrides
	return nil
}

// --- Voice mocks ---

type mockVoiceParticipantRepo struct {
	participants map[string]*domain.VoiceParticipant // key: convID:userID
	userNames    map[string]string                   // userID -> display name; used by ListByConversationWithUser
}

func newMockVoiceParticipantRepo() *mockVoiceParticipantRepo {
	return &mockVoiceParticipantRepo{
		participants: make(map[string]*domain.VoiceParticipant),
		userNames:    make(map[string]string),
	}
}

func key(convID, userID string) string { return convID + ":" + userID }

func (m *mockVoiceParticipantRepo) ListByConversation(ctx context.Context, orgID, convID string) ([]*domain.VoiceParticipant, error) {
	var result []*domain.VoiceParticipant
	for _, p := range m.participants {
		if p.ConversationID == convID && p.OrgID == orgID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockVoiceParticipantRepo) ListByConversationWithUser(ctx context.Context, orgID, convID string) ([]domain.VoiceParticipantInfo, error) {
	var result []domain.VoiceParticipantInfo
	for _, p := range m.participants {
		if p.ConversationID == convID && p.OrgID == orgID {
			name := m.userNames[p.UserID]
			if name == "" {
				name = "User-" + p.UserID
			}
			result = append(result, domain.VoiceParticipantInfo{
				ID:       p.ID,
				UserID:   p.UserID,
				Name:     name,
				Muted:    p.Muted,
				Deafened: p.Deafened,
				JoinedAt: p.JoinedAt,
			})
		}
	}
	return result, nil
}

func (m *mockVoiceParticipantRepo) Join(ctx context.Context, p *domain.VoiceParticipant) error {
	cp := *p
	m.participants[key(p.ConversationID, p.UserID)] = &cp
	return nil
}

func (m *mockVoiceParticipantRepo) Leave(ctx context.Context, orgID, convID, userID string) error {
	delete(m.participants, key(convID, userID))
	return nil
}

func (m *mockVoiceParticipantRepo) UpdateFlags(ctx context.Context, orgID, convID, userID string, muted, deafened bool) error {
	p, ok := m.participants[key(convID, userID)]
	if !ok {
		return nil
	}
	p.Muted = muted
	p.Deafened = deafened
	return nil
}

func (m *mockVoiceParticipantRepo) Get(ctx context.Context, orgID, convID, userID string) (*domain.VoiceParticipant, error) {
	p, ok := m.participants[key(convID, userID)]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	cp := *p
	return &cp, nil
}

func (m *mockVoiceParticipantRepo) ListActiveVoiceForUser(ctx context.Context, orgID, userID string) ([]*domain.VoiceParticipant, error) {
	var result []*domain.VoiceParticipant
	for _, p := range m.participants {
		if p.UserID == userID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockVoiceParticipantRepo) Count(ctx context.Context, orgID, convID string) (int, error) {
	n := 0
	for _, p := range m.participants {
		if p.ConversationID == convID && p.OrgID == orgID {
			n++
		}
	}
	return n, nil
}

func (m *mockVoiceParticipantRepo) UpdateConnection(ctx context.Context, orgID, convID, userID, connectionID string) error {
	p, ok := m.participants[key(convID, userID)]
	if !ok {
		return fmt.Errorf("not found")
	}
	p.ConnectionID = connectionID
	return nil
}

type mockVoiceSFU struct {
	createPublisherCalls   []struct{ userID, convID, connID, orgID string }
	createSubscriberCalls  []struct{ subscriberID, subscriberConnID, publisherID, convID, orgID string }
	removeParticipantCalls []struct{ userID, convID string }
	handleAnswerCalls      []struct{ userID, convID, sdp string }
	setMutedCalls          []struct {
		userID, convID string
		muted          bool
	}
	mutedStates         map[string]bool // userID -> muted
	maxParticipants     int
	iceServersForUserFn func(userID string) []domain.ICEServer
}

func newMockVoiceSFU() *mockVoiceSFU {
	return &mockVoiceSFU{mutedStates: make(map[string]bool), maxParticipants: 25}
}

func (m *mockVoiceSFU) CreatePublisher(ctx context.Context, orgID, userID, connID, convID string) (string, error) {
	m.createPublisherCalls = append(m.createPublisherCalls, struct{ userID, convID, connID, orgID string }{userID, convID, connID, orgID})
	return "v=0\no=- mock", nil
}

func (m *mockVoiceSFU) CreateSubscriber(ctx context.Context, orgID, subscriberID, subscriberConnID, publisherID, convID string) (string, error) {
	m.createSubscriberCalls = append(m.createSubscriberCalls, struct{ subscriberID, subscriberConnID, publisherID, convID, orgID string }{subscriberID, subscriberConnID, publisherID, convID, orgID})
	return "v=0\no=- mock-sub", nil
}

func (m *mockVoiceSFU) HandleAnswer(ctx context.Context, userID, convID, sdp string) error {
	m.handleAnswerCalls = append(m.handleAnswerCalls, struct{ userID, convID, sdp string }{userID, convID, sdp})
	return nil
}

func (m *mockVoiceSFU) HandleSubscriberAnswer(ctx context.Context, subscriberID, publisherID, convID, sdp string) error {
	return nil
}

func (m *mockVoiceSFU) HandleICECandidate(ctx context.Context, userID, convID, candidateJSON string) error {
	return nil
}

func (m *mockVoiceSFU) HandleSubscriberICECandidate(ctx context.Context, userID, convID, publisherID, candidateJSON string) error {
	return nil
}

func (m *mockVoiceSFU) RemoveParticipant(ctx context.Context, userID, convID string) error {
	m.removeParticipantCalls = append(m.removeParticipantCalls, struct{ userID, convID string }{userID, convID})
	return nil
}

func (m *mockVoiceSFU) ICEServers() []domain.ICEServer {
	return []domain.ICEServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
	}
}

// ICEServersForUser returns per-user ICE servers (with ephemeral TURN creds
// when configured). In tests this just returns the static list unless the
// test overrides iceServersForUserFn.
func (m *mockVoiceSFU) ICEServersForUser(userID string) []domain.ICEServer {
	if m.iceServersForUserFn != nil {
		return m.iceServersForUserFn(userID)
	}
	return m.ICEServers()
}

// MaxParticipants returns the configured participant cap.
func (m *mockVoiceSFU) MaxParticipants() int {
	return m.maxParticipants
}

func (m *mockVoiceSFU) SetMuted(ctx context.Context, userID, convID string, muted bool) error {
	m.setMutedCalls = append(m.setMutedCalls, struct {
		userID, convID string
		muted          bool
	}{userID, convID, muted})
	m.mutedStates[userID] = muted
	return nil
}

// Callback setters: allow the mock to be wired up like the real SFU
func (m *mockVoiceSFU) SetOnSpeaking(fn func(userID, orgID string, speaking bool)) {}
func (m *mockVoiceSFU) SetOnSubscriberOffer(fn func(subscriberID, subscriberConnID, publisherID, convID, orgID, sdp string)) {
}
func (m *mockVoiceSFU) SetOnICECandidate(fn func(userID, connID, convID, orgID, candidateJSON string)) {
}
func (m *mockVoiceSFU) SetOnSubscriberICECandidate(fn func(subscriberID, subscriberConnID, publisherID, convID, orgID, candidateJSON string)) {
}

type mockPermService struct {
	resolvePermissionsFn func(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (*domain.ChannelPermissions, error)
}

func newMockPermService() *mockPermService {
	return &mockPermService{
		resolvePermissionsFn: func(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (*domain.ChannelPermissions, error) {
			return &domain.ChannelPermissions{CanView: true, CanSend: true, CanManage: false, CanPermissions: false}, nil
		},
	}
}

func (m *mockPermService) ResolvePermissions(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (*domain.ChannelPermissions, error) {
	return m.resolvePermissionsFn(ctx, orgID, channelID, userID, userRole)
}

func (m *mockPermService) ResolveRolePermissions(ctx context.Context, orgID, channelID string) ([]*domain.EffectivePermission, error) {
	return nil, nil
}

func (m *mockPermService) GetPermissions(ctx context.Context, channelID string) ([]*domain.PermissionRule, error) {
	return nil, nil
}

func (m *mockPermService) SetPermissions(ctx context.Context, channelID string, rules []*domain.PermissionRule) error {
	return nil
}

func (m *mockPermService) GetUserOverrides(ctx context.Context, channelID string) ([]*domain.UserPermissionOverride, error) {
	return nil, nil
}

func (m *mockPermService) SetUserOverrides(ctx context.Context, channelID string, overrides []*domain.UserPermissionOverride) error {
	return nil
}

func (m *mockPermService) UserHasAccess(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (bool, error) {
	perms, err := m.ResolvePermissions(ctx, orgID, channelID, userID, userRole)
	if err != nil {
		return false, err
	}
	return perms.CanView, nil
}

func (m *mockPermService) GetUsersWithProjectAccess(ctx context.Context, orgID, projectID string) ([]*domain.User, error) {
	return nil, nil
}

func (m *mockPermService) ListAccess(ctx context.Context, orgID, channelID string) ([]*domain.ChannelAccessEntry, error) {
	return nil, nil
}

func newMockChannelPermissionService() *mockPermService {
	return newMockPermService()
}

type mockProjectMemberRepo struct {
	memberships map[string][]*domain.UserProjectMembership
}

func newMockProjectMemberRepo() *mockProjectMemberRepo {
	return &mockProjectMemberRepo{
		memberships: make(map[string][]*domain.UserProjectMembership),
	}
}

func (m *mockProjectMemberRepo) List(ctx context.Context, orgID, projectID string, filter domain.UserFilter) (*domain.ProjectMemberListResult, error) {
	return &domain.ProjectMemberListResult{}, nil
}

func (m *mockProjectMemberRepo) Get(ctx context.Context, orgID, projectID, userID string) (*domain.ProjectMember, error) {
	return nil, nil
}

func (m *mockProjectMemberRepo) Add(ctx context.Context, projectID, userID string, role domain.Role) error {
	m.memberships[userID] = append(m.memberships[userID], &domain.UserProjectMembership{
		ProjectID: projectID,
		Role:      role,
	})
	return nil
}

func (m *mockProjectMemberRepo) Remove(ctx context.Context, projectID, userID string) error {
	updated := m.memberships[userID][:0]
	for _, mem := range m.memberships[userID] {
		if mem.ProjectID != projectID {
			updated = append(updated, mem)
		}
	}
	m.memberships[userID] = updated
	return nil
}

func (m *mockProjectMemberRepo) UpdateRole(ctx context.Context, projectID, userID string, role domain.Role) error {
	for _, mem := range m.memberships[userID] {
		if mem.ProjectID == projectID {
			mem.Role = role
		}
	}
	return nil
}

func (m *mockProjectMemberRepo) ListByUser(ctx context.Context, orgID, userID string) ([]*domain.UserProjectMembership, error) {
	return m.memberships[userID], nil
}

func (m *mockProjectMemberRepo) SetMemberships(_ context.Context, _ string, userID string, assignments []domain.ProjectAssignment) error {
	// Build desired map from assignments
	desired := make(map[string]domain.Role)
	for _, a := range assignments {
		desired[a.ProjectID] = a.Role
	}

	// Build current map
	current := make(map[string]domain.Role)
	for _, mem := range m.memberships[userID] {
		current[mem.ProjectID] = mem.Role
	}

	// Add or update
	for projectID, role := range desired {
		if _, exists := current[projectID]; exists {
			for _, mem := range m.memberships[userID] {
				if mem.ProjectID == projectID {
					mem.Role = role
				}
			}
		} else {
			m.memberships[userID] = append(m.memberships[userID], &domain.UserProjectMembership{
				ProjectID: projectID,
				Role:      role,
			})
		}
	}

	// Remove ones not in desired
	var kept []*domain.UserProjectMembership
	for _, mem := range m.memberships[userID] {
		if _, keep := desired[mem.ProjectID]; keep {
			kept = append(kept, mem)
		}
	}
	m.memberships[userID] = kept

	return nil
}

// ── Comment repository mock ──────────────────────────────────
type mockCommentRepo struct {
	byID    map[string]*domain.Comment
	byTask  map[string][]*domain.Comment
	created []*domain.Comment
	updated []*domain.Comment
	deleted []string
}

func newMockCommentRepo() *mockCommentRepo {
	return &mockCommentRepo{
		byID:   make(map[string]*domain.Comment),
		byTask: make(map[string][]*domain.Comment),
	}
}

func (m *mockCommentRepo) GetByID(ctx context.Context, orgID, id string) (*domain.Comment, error) {
	c, ok := m.byID[id]
	if !ok || c.OrgID != orgID {
		return nil, errors.New("not found")
	}
	return c, nil
}

func (m *mockCommentRepo) ListByTask(_ context.Context, filter domain.CommentFilter) (*domain.CommentListResult, error) {
	all := m.byTask[filter.TaskID]
	var filtered []*domain.Comment
	for _, c := range all {
		if filter.OrgID != "" && c.OrgID != filter.OrgID {
			continue
		}
		if c.DeletedAt != nil {
			continue
		}
		if filter.BeforeCursor != "" && !c.CreatedAt.Before(parseTime(filter.BeforeCursor)) {
			continue
		}
		filtered = append(filtered, c)
	}
	// Sort DESC (newest first)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	hasMore := len(filtered) > limit
	if hasMore {
		filtered = filtered[:limit]
	}
	// Reverse to ASC for display
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}
	result := &domain.CommentListResult{Items: filtered, HasMore: hasMore}
	if hasMore && len(filtered) > 0 {
		result.NextCursor = filtered[0].CreatedAt.Format(time.RFC3339Nano)
	}
	return result, nil
}

func (m *mockCommentRepo) Create(ctx context.Context, comment *domain.Comment) error {
	m.created = append(m.created, comment)
	m.byID[comment.ID] = comment
	m.byTask[comment.TaskID] = append(m.byTask[comment.TaskID], comment)
	return nil
}

func (m *mockCommentRepo) Update(ctx context.Context, comment *domain.Comment) error {
	m.updated = append(m.updated, comment)
	m.byID[comment.ID] = comment
	return nil
}

func (m *mockCommentRepo) SoftDelete(ctx context.Context, orgID, id string) error {
	m.deleted = append(m.deleted, id)
	if c, ok := m.byID[id]; ok {
		now := time.Now()
		c.DeletedAt = &now
	}
	return nil
}

// ── Label repository mock ─────────────────────────────────────
type mockLabelRepo struct {
	byID    map[string]*domain.Label
	byOrg   map[string][]*domain.Label
	taskLab map[string][]*domain.Label
	added   []struct{ taskID, labelID string }
	cleared []string
}

func newMockLabelRepo() *mockLabelRepo {
	return &mockLabelRepo{
		byID:    make(map[string]*domain.Label),
		byOrg:   make(map[string][]*domain.Label),
		taskLab: make(map[string][]*domain.Label),
	}
}

func (m *mockLabelRepo) GetByID(ctx context.Context, orgID, id string) (*domain.Label, error) {
	l, ok := m.byID[id]
	if !ok || l.OrgID != orgID {
		return nil, errors.New("not found")
	}
	return l, nil
}

func (m *mockLabelRepo) ListByOrg(ctx context.Context, orgID string) ([]*domain.Label, error) {
	return m.byOrg[orgID], nil
}

func (m *mockLabelRepo) Create(ctx context.Context, label *domain.Label) error {
	m.byID[label.ID] = label
	m.byOrg[label.OrgID] = append(m.byOrg[label.OrgID], label)
	return nil
}

func (m *mockLabelRepo) Update(ctx context.Context, label *domain.Label) error {
	m.byID[label.ID] = label
	for i, l := range m.byOrg[label.OrgID] {
		if l.ID == label.ID {
			m.byOrg[label.OrgID][i] = label
		}
	}
	return nil
}

func (m *mockLabelRepo) Delete(ctx context.Context, orgID, id string) error {
	delete(m.byID, id)
	return nil
}

func (m *mockLabelRepo) ClearTaskLabels(ctx context.Context, taskID string) error {
	m.cleared = append(m.cleared, taskID)
	delete(m.taskLab, taskID)
	return nil
}

func (m *mockLabelRepo) AddTaskLabel(ctx context.Context, taskID, labelID string) error {
	m.added = append(m.added, struct{ taskID, labelID string }{taskID, labelID})
	if l, ok := m.byID[labelID]; ok {
		m.taskLab[taskID] = append(m.taskLab[taskID], l)
	}
	return nil
}

func (m *mockLabelRepo) GetTaskLabels(ctx context.Context, taskID string) ([]*domain.Label, error) {
	return m.taskLab[taskID], nil
}

func (m *mockLabelRepo) ListLabelsByTaskIDs(ctx context.Context, taskIDs []string) (map[string][]*domain.Label, error) {
	out := make(map[string][]*domain.Label)
	for _, tid := range taskIDs {
		out[tid] = m.taskLab[tid]
	}
	return out, nil
}

func (m *mockLabelRepo) SetTaskLabels(ctx context.Context, taskID string, labelIDs []string) error {
	delete(m.taskLab, taskID)
	for _, lid := range labelIDs {
		if l, ok := m.byID[lid]; ok {
			m.taskLab[taskID] = append(m.taskLab[taskID], l)
		}
	}
	return nil
}

// mockMailer is a port.Mailer that records sent emails for assertion in tests.
type mockMailer struct {
	enabled bool
	sent    []mockEmail
}

type mockEmail struct {
	To, Subject, HTML, Text string
}

func newMockMailer(enabled bool) *mockMailer {
	return &mockMailer{enabled: enabled}
}

func (m *mockMailer) Enabled() bool { return m.enabled }

func (m *mockMailer) Send(_ context.Context, to, subject, html, text string) error {
	m.sent = append(m.sent, mockEmail{To: to, Subject: subject, HTML: html, Text: text})
	return nil
}

// mockUserPreferencesRepo is a port.UserPreferencesRepository backed by an
// in-memory map. Used by NotificationService tests.
type mockUserPreferencesRepo struct {
	prefs map[string]*domain.UserPreferences
}

func newMockUserPreferencesRepo() *mockUserPreferencesRepo {
	return &mockUserPreferencesRepo{prefs: make(map[string]*domain.UserPreferences)}
}

func (m *mockUserPreferencesRepo) Get(_ context.Context, userID string) (*domain.UserPreferences, error) {
	if p, ok := m.prefs[userID]; ok {
		return p, nil
	}
	// Default preferences (email + desktop on).
	return &domain.UserPreferences{EmailNotifications: true, DesktopNotifications: true}, nil
}

func (m *mockUserPreferencesRepo) Upsert(_ context.Context, prefs *domain.UserPreferences) error {
	m.prefs[prefs.UserID] = prefs
	return nil
}

// mockPushService is a port.PushService that records sent payloads for
// assertion in tests.
type mockPushService struct {
	enabled bool
	pubKey  string
	sent    []domain.PushPayload
	subs    []mockPushSub
}

type mockPushSub struct {
	userID, endpoint string
}

func newMockPushService(enabled bool) *mockPushService {
	return &mockPushService{enabled: enabled}
}

func (m *mockPushService) Enabled() bool     { return m.enabled }
func (m *mockPushService) PublicKey() string { return m.pubKey }

func (m *mockPushService) Subscribe(_ context.Context, userID, _ string, endpoint, _, _ string) error {
	m.subs = append(m.subs, mockPushSub{userID: userID, endpoint: endpoint})
	return nil
}

func (m *mockPushService) Unsubscribe(_ context.Context, _, _ string) error { return nil }

func (m *mockPushService) Send(_ context.Context, _ string, payload domain.PushPayload) error {
	m.sent = append(m.sent, payload)
	return nil
}

// mockPushSubRepo is a port.PushSubscriptionRepository backed by an in-memory
// slice. Used by PushService tests.
type mockPushSubRepo struct {
	subs []*domain.PushSubscription
}

func newMockPushSubRepo() *mockPushSubRepo {
	return &mockPushSubRepo{}
}

func (m *mockPushSubRepo) Upsert(_ context.Context, sub *domain.PushSubscription) (*domain.PushSubscription, error) {
	// Replace if endpoint exists for this user.
	for i, s := range m.subs {
		if s.UserID == sub.UserID && s.Endpoint == sub.Endpoint {
			m.subs[i] = sub
			return sub, nil
		}
	}
	m.subs = append(m.subs, sub)
	return sub, nil
}

func (m *mockPushSubRepo) ListByUser(_ context.Context, userID string) ([]*domain.PushSubscription, error) {
	var out []*domain.PushSubscription
	for _, s := range m.subs {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *mockPushSubRepo) Delete(_ context.Context, userID, endpoint string) (int64, error) {
	for i, s := range m.subs {
		if s.UserID == userID && s.Endpoint == endpoint {
			m.subs = append(m.subs[:i], m.subs[i+1:]...)
			return 1, nil
		}
	}
	return 0, nil
}

func (m *mockPushSubRepo) DeleteByUser(_ context.Context, userID string) (int64, error) {
	kept := m.subs[:0]
	var n int64
	for _, s := range m.subs {
		if s.UserID == userID {
			n++
		} else {
			kept = append(kept, s)
		}
	}
	m.subs = kept
	return n, nil
}

// mockTimeEntryRepo is an in-memory TimeEntryRepository for tests.
type mockTimeEntryRepo struct {
	entriesByTask map[string][]*domain.TimeEntry
}

func newMockTimeEntryRepo() *mockTimeEntryRepo {
	return &mockTimeEntryRepo{entriesByTask: make(map[string][]*domain.TimeEntry)}
}

func (m *mockTimeEntryRepo) ListByTask(_ context.Context, taskID string) ([]*domain.TimeEntry, error) {
	return m.entriesByTask[taskID], nil
}
func (m *mockTimeEntryRepo) GetActiveTimer(_ context.Context, taskID, userID string) (*domain.TimeEntry, error) {
	return nil, apperr.ErrNotFound
}
func (m *mockTimeEntryRepo) GetActiveTimerByUser(_ context.Context, userID string) ([]*domain.TimeEntry, error) {
	return nil, nil
}
func (m *mockTimeEntryRepo) StartTimer(_ context.Context, id, taskID, userID, description string) error {
	return nil
}
func (m *mockTimeEntryRepo) StopTimer(_ context.Context, id, userID string) error {
	return nil
}
func (m *mockTimeEntryRepo) Create(_ context.Context, entry *domain.TimeEntry) error {
	m.entriesByTask[entry.TaskID] = append(m.entriesByTask[entry.TaskID], entry)
	return nil
}
func (m *mockTimeEntryRepo) Update(_ context.Context, entry *domain.TimeEntry) error {
	entries := m.entriesByTask[entry.TaskID]
	for i, e := range entries {
		if e.ID == entry.ID {
			entries[i] = entry
			return nil
		}
	}
	return apperr.ErrNotFound
}
func (m *mockTimeEntryRepo) Delete(_ context.Context, id, taskID string) error {
	entries := m.entriesByTask[taskID]
	for i, e := range entries {
		if e.ID == id {
			m.entriesByTask[taskID] = append(entries[:i], entries[i+1:]...)
			return nil
		}
	}
	return apperr.ErrNotFound
}
func (m *mockTimeEntryRepo) TotalTimeByTask(_ context.Context, taskID string) (int64, error) {
	return 0, nil
}

func (m *mockTimeEntryRepo) StartTimerAtomic(_ context.Context, id, taskID, userID, description string) error {
	return nil
}

// mockTaskActivityRepo is an in-memory TaskActivityRepository for tests.
type mockTaskActivityRepo struct {
	entries []*domain.TaskActivity
}

func newMockTaskActivityRepo() *mockTaskActivityRepo {
	return &mockTaskActivityRepo{}
}

func (m *mockTaskActivityRepo) Create(_ context.Context, entry *domain.TaskActivity) error {
	m.entries = append(m.entries, entry)
	return nil
}

func (m *mockTaskActivityRepo) List(_ context.Context, taskID string, filter domain.TaskActivityFilter) (*domain.TaskActivityResult, error) {
	var all []*domain.TaskActivity
	for _, e := range m.entries {
		if e.TaskID == taskID {
			all = append(all, e)
		}
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	hasMore := len(all) > limit
	if hasMore {
		all = all[:limit]
	}
	return &domain.TaskActivityResult{
		Items:   all,
		HasMore: hasMore,
	}, nil
}

// mockTemplateRepo is an in-memory TaskTemplateRepository for tests.
type mockTemplateRepo struct {
	templates    []*domain.TaskTemplate
	listDueCalls int
}

func newMockTemplateRepo() *mockTemplateRepo {
	return &mockTemplateRepo{}
}

func (m *mockTemplateRepo) Create(_ context.Context, t *domain.TaskTemplate) error {
	m.templates = append(m.templates, t)
	return nil
}

func (m *mockTemplateRepo) GetByID(_ context.Context, orgID, id string) (*domain.TaskTemplate, error) {
	for _, t := range m.templates {
		if t.ID == id && t.OrgID == orgID {
			return t, nil
		}
	}
	return nil, apperr.ErrNotFound
}

func (m *mockTemplateRepo) ListByProject(_ context.Context, orgID, projectID string) ([]*domain.TaskTemplate, error) {
	var out []*domain.TaskTemplate
	for _, t := range m.templates {
		if t.OrgID == orgID && t.ProjectID == projectID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *mockTemplateRepo) ListDueRecurring(_ context.Context, before time.Time) ([]*domain.TaskTemplate, error) {
	m.listDueCalls++
	var out []*domain.TaskTemplate
	for _, t := range m.templates {
		if t.RecurrencePattern != "none" && t.NextRunAt != nil && !t.NextRunAt.After(before) {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *mockTemplateRepo) Update(_ context.Context, t *domain.TaskTemplate) error {
	for i, existing := range m.templates {
		if existing.ID == t.ID {
			m.templates[i] = t
			return nil
		}
	}
	return apperr.ErrNotFound
}

func (m *mockTemplateRepo) UpdateNextRun(_ context.Context, orgID, id string, nextRun *time.Time) error {
	for _, t := range m.templates {
		if t.ID == id && t.OrgID == orgID {
			t.NextRunAt = nextRun
			return nil
		}
	}
	return apperr.ErrNotFound
}

func (m *mockTemplateRepo) ClaimDueRecurring(_ context.Context, orgID, id string, currentNextRun, newNextRun *time.Time) (bool, error) {
	for _, t := range m.templates {
		if t.ID == id && t.OrgID == orgID {
			// CAS: only succeeds if currentNextRun matches what we read
			if t.NextRunAt == nil || currentNextRun == nil || !t.NextRunAt.Equal(*currentNextRun) {
				return false, nil
			}
			t.NextRunAt = newNextRun
			return true, nil
		}
	}
	return false, nil
}

func (m *mockTemplateRepo) SetLastError(_ context.Context, _, _, _ string) error {
	return nil
}

func (m *mockTemplateRepo) Delete(_ context.Context, orgID, id string) error {
	for i, t := range m.templates {
		if t.ID == id && t.OrgID == orgID {
			m.templates = append(m.templates[:i], m.templates[i+1:]...)
			return nil
		}
	}
	return apperr.ErrNotFound
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}
