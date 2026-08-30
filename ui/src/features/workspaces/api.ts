import { postWorkspaces, postWorkspacesByIdSwitch } from "@/api";
import type { DtoUserResponse, DtoWorkspaceResponse } from "@/api";

/** Create a new workspace; the caller becomes its owner. Returns the
 * created workspace so the caller can switch to it by ID. */
export async function createWorkspace(
  name: string,
): Promise<DtoWorkspaceResponse> {
  const { data } = await postWorkspaces({
    body: { name },
    throwOnError: true,
  });
  return data ?? null;
}

/**
 * Switch the active workspace. Returns the refreshed /auth/me-shaped response
 * (new session cookie is set server-side). The caller updates auth + workspace
 * signals from it.
 */
export async function switchWorkspace(
  orgID: string,
): Promise<DtoUserResponse> {
  const { data } = await postWorkspacesByIdSwitch({
    path: { id: orgID },
    throwOnError: true,
  });
  return data ?? null;
}

export type { DtoWorkspaceResponse };
