/**
 * DOMPurify stub for Deno test environments.
 * The real DOMPurify requires a browser DOM (document.createElement,
 * createHTMLDocument) which isn't available in Deno's runtime.
 *
 * This stub mirrors the DOMPurify.sanitize() interface but acts as a
 * pass-through. Security assertions on the final sanitized output rely on
 * the real DOMPurify library in the browser; this stub lets us verify the
 * markdown transformation pipeline and pre-sanitization layer.
 */

interface DOMPurifyConfig {
  ALLOWED_TAGS?: string[];
  ALLOWED_ATTR?: string[];
  ALLOW_DATA_ATTR?: boolean;
}

export function sanitize(html: string, _config?: DOMPurifyConfig): string {
  return html;
}

const DOMPurify = {
  sanitize,
  isSupported: false,
};

export default DOMPurify;
