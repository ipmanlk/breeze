package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/storage"

	"github.com/google/uuid"
)

type AttachmentService struct {
	repo         port.AttachmentRepository
	taskRepo     port.TaskRepository
	store        storage.Storage
	access       port.AccessChecker
	activityRepo port.TaskActivityRepository
	broadcaster  port.Broadcaster
}

var _ port.AttachmentService = (*AttachmentService)(nil)

func NewAttachmentService(repo port.AttachmentRepository, taskRepo port.TaskRepository, store storage.Storage, access port.AccessChecker, activityRepo port.TaskActivityRepository, broadcaster port.Broadcaster) *AttachmentService {
	return &AttachmentService{repo: repo, taskRepo: taskRepo, store: store, access: access, activityRepo: activityRepo, broadcaster: broadcaster}
}

func (s *AttachmentService) List(ctx context.Context, orgID, taskID, projectID string) ([]*domain.Attachment, error) {
	_, err := s.taskRepo.GetByID(ctx, orgID, taskID, projectID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	return s.repo.ListByTask(ctx, taskID)
}

func (s *AttachmentService) Create(ctx context.Context, p domain.CreateAttachmentParams) (*domain.Attachment, error) {
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, p.CreatedBy, p.OrgID, p.ProjectID, domain.PermAttachmentCreate); err != nil {
			return nil, err
		}
	}
	_, err := s.taskRepo.GetByID(ctx, p.OrgID, p.TaskID, p.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	id := uuid.New().String()
	ext := filepath.Ext(p.Filename)
	storagePath := filepath.Join(p.TaskID, id+ext)

	if err := s.store.Save(ctx, storagePath, p.File); err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	att := &domain.Attachment{
		ID:          id,
		TaskID:      p.TaskID,
		Filename:    p.Filename,
		ContentType: p.ContentType,
		Size:        p.Size,
		StoragePath: storagePath,
		CreatedBy:   p.CreatedBy,
	}

	if err := s.repo.Create(ctx, att); err != nil {
		_ = s.store.Delete(ctx, storagePath)
		return nil, err
	}

	att, err = s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Record activity best-effort.
	if s.activityRepo != nil {
		_ = s.activityRepo.Create(ctx, &domain.TaskActivity{
			ID:        uuid.New().String(),
			TaskID:    p.TaskID,
			OrgID:     p.OrgID,
			ProjectID: p.ProjectID,
			ActorID:   p.CreatedBy,
			Action:    domain.ActivityFileAttached,
			Field:     "attachment",
			OldValue:  "",
			NewValue:  p.Filename,
			CreatedAt: time.Now(),
		})
		s.broadcastTaskActivity(ctx, p.OrgID, p.ProjectID, p.TaskID)
	}

	return att, nil
}

func (s *AttachmentService) broadcastTaskActivity(ctx context.Context, orgID, projectID, taskID string) {
	if s.broadcaster == nil {
		return
	}
	_ = s.broadcaster.Broadcast(
		domain.RoomKeyProject(orgID, projectID),
		string(domain.WsTypeTaskActivityRecorded),
		map[string]any{"task_id": taskID},
	)
}

func (s *AttachmentService) Delete(ctx context.Context, userID, orgID, id, taskID, projectID string) error {
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, userID, orgID, projectID, domain.PermAttachmentDelete); err != nil {
			return err
		}
	}
	_, err := s.taskRepo.GetByID(ctx, orgID, taskID, projectID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}
	att, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if att.TaskID != taskID {
		return fmt.Errorf("attachment does not belong to task")
	}
	_ = s.store.Delete(ctx, att.StoragePath)

	// Record activity best-effort before the delete.
	if s.activityRepo != nil {
		_ = s.activityRepo.Create(ctx, &domain.TaskActivity{
			ID:        uuid.New().String(),
			TaskID:    taskID,
			OrgID:     orgID,
			ProjectID: projectID,
			ActorID:   userID,
			Action:    domain.ActivityFileRemoved,
			Field:     "attachment",
			OldValue:  att.Filename,
			NewValue:  "",
			CreatedAt: time.Now(),
		})
		s.broadcastTaskActivity(ctx, orgID, projectID, taskID)
	}

	return s.repo.Delete(ctx, id, taskID)
}

func (s *AttachmentService) Get(ctx context.Context, id string) (*domain.Attachment, error) {
	return s.repo.GetByID(ctx, id)
}

// Download returns the file reader plus the attachment's content type, the
// owning task's project ID (for access control), and the stored filename (for
// the Content-Disposition header).
//
// orgID is required so the task lookup is org-scoped: the attachment is only
// readable if its owning task belongs to the caller's org. The returned
// projectID lets the handler enforce project-level access (membership for
// viewer/guest roles) before streaming the bytes.
func (s *AttachmentService) Download(ctx context.Context, orgID, id string) (io.ReadCloser, string, string, string, error) {
	att, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, "", "", "", err
	}
	// Look up the owning task (org-scoped) to get the project ID for access
	// control. GetByIDAndOrg returns ErrNotFound when the task is not in
	// orgID, which blocks cross-org attachment reads.
	task, err := s.taskRepo.GetByIDAndOrg(ctx, orgID, att.TaskID)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("task not found: %w", err)
	}
	reader, err := s.store.Get(ctx, att.StoragePath)
	if err != nil {
		return nil, "", "", "", err
	}
	return reader, att.ContentType, task.ProjectID, att.Filename, nil
}
