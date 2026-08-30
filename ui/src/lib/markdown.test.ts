/**
 * Tests for ui/src/lib/markdown.ts
 *
 * NOTE: DOMPurify is stubbed in Deno test environment (no DOM). These tests
 * verify the markdown → HTML transformation pipeline (powered by `marked`).
 * Full XSS protection (javascript: URL stripping, attribute-breakout
 * sanitization) is provided by DOMPurify in the browser and covered by its
 * own test suite, not duplicated here.
 */
import { assertEquals, assertMatch, assertNotMatch } from "@std/assert";
import { renderMarkdown, renderMarkdownWithMentions } from "@/lib/markdown.ts";

// ── renderMarkdownWithMentions ──────────────────────────────────────

Deno.test("renderMarkdownWithMentions / empty input", () => {
  assertEquals(renderMarkdownWithMentions(""), "");
  assertEquals(renderMarkdownWithMentions(null as unknown as string), "");
  assertEquals(renderMarkdownWithMentions(undefined as unknown as string), "");
});

Deno.test("renderMarkdownWithMentions / bold text", () => {
  const result = renderMarkdownWithMentions("**hello world**");
  assertMatch(result, /<strong>hello world<\/strong>/);
});

Deno.test("renderMarkdownWithMentions / bold with underscores", () => {
  const result = renderMarkdownWithMentions("__hello world__");
  assertMatch(result, /<strong>hello world<\/strong>/);
});

Deno.test("renderMarkdownWithMentions / inline code", () => {
  const result = renderMarkdownWithMentions("use `code` here");
  assertMatch(result, /<code>code<\/code>/);
});

Deno.test("renderMarkdownWithMentions / code block", () => {
  const result = renderMarkdownWithMentions("```\nconst x = 1;\n```");
  assertMatch(result, /<pre><code>/);
  assertMatch(result, /const x = 1;/);
});

Deno.test("renderMarkdownWithMentions / strikethrough", () => {
  const result = renderMarkdownWithMentions("~~strike~~");
  // GFM strikethrough renders as <del>; DOMPurify allows both <s> and <del>.
  assertMatch(result, /<(?:s|del)>strike<\/(?:s|del)>/);
});

Deno.test("renderMarkdownWithMentions / safe link", () => {
  const result = renderMarkdownWithMentions(
    "[click](https://example.com/page)",
  );
  assertMatch(result, /<a href="https:\/\/example\.com\/page"/);
  assertMatch(result, /target="_blank"/);
  assertMatch(result, />click<\/a>/);
});

Deno.test("renderMarkdownWithMentions / safe link with mailto", () => {
  const result = renderMarkdownWithMentions(
    "[email](mailto:test@example.com)",
  );
  assertMatch(result, /<a href="mailto:test@example.com"/);
});

// XSS-related urls: the marked link renderer drops dangerous protocols
// (javascript:, data:, vbscript:) as a pre-sanitization defense-in-depth
// layer: the href is removed so nothing dangerous is clickable, even with
// the Deno DOMPurify stub (a pass-through). DOMPurify provides the same
// protection in the browser as a second layer.

Deno.test("renderMarkdownWithMentions / javascript: link is dropped pre-sanitization (defense-in-depth)", () => {
  const result = renderMarkdownWithMentions(
    "[click](javascript:alert(1))",
  );
  // The link renderer drops the dangerous href entirely (defense-in-depth
  // before DOMPurify). The visible text survives but is not clickable.
  assertNotMatch(result, /javascript:/i);
  assertNotMatch(result, /<a\b[^>]*href="javascript/i);
  assertMatch(result, /click/);
});

Deno.test("renderMarkdownWithMentions / data: link is dropped pre-sanitization", () => {
  const result = renderMarkdownWithMentions(
    "[x](data:text/html,<script>alert(1)</script>)",
  );
  assertNotMatch(result, /data:text\/html/i);
});

Deno.test("renderMarkdownWithMentions / allows relative urls", () => {
  const result = renderMarkdownWithMentions("[x](/relative/path)");
  assertMatch(result, /<a href="\/relative\/path"/);
});

Deno.test("renderMarkdownWithMentions / block content wrapped in <p>", () => {
  const result = renderMarkdownWithMentions("hello world");
  // marked wraps block text in <p> tags; chat display renders this inside a
  // <div class="content">, so the paragraph margin is neutralised by CSS.
  // marked may append a trailing newline.
  assertMatch(result, /^<p>hello world<\/p>\n?$/);
});

Deno.test("renderMarkdownWithMentions / renderMarkdown alias matches", () => {
  const a = renderMarkdown("**bold**");
  const b = renderMarkdownWithMentions("**bold**");
  assertEquals(a, b);
});

Deno.test("renderMarkdownWithMentions / mention chip via resolver", () => {
  const result = renderMarkdownWithMentions("hi <@user:u1>", {
    users: { u1: "Alice" },
    projects: {},
    tasks: {},
    channels: {},
  });
  assertMatch(result, /@Alice/);
  assertMatch(result, /mention-chip/);
  assertMatch(result, /mention-user/);
  assertMatch(result, /data-type="user"/);
  assertMatch(result, /data-id="u1"/);
});

Deno.test("renderMarkdownWithMentions / mention chip falls back to id when no resolver", () => {
  const result = renderMarkdownWithMentions("hello <@user:abc>");
  assertMatch(result, /@abc/);
  assertMatch(result, /mention-chip/);
});

Deno.test("renderMarkdownWithMentions / @everyone chip", () => {
  const result = renderMarkdownWithMentions("hey <@everyone>");
  assertMatch(result, /mention-everyone/);
  assertMatch(result, />@everyone</);
});

Deno.test("renderMarkdownWithMentions / channel mention is linked", () => {
  const result = renderMarkdownWithMentions("see <@channel:c1>", {
    users: {},
    projects: {},
    tasks: {},
    channels: { c1: "general" },
  });
  assertMatch(result, /href="\/chat\/c1"/);
  assertMatch(result, /mention-link/);
});

Deno.test("renderMarkdownWithMentions / task mention links to project if resolved", () => {
  const result = renderMarkdownWithMentions("fix <@task:t1>", {
    users: {},
    projects: { p1: "Project X" },
    tasks: { t1: { title: "Fix bug", project_id: "p1" } },
    channels: {},
  });
  assertMatch(result, /href="\/projects\/p1\?task=t1"/);
  assertMatch(result, /mention-link/);
});

Deno.test("renderMarkdownWithMentions / task mention without project_id is plain chip", () => {
  const result = renderMarkdownWithMentions("fix <@task:t1>", {
    users: {},
    projects: {},
    tasks: { t1: { title: "Orphan task", project_id: "" } },
    channels: {},
  });
  assertNotMatch(result, /<a\b/);
});
