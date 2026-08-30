import { signal } from "@preact/signals-core";

export const currentPath = signal<string>(window.location.pathname);

window.addEventListener("popstate", () => {
  currentPath.value = window.location.pathname;
});

export function navigate(to: string): void {
  const url = new URL(to, window.location.origin);
  if (
    url.pathname === window.location.pathname &&
    url.search === window.location.search
  ) {
    return;
  }
  window.history.pushState(null, "", url.pathname + url.search + url.hash);
  currentPath.value = url.pathname;
  window.scrollTo(0, 0);
}

/** pattern "/projects/:id" → { id: value } (keys have no colon prefix) */
export function matchRoute(
  pattern: string,
  path: string,
): Record<string, string> | null {
  const pSegs = pattern.split("/").filter(Boolean);
  const aSegs = path.split("/").filter(Boolean);
  if (pSegs.length !== aSegs.length) return null;
  const params: Record<string, string> = {};
  for (let i = 0; i < pSegs.length; i++) {
    const p = pSegs[i];
    const a = aSegs[i];
    if (p.startsWith(":")) {
      try {
        params[p.slice(1)] = decodeURIComponent(a);
      } catch {
        return null;
      }
    } else if (p !== a) return null;
  }
  return params;
}
