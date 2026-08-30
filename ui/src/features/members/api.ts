import { identify } from "@/lib/sdk-helpers";
import {
  deleteInvitesById,
  getInvites,
  getInvitesByTokenValidate,
  getUsers,
  postInvites,
  postInvitesByTokenAccept,
  putUsersByIdActive,
  putUsersByIdRole,
} from "@/api";
import type { DtoCreateInviteRequest, DtoPaginatedUsersResponse } from "@/api";

export interface InviteItem {
  id: string;
  role: string;
  email?: string | null;
  invited_by: string;
  invited_by_name: string;
  use_count: number;
  expires_at: string;
  created_at: string;
}

export interface InviteCreated {
  id: string;
  token: string;
  url: string;
  role: string;
  expires_at: string;
}

export interface InviteValidated {
  id: string;
  role: string;
  email?: string | null;
  expires_at: string;
}

export const membersApi = {
  /** List workspace members (active by default, use include_inactive for deactivated). */
  async list(params?: {
    cursor?: string;
    search?: string;
    role?: string;
    include_inactive?: boolean;
    limit?: number;
  }): Promise<DtoPaginatedUsersResponse> {
    const query: Record<string, string> = {};
    if (params?.cursor) query.cursor = params.cursor;
    if (params?.search) query.search = params.search;
    if (params?.role) query.role = params.role;
    if (params?.include_inactive) query.include_inactive = "true";
    if (params?.limit) query.limit = String(params.limit);
    const { data } = await getUsers({ query, throwOnError: true });
    return data;
  },

  /** Update a user's org role. */
  async updateRole(
    id: string,
    role: "owner" | "admin" | "member" | "viewer" | "guest",
  ) {
    const { data } = await putUsersByIdRole({
      path: { id },
      body: { role },
      throwOnError: true,
    });
    return data;
  },

  /** Activate or deactivate a user. */
  async updateActive(id: string, is_active: boolean) {
    const { data } = await putUsersByIdActive({
      path: { id },
      body: { is_active },
      throwOnError: true,
    });
    return data;
  },

  /** Create an invite token. */
  async createInvite(
    body: DtoCreateInviteRequest,
  ): Promise<InviteCreated> {
    const { data } = await postInvites({
      body,
      throwOnError: true,
    });
    return identify<InviteCreated>(data);
  },

  /** List pending invite tokens. */
  async listInvites(): Promise<InviteItem[]> {
    const { data } = await getInvites({ throwOnError: true });
    return identify<InviteItem[]>(data);
  },

  /** Revoke (delete) an invite token. */
  async revokeInvite(id: string): Promise<void> {
    await deleteInvitesById({ path: { id }, throwOnError: true });
  },

  /** Validate an invite token (public endpoint). */
  async validateInvite(token: string): Promise<InviteValidated> {
    const { data } = await getInvitesByTokenValidate({
      path: { token },
      throwOnError: true,
    });
    return identify<InviteValidated>(data);
  },

  /** Accept an invite and register a new user (public endpoint). */
  async acceptInvite(
    token: string,
    body: { name: string; email: string; password: string },
  ) {
    const { data } = await postInvitesByTokenAccept({
      path: { token },
      body,
      throwOnError: true,
    });
    return data;
  },
};
