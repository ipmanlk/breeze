/**
 * Global keyboard shortcuts.
 *
 * Installed once on the document. Shortcuts are ignored when the user is
 * typing in an input, textarea, or contenteditable element (so they don't
 * interfere with text entry).
 *
 * Works in any context:
 *   /         : focus search / open command palette
 *   ?         : show keyboard shortcuts help (dispatches "show-shortcuts" event)
 *
 * Works on project-detail page:
 *   c         : create task (dispatches "create-task" event)
 *
 * Navigation (works globally):
 *   g then p  : go to projects
 *   g then i  : go to inbox
 *   g then m  : go to my-tasks
 *   g then d  : go to dashboard
 *
 * Component-level shortcuts (listeners in specific components):
 *   j/k       : navigate down/up in a task list (list-view.ts)
 */

import { navigate } from "@/routes/router";

let installed = false;
let gPressed = false;
let gTimer: ReturnType<typeof setTimeout> | null = null;

function isTyping(e: KeyboardEvent): boolean {
  // Walk the composed path so shortcuts are also ignored when the actual
  // focused element is inside a shadow-DOM input (e.g. <breeze-input>).
  for (const node of e.composedPath()) {
    if (!(node instanceof HTMLElement)) continue;
    const tag = node.tagName;
    if (
      tag === "INPUT" ||
      tag === "TEXTAREA" ||
      tag === "SELECT" ||
      node.isContentEditable
    ) {
      return true;
    }
  }
  return false;
}

function onKeydown(e: KeyboardEvent): void {
  // Esc is always handled (even in inputs): let dialogs close.
  if (e.key === "Escape") return;

  if (isTyping(e)) return;

  // Don't interfere with modifier combos (Cmd/Ctrl/Alt).
  if (e.metaKey || e.ctrlKey || e.altKey) return;

  // "g" prefix: wait for the next key.
  if (e.key === "g" && !gPressed) {
    gPressed = true;
    if (gTimer) clearTimeout(gTimer);
    gTimer = setTimeout(() => {
      gPressed = false;
    }, 800);
    return;
  }

  if (gPressed) {
    gPressed = false;
    if (gTimer) {
      clearTimeout(gTimer);
      gTimer = null;
    }
    switch (e.key) {
      case "p":
        e.preventDefault();
        navigate("/projects");
        return;
      case "i":
        e.preventDefault();
        navigate("/inbox");
        return;
      case "m":
        e.preventDefault();
        navigate("/my-tasks");
        return;
      case "d":
        e.preventDefault();
        navigate("/");
        return;
    }
    return;
  }

  switch (e.key) {
    case "j":
      e.preventDefault();
      document.dispatchEvent(new CustomEvent("shortcut-next"));
      return;
    case "k":
      e.preventDefault();
      document.dispatchEvent(new CustomEvent("shortcut-prev"));
      return;
    case "c":
      e.preventDefault();
      document.dispatchEvent(new CustomEvent("create-task"));
      return;
    case "/":
      e.preventDefault();
      document.dispatchEvent(new CustomEvent("open-command-palette"));
      return;
    case "?":
      e.preventDefault();
      document.dispatchEvent(new CustomEvent("show-shortcuts"));
      return;
  }
}

export function installShortcuts(): void {
  if (installed) return;
  installed = true;
  document.addEventListener("keydown", onKeydown);
}
