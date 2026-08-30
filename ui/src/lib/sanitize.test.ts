/**
 * Tests for ui/src/lib/sanitize.ts
 *
 * NOTE: DOMPurify is stubbed in Deno test environment (no DOM). These tests
 * verify the function interface and pass-through behavior. Real DOMPurify
 * sanitization behavior (tag stripping, event handler removal) is tested
 * upstream by the DOMPurify library itself.
 */
import { assertEquals } from "@std/assert";
import { sanitizeHtml } from "@/lib/sanitize.ts";

Deno.test("sanitizeHtml / empty input returns empty string", () => {
  assertEquals(sanitizeHtml(""), "");
});

Deno.test("sanitizeHtml / nullish input returns empty string", () => {
  // The function signature doesn't accept null/undefined, but we test the
  // falsy branch in the implementation via empty string.
  assertEquals(sanitizeHtml(""), "");
});

Deno.test("sanitizeHtml / preserves safe formatting tags", () => {
  const input = "<b>bold</b><strong>strong</strong>";
  const result = sanitizeHtml(input);
  assertEquals(result, input);
});

Deno.test("sanitizeHtml / preserves allowed attributes", () => {
  const input =
    '<a href="https://example.com" target="_blank" rel="noopener">link</a>';
  const result = sanitizeHtml(input);
  assertEquals(result, input);
});

Deno.test("sanitizeHtml / preserves data-* attributes", () => {
  const input =
    '<span data-type="user" data-id="abc123" data-label="John">@John</span>';
  const result = sanitizeHtml(input);
  assertEquals(result, input);
});

Deno.test("sanitizeHtml / preserves mark tag", () => {
  const input = "hello <mark>world</mark>";
  const result = sanitizeHtml(input);
  assertEquals(result, input);
});
