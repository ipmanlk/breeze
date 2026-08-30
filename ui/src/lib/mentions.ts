import { identify } from "@/lib/sdk-helpers";
import { getMentionsSearch } from "@/api";
import type { DtoMentionResultResponse } from "@/api";

/** Reusable mention result: shared by chat messages and task comments. */
export interface MentionResult extends Omit<DtoMentionResultResponse, "type"> {
  id: string;
  type: string;
  label: string;
}

/**
 * Search mentionable entities (users, @everyone, channels, projects, tasks)
 * for the @-mention popover. Used by both the chat composer and the task
 * comment composer so both support identical pings/mentions.
 */
export async function searchMentions(
  q: string,
  types?: string[],
  limit?: number,
): Promise<{ results: MentionResult[] }> {
  try {
    const { data } = await getMentionsSearch({
      query: {
        q,
        types: types?.length ? types.join(",") : undefined,
        limit,
      },
      throwOnError: true,
    });
    return identify<{ results: MentionResult[] }>(data);
  } catch {
    return { results: [] };
  }
}
