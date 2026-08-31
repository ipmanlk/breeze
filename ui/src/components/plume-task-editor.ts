import { Editor, mergeAttributes, Node } from "@tiptap/core";
import { Placeholder } from "@tiptap/extension-placeholder";
import { BubbleMenu } from "@tiptap/extension-bubble-menu";
import StarterKit from "@tiptap/starter-kit";
import { Markdown } from "@tiptap/markdown";
import { localized, msg } from "@lit/localize";
import { css, html, LitElement, nothing } from "lit";
import { customElement, property, query, state } from "lit/decorators.js";
import { TRAILING_AT_RE } from "@/features/chat/mention-utils";
import type { MentionResult } from "@/lib/mentions";
import "@/components/mention/mention-popover.ts";
import "@/components/ui/plume-icon.ts";

// Mention node: an inline atom that stores <@type:id> tokens.
//
// The markdown spec registers a custom tokenizer so `<@user:abc>` and
// `<@everyone>` survive the markdown round-trip unchanged. In the editor the
// node renders as a styled chip (same look as chat chips); when serialized to
// markdown it emits the original `<@type:id>` token.

const MENTION_SYMBOLS: Record<string, string> = {
  user: "@",
  everyone: "@",
  channel: "#",
  project: "📁",
  task: "📋",
};

/** Maps type+id → display label. Mirrors MentionResolver from chat. */
export interface MentionLabelResolver {
  (type: string, id: string): string;
}

/**
 * Create a TipTap Mention node extension.
 *
 * The node is an inline atom that stores `<@type:id>` tokens. A custom
 * marked.js tokenizer recognizes the `<@...>` syntax so tokens survive the
 * markdown round-trip unchanged. `resolveLabel` (optional) lets read-mode
 * editors render human-readable labels instead of raw IDs: when content is
 * parsed, the resolver fills in the `label` attribute.
 */
function createMention(resolveLabel?: MentionLabelResolver) {
  return Node.create<{
    HTMLAttributes: Record<string, unknown>;
  }>({
    name: "mention",

    group: "inline",

    inline: true,

    atom: true,

    selectable: true,

    addAttributes() {
      return {
        type: { default: "user" },
        id: { default: "" },
        label: { default: "" },
      };
    },

    parseHTML() {
      return [
        {
          tag: "span[data-mention]",
        },
      ];
    },

    renderHTML({ node, HTMLAttributes }) {
      const type = (node.attrs.type as string) || "user";
      const symbol = MENTION_SYMBOLS[type] || "@";
      return [
        "span",
        mergeAttributes(HTMLAttributes, {
          "data-mention": "",
          "data-type": type,
          "data-id": node.attrs.id,
          contenteditable: "false",
          class: `mention-chip mention-${type}`,
        }),
        `${symbol}${node.attrs.label}`,
      ];
    },

    markdownTokenName: "mention",
    markdownTokenizer: {
      name: "mention",
      level: "inline",
      start(src: string) {
        return src.indexOf("<@");
      },
      tokenize(src: string) {
        const match = /^<@([^:>]+)(?::([^>]+))?>/.exec(src);
        if (!match) return;
        const type = match[1];
        const id = match[2] ?? "";
        const text = match[0];
        return {
          type: "mention",
          raw: text,
          text,
          tokens: [],
          attrs: { type, id },
        };
      },
    },
    parseMarkdown(token) {
      const attrs = (token as { attrs?: { type: string; id: string } })
        .attrs ?? { type: "everyone", id: "" };
      const type = attrs.type;
      const id = attrs.id;
      const label = resolveLabel ? resolveLabel(type, id) : (id || "everyone");
      return {
        type: "mention",
        attrs: { type, id, label },
      };
    },
    renderMarkdown(node) {
      const type = node.attrs?.type ?? "user";
      const id = node.attrs?.id ?? "";
      return type === "everyone" ? "<@everyone>" : `<@${type}:${id}>`;
    },
  });
}

