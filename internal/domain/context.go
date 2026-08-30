package domain

type ContextKey string

const (
	CtxUserID        ContextKey = "user_id"
	CtxOrgID         ContextKey = "org_id"
	CtxRole          ContextKey = "role"
	CtxSessionID     ContextKey = "session_id"
	CtxEffectiveRole ContextKey = "effective_role"
	// CtxEffectiveRoleProjectID records which project the stashed effective
	// role was resolved for, so the service-layer fast path can't mistake a
	// role cached for one project as authorization for another.
	CtxEffectiveRoleProjectID ContextKey = "effective_role_project_id"
	CtxLocale                 ContextKey = "locale"
)
