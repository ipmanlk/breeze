package store

import (
	"context"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

var _ port.VoiceParticipantRepository = (*VoiceParticipantStore)(nil)

// VoiceParticipantStore implements port.VoiceParticipantRepository using sqlc.
type VoiceParticipantStore struct {
	queries *sqlc.Queries
}

// NewVoiceParticipantStore creates a new VoiceParticipantStore.
func NewVoiceParticipantStore(queries *sqlc.Queries) *VoiceParticipantStore {
	return &VoiceParticipantStore{queries: queries}
}

// ListByConversation returns all voice participants in a conversation.
func (s *VoiceParticipantStore) ListByConversation(ctx context.Context, orgID, convID string) ([]*domain.VoiceParticipant, error) {
	rows, err := s.queries.ListVoiceParticipants(ctx, sqlc.ListVoiceParticipantsParams{
		ConversationID: convID,
		OrgID:          orgID,
	})
	if err != nil {
		return nil, err
	}

	participants := make([]*domain.VoiceParticipant, len(rows))
	for i, row := range rows {
		participants[i] = listVoiceRowToDomain(row)
	}
	return participants, nil
}

// ListByConversationWithUser returns voice participants with user info (name,
// avatar_url) in a single JOIN query, avoiding the N+1 pattern of calling
// userRepo.GetByID per participant.
func (s *VoiceParticipantStore) ListByConversationWithUser(ctx context.Context, orgID, convID string) ([]domain.VoiceParticipantInfo, error) {
	rows, err := s.queries.ListVoiceParticipantsWithUser(ctx, sqlc.ListVoiceParticipantsWithUserParams{
		ConversationID: convID,
		OrgID:          orgID,
	})
	if err != nil {
		return nil, err
	}

	infos := make([]domain.VoiceParticipantInfo, len(rows))
	for i, row := range rows {
		var avatarURL string
		if row.UserAvatarUrl != nil {
			avatarURL = *row.UserAvatarUrl
		}
		infos[i] = domain.VoiceParticipantInfo{
			ID:        row.ID,
			UserID:    row.UserID,
			Name:      row.UserName,
			AvatarURL: avatarURL,
			Muted:     row.Muted != 0,
			Deafened:  row.Deafened != 0,
			Speaking:  false, // ephemeral; service sets this from cache
			JoinedAt:  parseTime(row.JoinedAt),
		}
	}
	return infos, nil
}

// Join adds a user to a voice channel.
func (s *VoiceParticipantStore) Join(ctx context.Context, p *domain.VoiceParticipant) error {
	return s.queries.JoinVoiceChannel(ctx, sqlc.JoinVoiceChannelParams{
		ID:             p.ID,
		ConversationID: p.ConversationID,
		OrgID:          p.OrgID,
		UserID:         p.UserID,
		ConnectionID:   p.ConnectionID,
	})
}

// Leave removes a user from a voice channel.
func (s *VoiceParticipantStore) Leave(ctx context.Context, orgID, convID, userID string) error {
	return s.queries.LeaveVoiceChannel(ctx, sqlc.LeaveVoiceChannelParams{
		ConversationID: convID,
		UserID:         userID,
		OrgID:          orgID,
	})
}

// UpdateFlags updates muted/deafened flags for a participant.
func (s *VoiceParticipantStore) UpdateFlags(ctx context.Context, orgID, convID, userID string, muted, deafened bool) error {
	var m, d int64
	if muted {
		m = 1
	}
	if deafened {
		d = 1
	}
	return s.queries.UpdateVoiceFlags(ctx, sqlc.UpdateVoiceFlagsParams{
		Muted:          m,
		Deafened:       d,
		ConversationID: convID,
		UserID:         userID,
		OrgID:          orgID,
	})
}

// UpdateConnection reassigns the owning WS connection for a participant (used
// when a tab takes over an existing voice session; a takeover, not a new slot).
func (s *VoiceParticipantStore) UpdateConnection(ctx context.Context, orgID, convID, userID, connectionID string) error {
	return s.queries.UpdateVoiceConnection(ctx, sqlc.UpdateVoiceConnectionParams{
		ConnectionID:   connectionID,
		ConversationID: convID,
		UserID:         userID,
		OrgID:          orgID,
	})
}

// Count returns the number of participants in a voice channel.
func (s *VoiceParticipantStore) Count(ctx context.Context, orgID, convID string) (int, error) {
	n, err := s.queries.CountVoiceParticipants(ctx, sqlc.CountVoiceParticipantsParams{
		ConversationID: convID,
		OrgID:          orgID,
	})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// Get returns a specific voice participant.
func (s *VoiceParticipantStore) Get(ctx context.Context, orgID, convID, userID string) (*domain.VoiceParticipant, error) {
	row, err := s.queries.GetVoiceParticipant(ctx, sqlc.GetVoiceParticipantParams{
		ConversationID: convID,
		UserID:         userID,
		OrgID:          orgID,
	})
	if err != nil {
		return nil, err
	}
	return getVoiceRowToDomain(row), nil
}

// ListActiveVoiceForUser returns all active voice channels for a user.
// DeleteAll removes every voice_participants row. Called once at startup:
// the table is in-memory session state and a crash leaves stale rows whose
// connections no longer exist.
func (s *VoiceParticipantStore) DeleteAll(ctx context.Context) (int64, error) {
	return s.queries.DeleteAllVoiceParticipants(ctx)
}

func (s *VoiceParticipantStore) ListActiveVoiceForUser(ctx context.Context, orgID, userID string) ([]*domain.VoiceParticipant, error) {
	rows, err := s.queries.ListActiveVoiceForUser(ctx, sqlc.ListActiveVoiceForUserParams{
		UserID: userID,
		OrgID:  orgID,
	})
	if err != nil {
		return nil, err
	}

	participants := make([]*domain.VoiceParticipant, len(rows))
	for i, row := range rows {
		participants[i] = activeVoiceRowToDomain(row)
	}
	return participants, nil
}

func listVoiceRowToDomain(row sqlc.VoiceParticipant) *domain.VoiceParticipant {
	return &domain.VoiceParticipant{
		ID:             row.ID,
		ConversationID: row.ConversationID,
		OrgID:          row.OrgID,
		UserID:         row.UserID,
		Muted:          row.Muted != 0,
		Deafened:       row.Deafened != 0,
		ConnectionID:   row.ConnectionID,
		JoinedAt:       parseTime(row.JoinedAt),
	}
}

func getVoiceRowToDomain(row sqlc.VoiceParticipant) *domain.VoiceParticipant {
	return &domain.VoiceParticipant{
		ID:             row.ID,
		ConversationID: row.ConversationID,
		OrgID:          row.OrgID,
		UserID:         row.UserID,
		Muted:          row.Muted != 0,
		Deafened:       row.Deafened != 0,
		ConnectionID:   row.ConnectionID,
		JoinedAt:       parseTime(row.JoinedAt),
	}
}

func activeVoiceRowToDomain(row sqlc.VoiceParticipant) *domain.VoiceParticipant {
	return &domain.VoiceParticipant{
		ID:             row.ID,
		ConversationID: row.ConversationID,
		OrgID:          row.OrgID,
		UserID:         row.UserID,
		Muted:          row.Muted != 0,
		Deafened:       row.Deafened != 0,
		ConnectionID:   row.ConnectionID,
		JoinedAt:       parseTime(row.JoinedAt),
	}
}
