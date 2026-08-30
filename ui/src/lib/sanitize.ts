import DOMPurify from "dompurify";

/**
 * Sanitize HTML string against XSS using DOMPurify.
 * Allows safe formatting tags but strips event handlers, javascript: URLs,
 * and dangerous elements.
 */
export function sanitizeHtml(html: string): string {
  if (!html) return "";
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS: [
      "b",
      "i",
      "em",
      "strong",
      "s",
      "del",
      "code",
      "pre",
      "a",
      "br",
      "p",
      "ul",
      "ol",
      "li",
      "blockquote",
      "span",
      "img",
      "div",
      "h1",
      "h2",
      "h3",
      "h4",
      "h5",
      "h6",
      "mark",
    ],
    ALLOWED_ATTR: [
      "href",
      "target",
      "rel",
      "src",
      "alt",
      "class",
      "data-type",
      "data-id",
      "data-label",
    ],
    ALLOW_DATA_ATTR: true,
  });
}
