import { localized, msg } from "@lit/localize";
import { css, html, LitElement, nothing } from "lit";
import { customElement, property, query, state } from "lit/decorators.js";
import { unsafeHTML } from "lit/directives/unsafe-html.js";
import type { DtoCommentResponse, DtoMentionsResponse } from "@/api";
import {
  deleteProjectsByIdTasksByTaskIdCommentsByCommentId,
  getProjectsByIdTasksByTaskIdComments,
  patchProjectsByIdTasksByTaskIdCommentsByCommentId,
  postProjectsByIdTasksByTaskIdComments,
} from "@/api";
import { auth } from "@/store/auth";
import { wsClient } from "@/store/ws";
import { renderMarkdownWithMentions } from "@/lib/markdown";
import { buildResolver } from "@/features/chat/mention-utils";
import { SignalController } from "@/lib/signal-controller";
import { timeAgo } from "@/lib/format/time-ago";
import "../../components/ui/avatar.ts";
import "../../components/ui/plume-icon.ts";
import "../../components/ui/button.ts";
import { PlumeCommentComposer } from "./comment-composer";
import "./comment-composer.ts";

/** Absolute time for the hover tooltip. */
function formatAbsolute(date: Date): string {
  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

function commentInitials(name?: string): string {
  if (!name) return "??";
  return name.trim().split(/\s+/).map((w) => w[0]).join("").toUpperCase()
    .slice(0, 2);
}

interface CommentNode extends DtoCommentResponse {
  id: string;
  children: CommentNode[];
}

/** Build a threaded tree from a flat comment list (parent_id → children). */
function buildTree(comments: DtoCommentResponse[]): CommentNode[] {
  const byID = new Map<string, CommentNode>();
  for (const c of comments) {
    byID.set(c.id!, { ...c, id: c.id!, children: [] });
  }
  const roots: CommentNode[] = [];
  for (const node of byID.values()) {
    if (node.parent_id && byID.has(node.parent_id)) {
      byID.get(node.parent_id)!.children.push(node);
    } else {
      roots.push(node);
    }
  }
  return roots;
}

/**
 * Comment thread for a task.
 *
 * - Rich markdown rendering with @mention chips (reuses chat's renderer)
 * - Threaded replies (parent_id) with an inline reply composer
 * - Edit / delete your own comments, with an "edited" indicator
 * - Live updates over WebSocket (comment_new / comment_updated / comment_deleted)
 * - Avatars, author + relative time, hover for absolute timestamp
 */
@localized()
@customElement("plume-comment-thread")
export class PlumeCommentThread extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
    }

    .thread {
      display: flex;
      flex-direction: column;
      gap: var(--space-4);
    }

    /* Empty state */
    .empty {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: var(--space-2);
      padding: var(--space-8) var(--space-4);
      text-align: center;
      color: var(--muted-foreground);
    }
    .empty-icon {
      display: inline-flex;
      width: var(--space-10);
      height: var(--space-10);
      align-items: center;
      justify-content: center;
      border-radius: var(--radius-full);
      background: var(--muted);
      color: var(--muted-foreground);
    }
    .empty-title {
      font-size: var(--text-sm);
      font-weight: 600;
      color: var(--foreground);
    }
    .empty-sub {
      font-size: var(--text-xs);
    }

    /* Composer at the bottom */
    .composer {
      margin-top: var(--space-2);
    }
    .load-more {
      display: flex;
      justify-content: center;
      padding: var(--space-1) 0;
    }

    /* Single comment */
    .comment {
      display: flex;
      gap: var(--space-2);
    }
    .comment-body {
      flex: 1;
      min-width: 0;
      display: flex;
      flex-direction: column;
      gap: var(--space-1);
    }
    .comment-head {
      display: flex;
      align-items: baseline;
      gap: var(--space-2);
      flex-wrap: wrap;
    }
    .comment-author {
      font-size: var(--text-sm);
      font-weight: 600;
      color: var(--foreground);
    }
    .comment-time {
      font-size: var(--text-2xs);
      color: var(--muted-foreground);
      cursor: default;
    }
    .comment-edited {
      font-size: var(--text-2xs);
      color: var(--muted-foreground);
    }

    /* Markdown content */
    .content {
      font-size: var(--text-sm);
      line-height: 1.55;
      color: var(--foreground);
      word-wrap: break-word;
      overflow-wrap: anywhere;
    }
    .content :where(p) {
      margin: 0 0 var(--space-1);
    }
    .content :where(p:last-child) {
      margin-bottom: 0;
    }
    .content code {
      background: var(--muted);
      padding: 1px 5px;
      border-radius: var(--radius-sm);
      font-size: var(--text-xs);
      font-family: var(--font-mono);
    }
    .content pre {
      background: var(--muted);
      padding: var(--space-2) var(--space-3);
      border-radius: var(--radius-md);
      overflow-x: auto;
      margin: var(--space-1) 0;
    }
    .content pre code {
      background: transparent;
      padding: 0;
    }
    .content a {
      color: var(--primary);
      text-decoration: underline;
    }
    .content .mention-link {
      text-decoration: none;
    }
    .content .mention-chip {
      display: inline-block;
      border-radius: var(--radius-sm);
      padding: 0 var(--space-1);
      font-size: var(--text-sm);
      font-weight: 500;
    }
    .content .mention-user,
    .content .mention-everyone {
      background: color-mix(in oklch, var(--primary) 15%, transparent);
      color: var(--primary);
    }
    .content .mention-channel {
      background: color-mix(in oklch, #6366f1 15%, transparent);
      color: light-dark(#4f46e5, #a5b4fc);
    }
    .content .mention-project {
      background: color-mix(in oklch, #f59e0b 15%, transparent);
      color: light-dark(#b45309, #fcd34d);
    }
    .content .mention-task {
      background: color-mix(in oklch, #10b981 15%, transparent);
      color: light-dark(#059669, #6ee7b7);
    }

    /* Hover actions (reply / edit / delete) */
    .actions {
      display: flex;
      gap: var(--space-1);
      opacity: 0;
      transition: opacity var(--dur-fast) var(--ease-1);
    }
    /* Only show actions for the directly-hovered comment, not the entire
      thread chain. Direct-child combinators ensure nested replies' actions
      (which live in separate .comment elements inside .replies) don't match. */
    .comment:hover > .comment-body > .comment-head > .actions {
      opacity: 1;
    }
    .action-btn {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
      padding: 2px var(--space-1);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: var(--muted-foreground);
      font-size: var(--text-2xs);
      font-family: inherit;
      cursor: pointer;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .action-btn:hover {
      background: var(--accent);
      color: var(--foreground);
    }
    .action-btn[data-danger]:hover {
      color: var(--destructive);
    }

    /* Deleted comment placeholder */
    .deleted {
      font-size: var(--text-xs);
      font-style: italic;
      color: var(--muted-foreground);
      padding: var(--space-1) 0;
    }

    /* Threaded replies */
    .replies {
      margin-top: var(--space-2);
      padding-left: calc(var(--avatar-sm, 1.5rem) + var(--space-2));
      display: flex;
      flex-direction: column;
      gap: var(--space-3);
      border-left: 2px solid var(--border);
      margin-left: calc(var(--avatar-sm, 1.5rem) / 2 - 1px);
    }
    .reply-composer {
      margin-top: var(--space-1);
    }

    /* Inline edit composer */
    .edit-wrap {
      display: flex;
      flex-direction: column;
      gap: var(--space-2);
    }
  `;

  @property()
  projectId = "";

  @property()
  taskId = "";

  @state()
  private _comments: DtoCommentResponse[] = [];

  @state()
  private _loading = true;

  @state()
  private _sending = false;

  @state()
  private _hasMore = false;

  private _nextCursor = "";

  @state()
  private _replyTo: string | null = null;

  @state()
  private _editingId: string | null = null;

  @query(".composer > plume-comment-composer")
  private _mainComposer!: PlumeCommentComposer;
  @query("plume-comment-composer[editing]")
  private _editingComposer!: PlumeCommentComposer | null;
  @query(".reply-composer > plume-comment-composer")
  private _replyComposer!: PlumeCommentComposer | null;

  #signals = new SignalController(this);
  #wsMessageHandler: ((e: MessageEvent) => void) | null = null;
  #prevWs: WebSocket | null = null;

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(wsClient);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this.#detachWs();
  }

  protected willUpdate(changed: Map<string, unknown>): void {
    if (
      (changed.has("taskId") || changed.has("projectId")) &&
      this.projectId &&
      this.taskId
    ) {
      this._loadComments();
    }
    if (changed.has("taskId") && wsClient.value) {
      this.#attachWs(wsClient.value);
    }
  }

  protected updated(changed: Map<string, unknown>): void {
    // (Re)attach WS listener when the socket (re)connects.
    const ws = wsClient.value;
    if (ws && ws !== this.#prevWs) {
      this.#attachWs(ws);
    } else if (!ws && this.#prevWs) {
      this.#detachWs();
    }
    // Seed the inline edit composer with the comment's existing content
    // whenever we enter edit mode (the composer is freshly rendered).
    if (changed.has("_editingId") && this._editingId) {
      const node = this._comments.find((c) => c.id === this._editingId);
      if (node) {
        const composer = this._editingComposer;
        if (composer) {
          composer.setContent(node.content ?? "");
          composer.focus();
        }
      }
    }
    // Focus the reply composer as soon as it appears. Deferred to the next
    // frame so the nested plume-chat-editor (and its contenteditable) has
    // rendered its shadow DOM: otherwise focus() is a no-op.
    if (changed.has("_replyTo") && this._replyTo) {
      requestAnimationFrame(() => {
        this._replyComposer?.focus();
      });
    }
  }

  // WebSocket: live comment updates (project room broadcasts)

  #attachWs(ws: WebSocket): void {
    this.#detachWs();
    this.#prevWs = ws;
    this.#wsMessageHandler = (e) => this.#onWsMessage(e);
    ws.addEventListener("message", this.#wsMessageHandler);
  }

  #detachWs(): void {
    if (this.#prevWs && this.#wsMessageHandler) {
      this.#prevWs.removeEventListener("message", this.#wsMessageHandler);
    }
    this.#prevWs = null;
    this.#wsMessageHandler = null;
  }

  #onWsMessage(e: MessageEvent): void {
    let data: { type?: string; payload?: Record<string, unknown> };
    try {
      data = JSON.parse(e.data);
    } catch {
      return;
    }
    const payload = data.payload ?? {};
    switch (data.type) {
      case "comment_new": {
        const comment = payload.comment as DtoCommentResponse | undefined;
        if (comment && comment.task_id === this.taskId) {
          this.#upsertComment(comment);
        }
        break;
      }
      case "comment_updated": {
        const comment = payload.comment as DtoCommentResponse | undefined;
        if (comment && comment.task_id === this.taskId) {
          this.#upsertComment(comment);
        }
        break;
      }
      case "comment_deleted": {
        const id = payload.comment_id as string | undefined;
        const tid = payload.task_id as string | undefined;
        if (id && tid === this.taskId) {
          this._comments = this._comments.filter((c) => c.id !== id);
        }
        break;
      }
    }
  }

  #upsertComment(comment: DtoCommentResponse): void {
    const idx = this._comments.findIndex((c) => c.id === comment.id);
    if (idx >= 0) {
      const next = [...this._comments];
      next[idx] = comment;
      this._comments = next;
    } else {
      // Insert in chronological order (oldest first, like the API).
      const created = new Date(comment.created_at ?? 0).getTime();
      const next = [...this._comments];
      let i = next.length;
      for (let j = 0; j < next.length; j++) {
        if (new Date(next[j].created_at ?? 0).getTime() > created) {
          i = j;
          break;
        }
      }
      next.splice(i, 0, comment);
      this._comments = next;
    }
    // Exit edit mode if the updated comment was being edited.
    if (this._editingId === comment.id) {
      this._editingId = null;
    }
  }

  // Data loading

  /**
   * Scroll the nearest scrollable ancestor all the way to its bottom
   * (scrollHeight). Walks up from the host element, crossing shadow DOM
   * boundaries, so it finds the scroll container regardless of which
   * custom element's shadow tree it lives in.
   */
  #scrollAncestorToBottom(): void {
    let node: Element | null = this;
    while (node) {
      const style = getComputedStyle(node);
      if (
        (style.overflowY === "auto" || style.overflowY === "scroll") &&
        node.scrollHeight > node.clientHeight
      ) {
        node.scrollTop = node.scrollHeight;
        return;
      }
      if (node.parentElement) {
        node = node.parentElement;
      } else {
        const root = node.getRootNode();
        node = root instanceof ShadowRoot ? root.host : null;
      }
    }
  }

  private async _loadComments(silent = false): Promise<void> {
    if (!this.projectId || !this.taskId) return;
    if (!silent) this._loading = true;
    try {
      const { data } = await getProjectsByIdTasksByTaskIdComments({
        path: { id: this.projectId, taskId: this.taskId },
        throwOnError: true,
      });
      const result = data ?? {
        items: [],
      };
      this._comments = result.items ?? [];
      this._hasMore = result.has_more ?? false;
      this._nextCursor = result.next_cursor ?? "";
    } catch {
      if (!silent) this._comments = [];
    } finally {
      if (!silent) this._loading = false;
    }
  }

  private async _loadMore(): Promise<void> {
    if (!this.projectId || !this.taskId || !this._nextCursor || this._loading) {
      return;
    }
    this._loading = true;
    try {
      const { data } = await getProjectsByIdTasksByTaskIdComments({
        path: { id: this.projectId, taskId: this.taskId },
        query: { before: this._nextCursor, limit: 50 },
        throwOnError: true,
      });
      const result = data ?? {
        items: [],
      };
      // Prepend older comments to the front of the list.
      this._comments = [...(result.items ?? []), ...this._comments];
      this._hasMore = result.has_more ?? false;
      this._nextCursor = result.next_cursor ?? "";
    } catch {
      // keep existing comments on error
    } finally {
      this._loading = false;
    }
  }

  // Actions

  private async _onSubmit(e: CustomEvent): Promise<void> {
    const content = (e.detail.content as string)?.trim();
    if (!content || !this.projectId || !this.taskId || this._sending) return;
    this._sending = true;
    const parentId = this._replyTo;
    const composer = e.target as PlumeCommentComposer | null;
    try {
      await postProjectsByIdTasksByTaskIdComments({
        path: { id: this.projectId, taskId: this.taskId },
        body: { content, parent_id: parentId ?? undefined },
        throwOnError: true,
      });
      if (parentId) {
        // Reply: close the inline reply composer (standard behavior;
        // the box goes away, no refocus needed).
        this._replyTo = null;
      } else {
        // Main comment: clear the draft so the user can type the next one.
        composer?.clear();
      }
      // Silent refetch: don't flip _loading (which would destroy the DOM,
      // reset scroll, and lose focus). The WS broadcast handles live updates;
      // this covers the no-WS case.
      await this._loadComments(true);
    } catch {
      // keep draft on failure
    } finally {
      this._sending = false;
      // For main comments, restore focus after the editor is re-enabled
      // (sending=false → contenteditable=true) and scroll the composer into
      // view so it stays fully visible after the new comment pushes content
      // down. Replies don't need this: the reply box is gone by design.
      if (!parentId) {
        requestAnimationFrame(() => {
          this._mainComposer?.focus();
          // Scroll the nearest scrollable ancestor all the way to its bottom
          // so the composer sits flush at the bottom with full breathing room.
          // scrollIntoView leaves a gap when the scroll container has padding.
          this.#scrollAncestorToBottom();
        });
      }
    }
  }

  private async _onEditSubmit(e: CustomEvent): Promise<void> {
    const content = (e.detail.content as string)?.trim();
    if (!content || !this._editingId || !this.projectId || !this.taskId) {
      return;
    }
    const id = this._editingId;
    this._sending = true;
    try {
      await patchProjectsByIdTasksByTaskIdCommentsByCommentId({
        path: {
          id: this.projectId,
          taskId: this.taskId,
          commentId: id,
        },
        body: { content },
        throwOnError: true,
      });
      this._editingId = null;
      await this._loadComments(true);
    } catch {
      // keep editing on failure
    } finally {
      this._sending = false;
    }
  }

  private async _onDelete(id: string): Promise<void> {
    if (!this.projectId || !this.taskId) return;
    if (!confirm(msg("Delete this comment?"))) return;
    try {
      await deleteProjectsByIdTasksByTaskIdCommentsByCommentId({
        path: { id: this.projectId, taskId: this.taskId, commentId: id },
        throwOnError: true,
      });
      await this._loadComments(true);
    } catch {
      // ignore
    }
  }

  // Render

  protected render() {
    if (this._loading) {
      return html`<div class="empty"><span class="empty-sub">${
        msg("Loading…")
      }</span></div>`;
    }

    const tree = buildTree(this._comments);

    return html`
      <div class="thread">
        ${this._hasMore
          ? html`
            <div class="load-more">
              <plume-button
                variant="ghost"
                size="sm"
                ?disabled="${this._loading}"
                @click="${() => this._loadMore()}"
              >${msg("Load older comments")}</plume-button>
            </div>
          `
          : nothing}
        ${tree.length === 0
          ? this._renderEmpty()
          : tree.map((node) => this._renderComment(node))}
        <div class="composer">
          <plume-comment-composer
            placeholder="${msg("Write a comment…")}"
            ?sending="${this._sending}"
            @submit="${this._onSubmit}"
          ></plume-comment-composer>
        </div>
      </div>
    `;
  }

  private _renderEmpty() {
    return html`
      <div class="empty">
        <span class="empty-icon">
          <plume-icon name="message-square" size="20"></plume-icon>
        </span>
        <span class="empty-title">${msg("No comments yet")}</span>
        <span class="empty-sub">
          ${msg(
            "Start the conversation: mention someone with @ to ping them.",
          )}
        </span>
      </div>
    `;
  }

  private _renderComment(node: CommentNode): unknown {
    const me = auth.value.user?.id;
    const isMine = node.author_id === me;
    const created = node.created_at ? new Date(node.created_at + "Z") : null;
    const edited = node.edited_at ? new Date(node.edited_at + "Z") : null;
    const mentions = node.mentions
      ? buildResolver(node.mentions as DtoMentionsResponse)
      : null;

    if (this._editingId === node.id) {
      return html`
        <div class="comment-row">
          <div class="comment">
            <plume-avatar
              size="sm"
              src="${node.author_avatar_url ?? ""}"
            >${commentInitials(node.author_name)}</plume-avatar>
            <div class="comment-body">
              <div class="edit-wrap">
                <plume-comment-composer
                  .placeholder=${msg("Edit comment…")}
                  ?editing=${true}
                  ?sending=${this._sending}
                  @submit="${this._onEditSubmit}"
                  @cancel="${() => (this._editingId = null)}"
                ></plume-comment-composer>
              </div>
            </div>
          </div>
          ${node.children.length > 0
            ? html`<div class="replies">${
              node.children.map((c) => this._renderComment(c))
            }</div>`
            : nothing}
        </div>
      `;
    }

    return html`
      <div class="comment-row">
        <div class="comment">
          <plume-avatar
            size="sm"
            src="${node.author_avatar_url ?? ""}"
          >${commentInitials(node.author_name)}</plume-avatar>
          <div class="comment-body">
            <div class="comment-head">
              <span class="comment-author">${node.author_name ??
                msg("Unknown")}</span>
              ${created
                ? html`
                  <span
                    class="comment-time"
                    title="${formatAbsolute(created)}"
                  >${timeAgo(created)}</span>
                `
                : nothing}
              ${edited
                ? html`<span class="comment-edited" title="${msg("Edited")} ${
                  formatAbsolute(edited)
                }">${msg("(edited)")}</span>`
                : nothing}
              <span class="actions">
                <button
                  class="action-btn"
                  @click="${() => (this._replyTo = this._replyTo === node.id
                    ? null
                    : node.id)}"
                >
                  <plume-icon name="reply" size="13"></plume-icon>
                  ${msg("Reply")}
                </button>
                ${isMine
                  ? html`
                    <button
                      class="action-btn"
                      @click="${() => {
                        this._editingId = node.id;
                        this._replyTo = null;
                      }}"
                    >
                      <plume-icon name="pencil" size="13"></plume-icon>
                      ${msg("Edit")}
                    </button>
                    <button
                      class="action-btn"
                      data-danger
                      @click="${() => this._onDelete(node.id)}"
                    >
                      <plume-icon name="trash-2" size="13"></plume-icon>
                      ${msg("Delete")}
                    </button>
                  `
                  : nothing}
              </span>
            </div>
            <div class="content">
              ${unsafeHTML(
                renderMarkdownWithMentions(node.content ?? "", mentions),
              )}
            </div>
            ${this._replyTo === node.id
              ? html`
                <div class="reply-composer">
                  <plume-comment-composer
                    placeholder="${msg("Reply…")}"
                    ?sending="${this._sending}"
                    @submit="${this._onSubmit}"
                    @cancel="${() => (this._replyTo = null)}"
                  ></plume-comment-composer>
                </div>
              `
              : nothing}
          </div>
        </div>
        ${node.children.length > 0
          ? html`<div class="replies">${
            node.children.map((c) => this._renderComment(c))
          }</div>`
          : nothing}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-comment-thread": PlumeCommentThread;
  }
}
