import type { DtoMentionsResponse } from "@/api/types.gen";

/** Regex matching a mention token anywhere in a string. */
export const MENTION_TOKEN_RE = /<@([^:>]+)(?::([^>]+))?>/g;

/** Regex matching a trailing @query in plain text (not inside a token). */
export const TRAILING_AT_RE = /(?<!\\)@(?![\w-]*:)([\w-]*)$/;

/** Escape a string for use as a literal in a RegExp. */
function escapeRe(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/**
 * MentionResolver: maps type+id → display label.
 * Built from a message's `mentions` payload.
 */
export interface MentionResolver {
  users: Record<string, string>;
  channels: Record<string, string>;
  projects: Record<string, string>;
  tasks: Record<string, { title: string; project_id?: string }>;
}

/** Build a resolver from a DtoMentionsResponse. */
export function buildResolver(
  mentions?: DtoMentionsResponse | null,
): MentionResolver {
  const r: MentionResolver = {
    users: {},
    channels: {},
    projects: {},
    tasks: {},
  };
  if (!mentions) return r;
  if (mentions.users) {
    for (const [id, name] of Object.entries(mentions.users)) {
      r.users[id] = (name as string) ?? id;
    }
  }
  if (mentions.channels) {
    for (const [id, name] of Object.entries(mentions.channels)) {
      r.channels[id] = (name as string) ?? id;
    }
  }
  if (mentions.projects) {
    for (const [id, name] of Object.entries(mentions.projects)) {
      r.projects[id] = (name as string) ?? id;
    }
  }
  if (mentions.tasks) {
    for (const [id, t] of Object.entries(mentions.tasks)) {
      const task = t as { title?: string; project_id?: string };
      r.tasks[id] = {
        title: task.title ?? id,
        project_id: task.project_id,
      };
    }
  }
  return r;
}

/** Resolve a mention type+id to a display label. */
export function resolveLabel(
  resolver: MentionResolver | null,
  type: string,
  id: string,
): string {
  if (!resolver) return id;
  switch (type) {
    case "user":
      return resolver.users[id] ?? id;
    case "channel":
      return resolver.channels[id] ?? id;
    case "project":
      return resolver.projects[id] ?? id;
    case "task":
      return resolver.tasks[id]?.title ?? id;
    case "everyone":
      return "everyone";
    default:
      return id;
  }
}

/**
 * Entry tracking a single mention inserted into the textarea.
 * `@label` is the visible text; `token` is what we send to the API.
 */
export interface MentionEntry {
  type: string;
  id: string;
  label: string;
  token: string;
}

/** Build a mention token string from type + id. */
export function makeToken(type: string, id: string): string {
  return type === "everyone" ? "<@everyone>" : `<@${type}:${id}>`;
}

/**
 * Convert textarea content (with @label visible text) to API content
 * (with <@type:id> tokens), using the provided entries to map labels.
 *
 * Also strips backslash-escaped @ (e.g. `\@hello` → `@hello`).
 */
export function contentToTokens(
  textareaValue: string,
  entries: MentionEntry[],
): string {
  let result = textareaValue;
  // Replace longest labels first to avoid partial-match collisions
  const sorted = [...entries].sort((a, b) => b.label.length - a.label.length);
  for (const entry of sorted) {
    const visible = `@${entry.label}`;
    // Escape regex special chars in label
    result = result.replace(
      new RegExp(escapeRe(visible), "g"),
      entry.token,
    );
  }
  // Strip backslash-escaped @
  result = result.replace(/\\@/g, "@");
  return result;
}

/**
 * Convert stored markdown content (with tokens) to textarea display text
 * (with @label visible text), and extract mention entries for round-tripping.
 *
 * Returns { text, entries }: entries can be passed to contentToTokens
 * when saving the edit.
 */
export function tokensToContent(
  content: string,
  resolver: MentionResolver,
): { text: string; entries: MentionEntry[] } {
  const entries: MentionEntry[] = [];
  const seen = new Set<string>();
  let text = content.replace(
    MENTION_TOKEN_RE,
    (match, type: string, id?: string) => {
      const t = type || "everyone";
      const i = id ?? "";
      const key = `${t}:${i}`;
      const label = resolveLabel(resolver, t, i);
      if (!seen.has(key)) {
        seen.add(key);
        entries.push({
          type: t,
          id: i,
          label,
          token: match,
        });
      }
      return `@${label}`;
    },
  );
  return { text, entries };
}
