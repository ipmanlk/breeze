import { marked } from "marked";
import type { DtoMentionsResponse } from "@/api/types.gen";
import {
  buildResolver,
  type MentionResolver,
  resolveLabel,
} from "@/lib/mention-utils";
import { sanitizeHtml } from "@/lib/sanitize";

// ---------------------------------------------------------------------------
// Mention rendering: a marked inline extension converts <@type:id> /
// <@everyone> tokens into styled chips with optional anchor links.
// ---------------------------------------------------------------------------

let currentResolver: MentionResolver | null = null;

const MENTION_SYMBOLS: Record<string, string> = {
  user: "@",
  everyone: "@",
  channel: "#",
  project: "📁",
  task: "📋",
};

const MENTION_CLASSES: Record<string, string> = {
  user: "mention-user",
  everyone: "mention-everyone",
  channel: "mention-channel",
  project: "mention-project",
  task: "mention-task",
};

function escapeHtml(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

const mentionExt = {
  name: "mention",
  level: "inline" as const,
  start(src: string) {
    return src.indexOf("<@");
  },
  tokenizer(src: string) {
    const rule = /^<@([^:>]+)(?::([^>]+))?>/;
    const match = rule.exec(src);
    if (match) {
      return {
        type: "mention" as const,
        raw: match[0],
        mentionType: match[1],
        mentionId: match[2] ?? "",
      };
    }
    return undefined;
  },
  renderer(token: { mentionType: string; mentionId: string }) {
    const type = token.mentionType || "everyone";
    const id = token.mentionId || "";
    // @everyone is a literal mention (no id lookup): always labelled
    // "everyone" regardless of resolver availability.
    const label = type === "everyone"
      ? "everyone"
      : resolveLabel(currentResolver, type, id);
    const symbol = MENTION_SYMBOLS[type] || "@";
    const cls = MENTION_CLASSES[type] || "mention-user";

    const chip = `<span class="mention-chip ${cls}" data-type="${
      escapeHtml(type)
    }" data-id="${escapeHtml(id)}">${symbol}${escapeHtml(label)}</span>`;

    // Users and @everyone are plain chips (not links)
    if (type === "user" || type === "everyone") return chip;

    let href = "";
    if (type === "channel") href = `/chat/${encodeURIComponent(id)}`;
    else if (type === "project") href = `/projects/${encodeURIComponent(id)}`;
    else if (type === "task") {
      const pid = currentResolver?.tasks?.[id]?.project_id;
      if (pid) {
        href = `/projects/${encodeURIComponent(pid)}?task=${
          encodeURIComponent(id)
        }`;
      }
    }

    return href ? `<a href="${href}" class="mention-link">${chip}</a>` : chip;
  },
};

// ---------------------------------------------------------------------------
// Link renderer: add target="_blank" rel="noopener noreferrer" to all
// external links (matching the previous hand-rolled behaviour), and drop
// dangerous URL protocols (javascript:, data:, vbscript:) as a
// pre-sanitization defense-in-depth layer. DOMPurify strips these too in
// the browser, but blocking them here means the renderer never emits a
// clickable dangerous link even if the sanitize pass is ever bypassed.
// ---------------------------------------------------------------------------

/** Returns true for URL protocols that are safe to render as href. */
function isSafeUrl(url: string): boolean {
  const trimmed = url.trim().toLowerCase();
  if (!trimmed) return false;
  // Safe explicit protocols
  if (/^(https?:\/\/|mailto:|tel:)/.test(trimmed)) return true;
  // Relative URLs / fragment / query: no protocol, safe
  if (!/^[a-z][a-z0-9+.-]*:/.test(trimmed)) return true;
  return false;
}

const linkRenderer = {
  link({ href, text }: { href: string; text: string }) {
    // Unsafe protocol (javascript:, data:, vbscript:): render the link text
    // without an href so nothing dangerous is clickable.
    if (!isSafeUrl(href)) {
      return text;
    }
    return `<a href="${href}" target="_blank" rel="noopener noreferrer">${text}</a>`;
  },
};

marked.use({
  breaks: true,
  gfm: true,
  extensions: [mentionExt],
  renderer: linkRenderer,
});

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/**
 * Render markdown (with optional @-mention resolution) to sanitized HTML.
 *
 * Uses `marked` under the hood, which is the same library `@tiptap/markdown`
 * builds on: the task editor and this renderer now use the same parsing
 * engine, eliminating the old hand-rolled regex pipeline.
 */
export function renderMarkdownWithMentions(
  md: string,
  mentions?: DtoMentionsResponse | null,
): string {
  if (!md) return "";
  currentResolver = mentions ? buildResolver(mentions) : null;
  try {
    const html = marked.parse(md) as string;
    return sanitizeHtml(html);
  } finally {
    // Always clear the resolver, even if marked.parse throws, so a stale
    // resolver can never leak into a subsequent render.
    currentResolver = null;
  }
}

/**
 * Render plain markdown (no mention resolution) to sanitized HTML.
 */
export function renderMarkdown(md: string): string {
  return renderMarkdownWithMentions(md, null);
}
