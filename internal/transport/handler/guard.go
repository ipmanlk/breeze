package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
)

// EnsureProjectAccess returns nil if the authenticated user may access the
// given project. It extracts identity from context and delegates to the
// AccessService, which either performs a fast-path context lookup (when
// RequireProjectPermission middleware already ran) or resolves project
// membership from the database.
func EnsureProjectAccess(
	ctx context.Context,
	accessSvc port.AccessService,
	projectID string,
) error {
	orgID, _ := transport.OrgIDFromContext(ctx)
	userID, _ := transport.UserIDFromContext(ctx)
	roleStr, _ := transport.RoleFromContext(ctx)
	return accessSvc.EnsureProjectAccess(ctx, orgID, userID, domain.Role(roleStr), projectID)
}

// ResolveProjectEffectiveRole returns the caller's effective role on a
// project by delegating to the AccessService.
func ResolveProjectEffectiveRole(
	ctx context.Context,
	accessSvc port.AccessService,
	projectID string,
) (domain.Role, error) {
	orgID, _ := transport.OrgIDFromContext(ctx)
	userID, _ := transport.UserIDFromContext(ctx)
	roleStr, _ := transport.RoleFromContext(ctx)
	return accessSvc.ResolveProjectEffectiveRole(ctx, orgID, userID, domain.Role(roleStr), projectID)
}

// EnsureConversationAccess writes an error response and returns false when the
// authenticated user has no access to the given conversation. Delegates to
// the AccessService.
func EnsureConversationAccess(
	w http.ResponseWriter,
	r *http.Request,
	accessSvc port.AccessService,
	convID string,
) bool {
	return convAccessGuard(w, r, accessSvc, convID, false, "")
}

// EnsureConversationSendAccess writes an error response and returns false when
// the authenticated user lacks the channel-level "send" permission in the
// given conversation.
func EnsureConversationSendAccess(
	w http.ResponseWriter,
	r *http.Request,
	accessSvc port.AccessService,
	convID string,
) bool {
	return convAccessGuard(w, r, accessSvc, convID, true, "send")
}

// EnsureConversationManageAccess writes an error response and returns false
// when the authenticated user lacks the channel-level "manage" permission in
// the given conversation.
func EnsureConversationManageAccess(
	w http.ResponseWriter,
	r *http.Request,
	accessSvc port.AccessService,
	convID string,
) bool {
	return convAccessGuard(w, r, accessSvc, convID, true, "manage")
}

// convAccessGuard is the shared body of the conversation-access guards.
// It reads identity from context, calls the AccessService, and writes an
// HTTP error response when access is denied.
func convAccessGuard(
	w http.ResponseWriter,
	r *http.Request,
	accessSvc port.AccessService,
	convID string,
	usePermissions bool,
	permName string,
) bool {
	ctx := r.Context()
	orgID, ok := transport.OrgIDFromContext(ctx)
	if !ok || orgID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "invalid_request", "ErrOrgContextMissing")
		return false
	}
	userID, ok := transport.UserIDFromContext(ctx)
	if !ok || userID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", "ErrNotAuthenticated")
		return false
	}
	roleStr, _ := transport.RoleFromContext(ctx)

	var err error
	if usePermissions {
		switch permName {
		case "send":
			err = accessSvc.EnsureConversationSendAccess(ctx, orgID, userID, domain.Role(roleStr), convID)
		case "manage":
			err = accessSvc.EnsureConversationManageAccess(ctx, orgID, userID, domain.Role(roleStr), convID)
		default:
			transport.LocalizedErrorJSON(w, r, http.StatusForbidden, "forbidden", "ErrUnknownPermissionCheck")
			return false
		}
	} else {
		err = accessSvc.EnsureConversationAccess(ctx, orgID, userID, domain.Role(roleStr), convID)
	}

	if err != nil {
		if errors.Is(err, apperr.ErrForbidden) {
			transport.LocalizedErrorJSON(w, r, http.StatusForbidden, "forbidden", "ErrNoAccessToConversation")
			return false
		}
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "failed to check access")
		return false
	}
	return true
}

// parseCSVQueryParam splits a comma-separated query value into a trimmed,
// de-duplicated slice of non-empty tokens. Returns nil for empty input so
// callers can treat "no filter" uniformly.
func parseCSVQueryParam(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