/**
 * <plume-task-editor>: a rich-text editor for task
 * descriptions, built on TipTap (ProseMirror).
 *
 * - Stores content as **markdown** (via @tiptap/markdown), identical to the
 *   chat message format. Read mode is rendered by TipTap itself
 *   (editable=false): no regex renderer, the editor handles all formatting.
 * - Mentions are a custom TipTap node that round-trips `<@type:id>` tokens
 *   through markdown unchanged, rendered as chips in the editor.
 * - Bubble menu for inline formatting (bold/italic/strike/code) on selection.
 * - Toolbar with heading/list/quote/undo/redo.
 * - Auto-saves via debounced `plume-change` events (markdown string).
 *
 * Public API: getValue(), setValue(), clear(), focus().
 */
@localized()
@customElement("plume-task-editor")
export class PlumeTaskEditor extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
    }

    .editor-shell {
      border: 1px solid var(--input);
      border-radius: var(--radius-md);
      background: var(--background);
      transition: border-color var(--dur-fast) var(--ease-1);
      overflow: hidden;
    }
    .editor-shell:focus-within {
      border-color: var(--ring);
      box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 30%, transparent);
    }
    /* Read-only mode: no border/shadow, lighter padding. */
    .editor-shell[data-read-only="true"] {
      border: none;
      background: transparent;
    }
    .editor-shell[data-read-only="true"]:focus-within {
      box-shadow: none;
    }
    .editor-shell[data-read-only="true"] .ProseMirror {
      min-height: auto;
      max-height: none;
      padding: 0;
    }

    /* Toolbar */
    .toolbar {
      display: flex;
      align-items: center;
      gap: var(--space-0-5);
      padding: var(--space-1) var(--space-2);
      border-bottom: 1px solid var(--border);
      background: var(--muted);
    }
    .toolbar-group {
      display: flex;
      align-items: center;
      gap: var(--space-0-5);
    }
    .toolbar-divider {
      width: 1px;
      height: var(--space-4);
      background: var(--border);
      margin: 0 var(--space-0-5);
    }
    .tb-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--control-h-xs);
      height: var(--control-h-xs);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: var(--muted-foreground);
      cursor: pointer;
      transition: background var(--dur-fast) var(--ease-1),
        color var(--dur-fast) var(--ease-1);
    }
    .tb-btn:hover {
      background: var(--accent);
      color: var(--accent-foreground);
    }
    .tb-btn.is-active {
      background: color-mix(in oklch, var(--primary) 15%, transparent);
      color: var(--primary);
    }
    .tb-btn:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }

    /* ProseMirror content */
    .ProseMirror {
      min-height: 8rem;
      max-height: 32rem;
      overflow-y: auto;
      padding: var(--space-3) var(--space-4);
      font-size: var(--text-sm);
      font-family: inherit;
      line-height: 1.6;
      color: var(--foreground);
      outline: none;
    }
    .ProseMirror p {
      margin: 0 0 var(--space-2);
    }
    .ProseMirror p:last-child {
      margin-bottom: 0;
    }
    .ProseMirror h1 {
      font-size: var(--text-xl);
      font-weight: 600;
      margin: var(--space-4) 0 var(--space-2);
      line-height: var(--leading-tight);
    }
    .ProseMirror h2 {
      font-size: var(--text-lg);
      font-weight: 600;
      margin: var(--space-4) 0 var(--space-2);
      line-height: var(--leading-tight);
    }
    .ProseMirror h3 {
      font-size: var(--text-base);
      font-weight: 600;
      margin: var(--space-3) 0 var(--space-2);
      line-height: var(--leading-tight);
    }
    .ProseMirror ul,
    .ProseMirror ol {
      padding-left: var(--space-5);
      margin: 0 0 var(--space-2);
    }
    .ProseMirror li {
      margin: var(--space-0-5) 0;
    }
    .ProseMirror li > p {
      margin: 0;
    }
    .ProseMirror blockquote {
      margin: 0 0 var(--space-2);
      padding-left: var(--space-3);
      border-left: 3px solid var(--border);
      color: var(--muted-foreground);
    }
    .ProseMirror blockquote p {
      margin: 0;
    }
    .ProseMirror pre {
      margin: 0 0 var(--space-2);
      padding: var(--space-2) var(--space-3);
      border-radius: var(--radius-md);
      background: var(--muted);
      color: var(--foreground);
      font-family: var(--font-mono);
      font-size: var(--text-xs);
      overflow-x: auto;
    }
    .ProseMirror pre code {
      background: none;
      padding: 0;
      font-size: inherit;
    }
    .ProseMirror code {
      padding: 0 var(--space-1);
      border-radius: var(--radius-sm);
      background: var(--muted);
      font-family: var(--font-mono);
      font-size: 0.85em;
    }
    .ProseMirror hr {
      border: none;
      border-top: 1px solid var(--border);
      margin: var(--space-3) 0;
    }
    .ProseMirror a {
      color: var(--primary);
      text-decoration: underline;
      text-underline-offset: 2px;
    }

    /* Mention chips */
    .ProseMirror .mention-chip {
      display: inline-block;
      border-radius: var(--radius-sm);
      padding: 0 var(--space-1);
      font-size: var(--text-sm);
      font-weight: 500;
      cursor: default;
      user-select: none;
    }
    .ProseMirror .mention-user,
    .ProseMirror .mention-everyone {
      background: color-mix(in oklch, var(--primary) 15%, transparent);
      color: var(--primary);
    }
    .ProseMirror .mention-channel {
      background: color-mix(in oklch, #6366f1 15%, transparent);
      color: light-dark(#4f46e5, #a5b4fc);
    }
    .ProseMirror .mention-project {
      background: color-mix(in oklch, #f59e0b 15%, transparent);
      color: light-dark(#b45309, #fcd34d);
    }
    .ProseMirror .mention-task {
      background: color-mix(in oklch, #10b981 15%, transparent);
      color: light-dark(#059669, #6ee7b7);
    }

    /* Placeholder */
    .ProseMirror p.is-editor-empty:first-child::before {
      content: attr(data-placeholder);
      color: var(--muted-foreground);
      pointer-events: none;
      float: left;
      height: 0;
    }

    /* Bubble menu */
    .bubble-menu {
      display: flex;
      gap: 1px;
      padding: var(--space-1);
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
      background: var(--popover);
      box-shadow: var(--shadow-lg);
    }
    .bubble-menu button {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--control-h-xs);
      height: var(--control-h-xs);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: var(--muted-foreground);
      cursor: pointer;
      transition: background var(--dur-fast) var(--ease-1),
        color var(--dur-fast) var(--ease-1);
    }
    .bubble-menu button:hover {
      background: var(--accent);
      color: var(--accent-foreground);
    }
    .bubble-menu button.is-active {
      background: var(--primary);
      color: var(--primary-foreground);
    }
  `;

  /** Initial markdown content. */
  @property()
  value = "";

  @property({ type: Boolean })
  editable = true;

  @property()
  placeholder = "Add description…";

  /** When true, focus the editor once initialized. */
  @property({ type: Boolean })
  autofocus = false;

  /** Optional resolver for mention labels in read mode. */
  @property({ attribute: false })
  mentionResolver?: MentionLabelResolver;

  @query(".editor-mount")
  private _mount!: HTMLDivElement;

  #editor: Editor | null = null;
  #changeDebounce: ReturnType<typeof setTimeout> | null = null;
  // Tracks the last markdown emitted via plume-change so updated() can tell
  // an external value change (task reload) from the editor's own output and
  // avoid re-applying content the user just typed (which resets the cursor).
  #lastEmitted: string | null = null;

  // Mention popover state
  @state()
  private _mentionOpen = false;
  @state()
  private _mentionQuery = "";
  @state()
  private _mentionLeft = 8;
  // Document position where the @ query starts, captured at detection time.
  #mentionFrom: number | null = null;

  // Active toolbar states (driven by editor selection updates)
  @state()
  private _active: Record<string, boolean> = {};
  @state()
  private _canUndo = false;
  @state()
  private _canRedo = false;

  // Lifecycle

  protected firstUpdated(): void {
    this.#initEditor();
  }

  protected updated(changed: Map<string, unknown>): void {
    // Re-sync editor content when the value prop changes due to an EXTERNAL
    // update (e.g. task reloaded). We must NOT re-apply value that
    // originated from the editor's own onUpdate: that would create a
    // feedback loop: user types → change event → parent sets .value →
    // setContent() resets cursor → user fights the editor.
    //
    // The editor is the source of truth while editing; the parent only feeds
    // value back in on explicit reloads, detected by a mismatch that isn't
    // just the editor's own latest output.
    if (changed.has("value") && this.#editor && !this.#editor.isDestroyed) {
      if (
        this.value !== this.#lastEmitted &&
        this.value !== this.#editor.getMarkdown()
      ) {
        this.#editor.commands.setContent(this.value || "", {
          contentType: "markdown",
        });
      }
    }
    if (changed.has("editable") && this.#editor && !this.#editor.isDestroyed) {
      this.#editor.setEditable(this.editable);
    }
  }

  disconnectedCallback(): void {
    this.#editor?.destroy();
    this.#editor = null;
    if (this.#changeDebounce) clearTimeout(this.#changeDebounce);
    super.disconnectedCallback();
  }

  #initEditor(): void {
    if (!this._mount) return;

    // Build the bubble menu element in the light DOM (outside shadow root)
    // so ProseMirror's floating-ui positioning works reliably.
    const bubbleEl = document.createElement("div");
    bubbleEl.className = "bubble-menu";
    const bubbleButtons = [
      { cmd: "bold", title: msg("Bold"), label: "B" },
      { cmd: "italic", title: msg("Italic"), label: "I" },
      { cmd: "strike", title: msg("Strikethrough"), label: "S" },
      { cmd: "code", title: msg("Inline code"), label: "{ }" },
    ];
    for (const { cmd, title, label } of bubbleButtons) {
      const btn = document.createElement("button");
      btn.dataset.cmd = cmd;
      btn.title = title;
      btn.textContent = label;
      bubbleEl.appendChild(btn);
    }
    bubbleEl.style.display = "none";
    this._mount.appendChild(bubbleEl);

    bubbleEl.addEventListener("mousedown", (e) => {
      const btn = (e.target as HTMLElement).closest("button");
      if (!btn) return;
      e.preventDefault();
      const cmd = btn.dataset.cmd;
      if (cmd) this.#editor?.chain().focus().toggleMark(cmd).run();
    });

    this.#editor = new Editor({
      element: this._mount,
      extensions: [
        StarterKit.configure({
          heading: { levels: [1, 2, 3] },
        }),
        Markdown,
        createMention(this.mentionResolver),
        Placeholder.configure({ placeholder: this.placeholder }),
        BubbleMenu.configure({
          element: bubbleEl,
          updateDelay: 100,
          shouldShow: ({ editor, from, to }) => {
            if (from === to) return false; // no selection
            return !editor.isActive("codeBlock");
          },
        }),
      ],
      content: this.value || "",
      contentType: "markdown",
      editable: this.editable,
      injectCSS: false,
      editorProps: {
        handleKeyDown: (_view, event) => {
          const e = event as KeyboardEvent;
          if (e.key === "Escape") {
            this.dispatchEvent(
              new CustomEvent("plume-escape", {
                bubbles: true,
                composed: true,
              }),
            );
            return true;
          }
          // Cmd/Ctrl+Enter saves the description.
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
            this.dispatchEvent(
              new CustomEvent("plume-save", {
                bubbles: true,
                composed: true,
              }),
            );
            return true;
          }
          return false;
        },
      },
      onUpdate: () => {
        this.#syncToolbar();
        this.#detectMention();
        this.#emitChange();
      },
      onSelectionUpdate: () => {
        this.#syncToolbar();
      },
      onTransaction: () => {
        this.#syncToolbar();
      },
      onBlur: () => {
        this._mentionOpen = false;
      },
    });

    this.#syncToolbar();

    if (this.autofocus) {
      requestAnimationFrame(() => this.#editor?.commands.focus("end"));
    }
  }

  #syncToolbar(): void {
    const e = this.#editor;
    if (!e) return;
    this._active = {
      bold: e.isActive("bold"),
      italic: e.isActive("italic"),
      strike: e.isActive("strike"),
      code: e.isActive("code"),
      heading: e.isActive("heading"),
      bulletList: e.isActive("bulletList"),
      orderedList: e.isActive("orderedList"),
      blockquote: e.isActive("blockquote"),
    };
    this._canUndo = e.can().undo();
    this._canRedo = e.can().redo();
  }

  #emitChange(): void {
    if (this.#changeDebounce) clearTimeout(this.#changeDebounce);
    this.#changeDebounce = setTimeout(() => {
      const md = this.#editor?.getMarkdown() ?? "";
      this.#lastEmitted = md;
      this.dispatchEvent(
        new CustomEvent("plume-change", {
          detail: { value: md },
          bubbles: true,
          composed: true,
        }),
      );
    }, 300);
  }

  // Mention detection + insertion

  #detectMention(): void {
    const e = this.#editor;
    if (!e) return;
    const text = e.getText();
    const match = TRAILING_AT_RE.exec(text);
    if (match) {
      this._mentionQuery = match[1] ?? "";
      this._mentionLeft = this.#getCaretLeft();
      this.#mentionFrom = e.state.selection.from - match[0].length;
      this._mentionOpen = true;
    } else {
      this._mentionOpen = false;
      this.#mentionFrom = null;
    }
  }

  #getCaretLeft(): number {
    const sel = window.getSelection();
    if (!sel || sel.rangeCount === 0) return 8;
    const range = sel.getRangeAt(0).cloneRange();
    const rect = range.getBoundingClientRect();
    const editorRect = this._mount.getBoundingClientRect();
    if (rect.left === 0 && rect.top === 0) return 8;
    return Math.max(8, rect.left - editorRect.left);
  }

  private _onMentionPick(e: CustomEvent): void {
    const result = e.detail as MentionResult;
    const type = result.type || "user";
    const id = result.id || "";
    const label = result.label || id;

    const ed = this.#editor;
    if (!ed) return;

    // Delete the @query text, then insert the mention node + a trailing space.
    if (this.#mentionFrom != null) {
      const from = this.#mentionFrom;
      const to = ed.state.selection.from;
      ed.chain()
        .focus()
        .deleteRange({ from, to })
        .insertContentAt(from, [
          { type: "mention", attrs: { type, id, label } },
          { type: "text", text: " " },
        ])
        .run();
    } else {
      ed.chain()
        .focus()
        .insertContent([
          { type: "mention", attrs: { type, id, label } },
          { type: "text", text: " " },
        ])
        .run();
    }

    this._mentionOpen = false;
    this.#mentionFrom = null;
    this.#emitChange();
  }

  private _onMentionClose(): void {
    this._mentionOpen = false;
  }

  // Toolbar commands

  private _run(cmd: string): void {
    const e = this.#editor;
    if (!e) return;
    const chain = e.chain().focus();
    switch (cmd) {
      case "bold":
        chain.toggleBold().run();
        break;
      case "italic":
        chain.toggleItalic().run();
        break;
      case "strike":
        chain.toggleStrike().run();
        break;
      case "code":
        chain.toggleCode().run();
        break;
      case "h1":
        chain.toggleHeading({ level: 1 }).run();
        break;
      case "h2":
        chain.toggleHeading({ level: 2 }).run();
        break;
      case "h3":
        chain.toggleHeading({ level: 3 }).run();
        break;
      case "bulletList":
        chain.toggleBulletList().run();
        break;
      case "orderedList":
        chain.toggleOrderedList().run();
        break;
      case "blockquote":
        chain.toggleBlockquote().run();
        break;
      case "undo":
        chain.undo().run();
        break;
      case "redo":
        chain.redo().run();
        break;
    }
    this.#syncToolbar();
  }

  // Public API

  getValue(): string {
    return this.#editor?.getMarkdown() ?? "";
  }

  setValue(md: string): void {
    this.#editor?.commands.setContent(md || "", {
      contentType: "markdown",
    });
  }

  clear(): void {
    this.#editor?.commands.clearContent();
  }

  focus(): void {
    this.#editor?.commands.focus("end");
  }

  // Render

  protected render() {
    return html`
      <div class="wrap">
        <div class="editor-shell" data-read-only=${this.editable
          ? "false"
          : "true"}>
          ${this.editable
            ? html`
              <div class="toolbar">
                <div class="toolbar-group">
                  <button
                    class="tb-btn ${this._active.heading ? "is-active" : ""}"
                    title="${msg("Heading 1")}"
                    @click=${() => this._run("h1")}
                  >
                    <b>H1</b>
                  </button>
                  <button
                    class="tb-btn"
                    title="${msg("Heading 2")}"
                    @click=${() => this._run("h2")}
                  >
                    <b>H2</b>
                  </button>
                  <button
                    class="tb-btn"
                    title="${msg("Heading 3")}"
                    @click=${() => this._run("h3")}
                  >
                    <b>H3</b>
                  </button>
                </div>
                <div class="toolbar-divider"></div>
                <div class="toolbar-group">
                  <button
                    class="tb-btn ${this._active.bold ? "is-active" : ""}"
                    title="${msg("Bold")}"
                    @click=${() => this._run("bold")}
                  >
                    <plume-icon name="bold" size="15"></plume-icon>
                  </button>
                  <button
                    class="tb-btn ${this._active.italic ? "is-active" : ""}"
                    title="${msg("Italic")}"
                    @click=${() => this._run("italic")}
                  >
                    <plume-icon name="italic" size="15"></plume-icon>
                  </button>
                  <button
                    class="tb-btn ${this._active.strike ? "is-active" : ""}"
                    title="${msg("Strikethrough")}"
                    @click=${() => this._run("strike")}
                  >
                    <plume-icon name="strikethrough" size="15"></plume-icon>
                  </button>
                  <button
                    class="tb-btn ${this._active.code ? "is-active" : ""}"
                    title="${msg("Inline code")}"
                    @click=${() => this._run("code")}
                  >
                    <plume-icon name="code" size="15"></plume-icon>
                  </button>
                </div>
                <div class="toolbar-divider"></div>
                <div class="toolbar-group">
                  <button
                    class="tb-btn ${this._active.bulletList ? "is-active" : ""}"
                    title="${msg("Bullet list")}"
                    @click=${() => this._run("bulletList")}
                  >
                    <plume-icon name="list" size="15"></plume-icon>
                  </button>
                  <button
                    class="tb-btn ${this._active.orderedList
                      ? "is-active"
                      : ""}"
                    title="${msg("Numbered list")}"
                    @click=${() => this._run("orderedList")}
                  >
                    <plume-icon name="list-ordered" size="15"></plume-icon>
                  </button>
                  <button
                    class="tb-btn ${this._active.blockquote ? "is-active" : ""}"
                    title="${msg("Quote")}"
                    @click=${() => this._run("blockquote")}
                  >
                    <plume-icon name="quote" size="15"></plume-icon>
                  </button>
                </div>
                <div class="toolbar-divider"></div>
                <div class="toolbar-group">
                  <button
                    class="tb-btn"
                    title="${msg("Undo")}"
                    ?disabled=${!this._canUndo}
                    @click=${() => this._run("undo")}
                  >
                    <plume-icon name="undo" size="15"></plume-icon>
                  </button>
                  <button
                    class="tb-btn"
                    title="${msg("Redo")}"
                    ?disabled=${!this._canRedo}
                    @click=${() => this._run("redo")}
                  >
                    <plume-icon name="redo" size="15"></plume-icon>
                  </button>
                </div>
              </div>
            `
            : nothing}
          <div class="editor-mount"></div>
        </div>
        ${this._mentionOpen
          ? html`
            <plume-mention-popover
              .query=${this._mentionQuery}
              .left=${this._mentionLeft}
              @pick=${this._onMentionPick}
              @close=${this._onMentionClose}
            ></plume-mention-popover>
          `
          : nothing}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-task-editor": PlumeTaskEditor;
  }
}
