/**
 * Tests for ui/src/features/chat/utils.ts
 */
import { assertEquals } from "@std/assert";
import {
  formatRelativeTime,
  sameDay,
  stripHtml,
} from "@/features/chat/utils.ts";

// ── stripHtml ──────────────────────────────────────────────────────

Deno.test("stripHtml / empty string returns empty", () => {
  assertEquals(stripHtml(""), "");
});

Deno.test("stripHtml / removes simple tags", () => {
  assertEquals(stripHtml("<b>bold</b>"), "bold");
});

Deno.test("stripHtml / removes tags with attributes", () => {
  assertEquals(
    stripHtml('<img src="x" onerror="alert(1)">'),
    "",
  );
});

Deno.test("stripHtml / nested tags", () => {
  assertEquals(
    stripHtml("<div><strong>nested</strong></div>"),
    "nested",
  );
});

Deno.test("stripHtml / multiple tags", () => {
  assertEquals(
    stripHtml("before <b>mid</b> after"),
    "before mid after",
  );
});

Deno.test("stripHtml / no HTML returns same string", () => {
  assertEquals(stripHtml("plain text"), "plain text");
});

Deno.test("stripHtml / self-closing tag", () => {
  assertEquals(
    stripHtml("hello<br>world"),
    "helloworld",
  );
});

// ── formatRelativeTime ──────────────────────────────────────────────

Deno.test("formatRelativeTime / 'now' for very recent", () => {
  const result = formatRelativeTime(new Date().toISOString());
  assertEquals(result, "now");
});

// ── sameDay ─────────────────────────────────────────────────────────

Deno.test("sameDay / same timestamp returns true", () => {
  const ts = new Date().toISOString();
  assertEquals(sameDay(ts, ts), true);
});

Deno.test("sameDay / different days returns false", () => {
  const a = "2024-01-01T00:00:00Z";
  const b = "2024-01-02T00:00:00Z";
  assertEquals(sameDay(a, b), false);
});
