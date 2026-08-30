import "./toast-host.ts";

let mounted = false;

/**
 * Append a single <breeze-toast-host> to <body> (idempotent). Called once
 * from app-shell's connectedCallback so the host lives for the whole app
 * session: independent of any page/view lifecycle. Toasts shown via
 * showToast() therefore survive client-side navigation.
 */
export function ensureToastHost(): void {
  if (mounted) return;
  if (document.querySelector("breeze-toast-host")) {
    mounted = true;
    return;
  }
  const el = document.createElement("breeze-toast-host");
  document.body.appendChild(el);
  mounted = true;
}
