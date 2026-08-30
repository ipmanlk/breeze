package store

import (
	"context"
	"time"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/store/sqlc"
)

type SessionStore struct {
	q *sqlc.Queries
}

func NewSessionStore(q *sqlc.Queries) *SessionStore {
	return &SessionStore{q: q}
}

var _ port.SessionRepository = (*SessionStore)(nil)

func (s *SessionStore) toDomain(sess struct {
	ID        string
	UserID    string
	OrgID     string
	Role      string
	ExpiresAt string
	RevokedAt *string
	CreatedAt string
	UserAgent *string
	IpAddress *string
}) domain.Session {
	var revokedAt *time.Time
	if sess.RevokedAt != nil {
		t := parseTime(*sess.RevokedAt)
		revokedAt = &t
	}
	var userAgent, ipAddress string
	if sess.UserAgent != nil {
		userAgent = *sess.UserAgent
	}
	if sess.IpAddress != nil {
		ipAddress = *sess.IpAddress
	}
	return domain.Session{
		ID:        sess.ID,
		UserID:    sess.UserID,
		OrgID:     sess.OrgID,
		Role:      domain.Role(sess.Role),
		ExpiresAt: parseTime(sess.ExpiresAt),
		RevokedAt: revokedAt,
		CreatedAt: parseTime(sess.CreatedAt),
		UserAgent: userAgent,
		IPAddress: ipAddress,
	}
}

func (s *SessionStore) Create(ctx context.Context, session *domain.Session) error {
	var userAgent, ipAddress *string
	if session.UserAgent != "" {
		userAgent = &session.UserAgent
	}
	if session.IPAddress != "" {
		ipAddress = &session.IPAddress
	}
	return s.q.CreateSession(ctx, sqlc.CreateSessionParams{
		ID:        session.ID,
		UserID:    session.UserID,
		OrgID:     session.OrgID,
		Role:      string(session.Role),
		ExpiresAt: formatTime(session.ExpiresAt),
		UserAgent: userAgent,
		IpAddress: ipAddress,
	})
}

func (s *SessionStore) GetByID(ctx context.Context, id string) (*domain.Session, error) {
	row, err := s.q.GetSessionByID(ctx, id)
	if err != nil {
		return nil, mapScanErr(err)
	}
	d := s.toDomain(row)
	return &d, nil
}

func (s *SessionStore) ListByUser(ctx context.Context, userID string) ([]*domain.Session, error) {
	rows, err := s.q.ListSessionsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Session, len(rows))
	for i, row := range rows {
		d := s.toDomain(row)
		out[i] = &d
	}
	return out, nil
}

func (s *SessionStore) Revoke(ctx context.Context, id string) error {
	return s.q.RevokeSession(ctx, id)
}

func (s *SessionStore) Delete(ctx context.Context, id string) error {
	return s.q.DeleteSession(ctx, id)
}

func (s *SessionStore) DeleteByUser(ctx context.Context, userID string) error {
	return s.q.DeleteUserSessions(ctx, userID)
}

func (s *SessionStore) DeleteExpired(ctx context.Context) error {
	return s.q.DeleteExpiredSessions(ctx)
}
