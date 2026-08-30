/**
 * Role + permission helpers for the UI.
 *
 * The backend is the single source of truth for the role→permission map. The
 * frontend never duplicates it: for project-scoped UI we read the effective
 * permission set from `GET /projects/{id}/my-access` and test membership; for
 * org-scoped UI we classify the org role with the helpers below.
 *
 * These helpers replace ad-hoc `role === "owner" || role === "admin"` checks
 * scattered across components.
 */

export type Role = "owner" | "admin" | "member" | "viewer" | "guest";

/** Org roles that carry implicit access to every project. */
const ORG_ELEVATED_ROLES: ReadonlySet<Role> = new Set([
  "owner",
  "admin",
  "member",
]);

/** Org roles that are project-scoped (access only via explicit membership). */
const PROJECT_SCOPED_ROLES: ReadonlySet<Role> = new Set(["viewer", "guest"]);

/** Org roles that can manage org-level resources (members, settings). */
const ORG_MANAGER_ROLES: ReadonlySet<Role> = new Set(["owner", "admin"]);

function asRole(role: string | undefined | null): Role | undefined {
  return role as Role | undefined;
}

/** True for owner/admin/member: implicit, org-wide project access. */
export function isOrgElevatedRole(role: string | undefined | null): boolean {
  const r = asRole(role);
  return r !== undefined && ORG_ELEVATED_ROLES.has(r);
}

/** True for viewer/guest: access is per-project and the role is overridable. */
export function isProjectScopedRole(role: string | undefined | null): boolean {
  const r = asRole(role);
  return r !== undefined && PROJECT_SCOPED_ROLES.has(r);
}

/** True for owner/admin: can manage org members, settings, and invites. */
export function canManageOrg(role: string | undefined | null): boolean {
  const r = asRole(role);
  return r !== undefined && ORG_MANAGER_ROLES.has(r);
}

/**
 * Project-scoped permission strings, mirrored from the Go `domain.Permission`
 * constants. Used only as typed keys for `hasProjectPermission` lookups: the
 * actual grant decision comes from the backend's `my-access` response.
 */
export const ProjectPermission = {
  ProjectView: "project:view",
  ProjectManage: "project:manage",
  ProjectDelete: "project:delete",
  ProjectStatusManage: "project:status.manage",
  ProjectCycleManage: "project:cycle.manage",
  ProjectMembersManage: "project:members.manage",
  TaskCreate: "task:create",
  TaskEdit: "task:edit",
  TaskDelete: "task:delete",
  TaskView: "task:view",
  AttachmentCreate: "attachment:create",
  AttachmentDelete: "attachment:delete",
  TimeCreate: "time:create",
  TimeDelete: "time:delete",
} as const;

export type ProjectPermissionKey =
  (typeof ProjectPermission)[keyof typeof ProjectPermission];
