package store

import (
	"context"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

var _ port.ChannelPermissionRepository = (*ChannelPermissionStore)(nil)

type ChannelPermissionStore struct {
	q *sqlc.Queries
}

func NewChannelPermissionStore(q *sqlc.Queries) *ChannelPermissionStore {
	return &ChannelPermissionStore{q: q}
}

func (s *ChannelPermissionStore) GetPermissions(ctx context.Context, channelID string) ([]*domain.PermissionRule, error) {
	rows, err := s.q.GetChannelPermissions(ctx, channelID)
	if err != nil {
		return nil, err
	}

	rules := make([]*domain.PermissionRule, len(rows))
	for i, r := range rows {
		role := domain.Role(r.Role)
		perm := domain.Permission(r.Permission)
		chID := channelID
		rules[i] = &domain.PermissionRule{
			ChannelID:  &chID,
			Role:       role,
			Permission: perm,
			Allow:      r.Allow != 0,
		}
	}
	return rules, nil
}

func (s *ChannelPermissionStore) SetPermissions(ctx context.Context, channelID string, rules []*domain.PermissionRule) error {
	if err := s.q.DeleteChannelPermissions(ctx, channelID); err != nil {
		return err
	}

	for _, rule := range rules {
		var allow int64
		if rule.Allow {
			allow = 1
		}
		if err := s.q.CreateChannelPermission(ctx, sqlc.CreateChannelPermissionParams{
			ChannelID:  channelID,
			Role:       string(rule.Role),
			Permission: string(rule.Permission),
			Allow:      allow,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *ChannelPermissionStore) GetUserOverrides(ctx context.Context, channelID string) ([]*domain.UserPermissionOverride, error) {
	rows, err := s.q.GetChannelUserOverrides(ctx, channelID)
	if err != nil {
		return nil, err
	}

	overrides := make([]*domain.UserPermissionOverride, len(rows))
	for i, r := range rows {
		overrides[i] = &domain.UserPermissionOverride{
			ChannelID:  channelID,
			UserID:     r.UserID,
			Permission: domain.Permission(r.Permission),
			Allow:      r.Allow != 0,
		}
	}
	return overrides, nil
}

func (s *ChannelPermissionStore) SetUserOverrides(ctx context.Context, channelID string, overrides []*domain.UserPermissionOverride) error {
	if err := s.q.DeleteChannelUserOverrides(ctx, channelID); err != nil {
		return err
	}

	for _, o := range overrides {
		var allow int64
		if o.Allow {
			allow = 1
		}
		if err := s.q.CreateChannelUserOverride(ctx, sqlc.CreateChannelUserOverrideParams{
			ChannelID:  channelID,
			UserID:     o.UserID,
			Permission: string(o.Permission),
			Allow:      allow,
		}); err != nil {
			return err
		}
	}
	return nil
}
