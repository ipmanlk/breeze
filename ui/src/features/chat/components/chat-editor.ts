import { css, html, LitElement } from "lit";
import { customElement, property, query } from "lit/decorators.js";
import { localized } from "@lit/localize";
import {
  type MentionResolver,
  resolveLabel,
  TRAILING_AT_RE,
} from "../mention-utils";

const MENTION_TOKEN_SINGLE_RE = /^<@([^:>]+)(?::([^>]+))?>$/;

const MENTION_SYMBOLS: Record<string, string> = {
  user: "@",
  everyone: "@",
  channel: "#",
  project: "📁",
  task: "📋",
};

// DOM ↔ markdown serialization

function serializeFromDom(root: HTMLElement): string {
  let text = "";
  const visit = (node: Node): void => {
    if (node.nodeType === Node.TEXT_NODE) {
      text += node.textContent ?? "";
      return;
    }
    if (node.nodeType === Node.ELEMENT_NODE) {
      const el = node as HTMLElement;
      if (el.classList.contains("mention-chip")) {
        const type = el.dataset.type ?? "";
        const id = el.dataset.id ?? "";
        text += type === "everyone" ? "<@everyone>" : `<@${type}:${id}>`;
        return;
      }
      if (el.tagName === "BR") {
        text += "\n";
        return;
      }
      if (el.tagName === "DIV") {
        for (const child of Array.from(el.childNodes)) visit(child);
        if (text.length > 0 && !text.endsWith("\n")) text += "\n";
        return;
      }
      for (const child of Array.from(el.childNodes)) visit(child);
    }
  };
  for (const child of Array.from(root.childNodes)) visit(child);
  return text.trimEnd();
}

function buildDomFragment(
  markdown: string,
  resolve: (type: string, id: string) => string,
): DocumentFragment {
  const frag = document.createDocumentFragment();
  const parts = markdown.split(/(<@[^>]+>)/g);
  for (const part of parts) {
    if (!part) continue;
    if (part.startsWith("<@") && part.endsWith(">")) {
      const m = MENTION_TOKEN_SINGLE_RE.exec(part);
      if (m) {
        const type = m[1];
        const id = m[2] ?? "";
        const span = document.createElement("span");
        span.className = `mention-chip mention-${type}`;
        span.contentEditable = "false";
        span.dataset.type = type;
        span.dataset.id = id;
        span.textContent = (MENTION_SYMBOLS[type] ?? "@") + resolve(type, id);
        frag.appendChild(span);
        frag.appendChild(document.createTextNode(" "));
        continue;
      }
    }
    const lines = part.split("\n");
    for (let i = 0; i < lines.length; i++) {
      if (i > 0) frag.appendChild(document.createElement("br"));
      if (lines[i]) frag.appendChild(document.createTextNode(lines[i]));
    }
  }
  return frag;
}

// Cursor / chip helpers

function getChipAtCursor(root: Node): HTMLElement | null {
  const sel = window.getSelection();
  if (!sel || sel.rangeCount === 0) return null;
  const node = sel.getRangeAt(0).startContainer;
  let el: Node | null = node;
  while (el && el !== root) {
    if (
      el.nodeType === Node.ELEMENT_NODE &&
      (el as HTMLElement).classList.contains("mention-chip")
    ) {
      return el as HTMLElement;
    }
    el = el.parentNode;
  }
  return null;
}

function getLastChipBeforeCursor(root: HTMLElement): HTMLElement | null {
  const sel = window.getSelection();
  if (!sel || sel.rangeCount === 0) return null;
  const range = sel.getRangeAt(0);
  if (!range.collapsed || range.startOffset !== 0) return null;

  let prev: Node | null = range.startContainer;
  while (prev && prev !== root) {
    const sibling = prev.previousSibling;
    if (sibling) {
      if (
        sibling.nodeType === Node.ELEMENT_NODE &&
        (sibling as HTMLElement).classList.contains("mention-chip")
      ) {
        return sibling as HTMLElement;
      }
      let last: Node | null = sibling;
      while (last?.lastChild) last = last.lastChild;
      if (
        last &&
        last.nodeType === Node.ELEMENT_NODE &&
        (last as HTMLElement).classList.contains("mention-chip")
      ) {
        return last as HTMLElement;
      }
      break;
    }
    prev = prev.parentNode;
  }
  return null;
}

