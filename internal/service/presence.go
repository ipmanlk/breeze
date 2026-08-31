package service

import (
	"context"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
)

type PresenceService struct {
	repo        port.PresenceRepository
	broadcaster port.Broadcaster
}

var _ port.PresenceService = (*PresenceService)(nil)

func NewPresenceService(repo port.PresenceRepository, broadcaster port.Broadcaster) *PresenceService {
	return &PresenceService{repo: repo, broadcaster: broadcaster}
}

func (s *PresenceService) SetStatus(ctx context.Context, orgID, userID string, status domain.PresenceStatus) error {
	if err := s.repo.Upsert(ctx, orgID, userID, status); err != nil {
		return err
	}
	if s.broadcaster != nil {
		_ = s.broadcaster.Broadcast(
			domain.RoomKeyOrg(orgID),
			string(domain.WsTypePresenceUpdated),
			map[string]any{
				"user_id": userID,
				"status":  string(status),
			},
		)
	}
	return nil
}

func (s *PresenceService) ListForOrg(ctx context.Context, orgID string) ([]*domain.UserPresence, error) {
	return s.repo.ListForOrg(ctx, orgID)
}
