import type { DtoVoiceParticipantResponse } from "@/api/types.gen";

export interface VoiceParticipant {
  id: string;
  user_id: string;
  name: string;
  avatar_url?: string;
  muted: boolean;
  deafened: boolean;
  speaking: boolean;
  joined_at: string;
}

export function toVoiceParticipant(
  dto: DtoVoiceParticipantResponse,
): VoiceParticipant {
  return {
    id: dto.id ?? "",
    user_id: dto.user_id ?? "",
    name: dto.name ?? "",
    avatar_url: dto.avatar_url ?? undefined,
    muted: dto.muted ?? false,
    deafened: dto.deafened ?? false,
    speaking: dto.speaking ?? false,
    joined_at: dto.joined_at ?? "",
  };
}