function placeCursorAtEnd(root: HTMLElement): void {
  const sel = window.getSelection();
  if (!sel) return;
  const range = document.createRange();
  range.selectNodeContents(root);
  range.collapse(false);
  sel.removeAllRanges();
  sel.addRange(range);
}

// Custom element

@localized()
@customElement("breeze-chat-editor")
export class BreezeChatEditor extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }

    :host {
      display: block;
    }

    .editor {
      min-height: var(--space-20);
      max-height: var(--editor-maxh);
      overflow-y: auto;
      border: 1px solid var(--input);
      border-radius: var(--radius-md);
      background: var(--background);
      padding: var(--space-2) var(--space-3);
      font-size: var(--text-sm);
      font-family: inherit;
      line-height: 1.5;
      color: var(--foreground);
      outline: none;
      word-wrap: break-word;
      overflow-wrap: break-word;
      transition: border-color var(--dur-fast) var(--ease-1);
    }

    .editor:focus {
      border-color: var(--ring);
      box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 40%, transparent);
    }

    .editor:empty::before {
      content: attr(data-placeholder);
      color: var(--muted-foreground);
      pointer-events: none;
    }

    .mention-chip {
      display: inline-block;
      border-radius: var(--radius-sm);
      padding: 0 var(--space-1);
      font-size: var(--text-sm);
      font-weight: 500;
      cursor: default;
      user-select: none;
    }

    .mention-user,
    .mention-everyone {
      background: color-mix(in oklch, var(--primary) 15%, transparent);
      color: var(--primary);
    }

    .mention-channel {
      background: color-mix(in oklch, #6366f1 15%, transparent);
      color: light-dark(#4f46e5, #a5b4fc);
    }

    .mention-project {
      background: color-mix(in oklch, #f59e0b 15%, transparent);
      color: light-dark(#b45309, #fcd34d);
    }

    .mention-task {
      background: color-mix(in oklch, #10b981 15%, transparent);
      color: light-dark(#059669, #6ee7b7);
    }
  `;

  @property()
  placeholder = "";

  @property({ type: Boolean })
  disabled = false;

  /** When true, Enter does NOT send: popover handles it. */
  @property({ type: Number })
  maxlength?: number;

  @property({ type: Boolean, attribute: "suggest-open" })
  suggestOpen = false;

  @query(".editor")
  private _editor!: HTMLDivElement;

  #resolver: MentionResolver | null = null;

  /** Cache of type:id → label for mentions inserted during this session. */
  #labelCache = new Map<string, string>();

  // Public API

  /** Serialize the editor DOM back to markdown (with <@type:id> tokens). */
  getContent(): string {
    return this._editor ? serializeFromDom(this._editor) : "";
  }

  /** Replace editor content with deserialized markdown. */
  setContent(markdown: string, resolver?: MentionResolver | null): void {
    if (!this._editor) return;
    this.#resolver = resolver ?? null;
    this.#labelCache.clear();
    this._editor.replaceChildren();
    this._editor.appendChild(
      buildDomFragment(markdown, (type, id) => this.#resolve(type, id)),
    );
  }

  /** Insert a mention chip at the current cursor position. */
  insertMention(type: string, id: string, label: string): void {
    const el = this._editor;
    if (!el) return;

    // Cache the label so deserialization resolves to the display name
    // instead of the raw UUID, even without a resolver.
    this.#labelCache.set(`${type}:${id}`, label);

    // Serialize → strip @query → append token → deserialize.
    // Working at the markdown level avoids fragile DOM text-node surgery.
    let md = serializeFromDom(el);
    md = md.replace(TRAILING_AT_RE, "");

    const token = type === "everyone" ? "<@everyone>" : `<@${type}:${id}>`;
    md = md + token + " ";

    el.replaceChildren();
    el.appendChild(
      buildDomFragment(md, (t, i) => this.#resolve(t, i)),
    );

    placeCursorAtEnd(el);
    this.#emitChange();
  }

  focus(): void {
    this._editor?.focus();
  }

  clear(): void {
    if (!this._editor) return;
    this.#labelCache.clear();
    this._editor.replaceChildren();
    this.#emitChange();
  }

  /** Get approximate caret horizontal position for popover placement. */
  getCaretPosition(): { left: number } | null {
    if (!this._editor) return null;
    const sel = window.getSelection();
    if (!sel || sel.rangeCount === 0) return null;
    const range = sel.getRangeAt(0).cloneRange();
    const editorRect = this._editor.getBoundingClientRect();

    let rect = range.getBoundingClientRect();
    if (rect.left === 0 && rect.top === 0 && rect.width === 0) {
      const rects = range.getClientRects();
      if (rects.length > 0) rect = rects[0];
    }
    if (rect.left === 0 && rect.top === 0) return null;

    return { left: Math.max(8, rect.left - editorRect.left) };
  }

  // Internal

  #resolve(type: string, id: string): string {
    const key = `${type}:${id}`;
    if (this.#labelCache.has(key)) return this.#labelCache.get(key)!;
    return resolveLabel(this.#resolver, type, id);
  }

  #emitChange(): void {
    this.dispatchEvent(
      new CustomEvent("breeze-change", {
        detail: { content: this.getContent() },
        bubbles: true,
        composed: true,
      }),
    );
  }

  #onInput(): void {
    if (this.maxlength != null && this.maxlength > 0) {
      const content = this.getContent();
      if (content.length > this.maxlength) {
        this.setContent(content.slice(0, this.maxlength));
        placeCursorAtEnd(this._editor);
      }
    }
    this.#emitChange();
  }

  #onKeydown(e: KeyboardEvent): void {
    // Backspace / Delete: delete whole chip if cursor is at chip boundary
    if (e.key === "Backspace" || e.key === "Delete") {
      const chip = getChipAtCursor(this._editor) ??
        getLastChipBeforeCursor(this._editor);
      if (chip) {
        e.preventDefault();
        const next = chip.nextSibling;
        chip.remove();
        // Also remove trailing space that was added after the chip
        if (
          next &&
          next.nodeType === Node.TEXT_NODE &&
          (next.textContent ?? "").startsWith(" ")
        ) {
          const text = next.textContent ?? "";
          if (text.length === 1) {
            next.remove();
          } else {
            (next as Text).textContent = text.slice(1);
          }
        }
        placeCursorAtEnd(this._editor);
        this.#emitChange();
        return;
      }
    }

    // Arrow keys: skip over chips
    if (e.key === "ArrowLeft" || e.key === "ArrowRight") {
      const chip = getChipAtCursor(this._editor);
      if (chip) {
        e.preventDefault();
        const range = document.createRange();
        if (e.key === "ArrowLeft") {
          range.setStartBefore(chip);
          range.collapse(true);
        } else {
          range.setStartAfter(chip);
          range.collapse(true);
        }
        const sel = window.getSelection();
        sel?.removeAllRanges();
        sel?.addRange(range);
        return;
      }
    }

    // Enter sends (when popover is not open)
    if (e.key === "Enter" && !e.shiftKey && !this.suggestOpen) {
      e.preventDefault();
      this.dispatchEvent(
        new CustomEvent("breeze-send", {
          bubbles: true,
          composed: true,
        }),
      );
      return;
    }

    // Escape: bubble to parent
    if (e.key === "Escape") {
      this.dispatchEvent(
        new CustomEvent("breeze-escape", {
          bubbles: true,
          composed: true,
        }),
      );
    }
  }

  #onPaste(e: ClipboardEvent): void {
    e.preventDefault();
    const text = e.clipboardData?.getData("text/plain") ?? "";
    document.execCommand("insertText", false, text);
    this.#emitChange();
  }

  protected render() {
    return html`
      <div
        class="editor"
        contenteditable="${this.disabled ? "false" : "true"}"
        data-placeholder="${this.placeholder}"
        @input="${this.#onInput}"
        @keydown="${this.#onKeydown}"
        @paste="${this.#onPaste}"
      ></div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-chat-editor": BreezeChatEditor;
  }
}
