import { html, LitElement, nothing } from "lit";
import { customElement, state } from "lit/decorators.js";
import { createRef, ref } from "lit/directives/ref.js";
import { SignalController } from "@/lib/signal-controller";
import {
  activeConversation,
  conversationList,
  settingsConvId,
  showChannelSettings,
  showCreateCategory,
  showCreateChannel,
} from "../store";
import type { Conversation } from "../types";
import { getConvDisplayName } from "../utils";
import { chatApi } from "../api";
import { navigate } from "@/routes/router";
import {
  setupCategoryDraggable,
  setupCategoryDropTarget,
  setupCategoryHeaderDropTarget,
  setupChannelDropTarget,
} from "../conversation-dnd";
import "@/components/ui/dialog.ts";
import "@/components/ui/button.ts";
import "@/components/ui/plume-icon.ts";
import "./channel-item.ts";
import { localized, msg } from "@lit/localize";

const WS_STYLES = `
plume-workspace-sidebar {
  display: flex;
  flex-direction: column;
  width: var(--space-64);
  border-right: 1px solid var(--border);
  background: var(--sidebar);
  color: var(--sidebar-foreground);
  flex-shrink: 0;
  overflow: hidden;
}
.ws-header {
  display: flex;
  align-items: center;
  height: var(--space-12);
  padding: 0 var(--space-3);
  border-bottom: 1px solid var(--sidebar-border);
  gap: var(--space-1);
}
.ws-header-title {
  font-size: var(--text-sm);
  font-weight: 600;
  flex: 1;
}
.ws-header-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: var(--space-7);
  height: var(--space-7);
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--sidebar-foreground);
  cursor: pointer;
  font-size: var(--text-sm);
}
.ws-header-btn:hover { background: var(--sidebar-accent); }
.ws-search-box {
  padding: var(--space-2) var(--space-3);
}
.ws-search-input {
  width: 100%;
  height: var(--control-h-sm);
  padding: 0 var(--space-2);
  border: 1px solid var(--sidebar-border);
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--sidebar-foreground);
  font-size: var(--text-sm);
  font-family: inherit;
  outline: none;
}
.ws-search-input::placeholder { color: var(--muted-foreground); }
.ws-search-input:focus { border-color: var(--sidebar-ring); }
.ws-scroll {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-2);
}
/* categories wrapper: drop target for category reorder */
.ws-cats-wrap {
  position: relative;
}
.ws-cats-wrap[data-over] {
  background: color-mix(in oklch, var(--primary) 5%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in oklch, var(--primary) 20%, transparent);
  border-radius: var(--radius-lg);
}
/* category indicator line */
.ws-cat-indicator {
  position: absolute;
  left: 0;
  right: 0;
  height: var(--space-0-5);
  display: none;
  align-items: center;
  padding: 0 var(--space-1);
  pointer-events: none;
  z-index: var(--z-sticky);
}
.ws-cat-indicator-line {
  height: var(--space-0-5);
  flex: 1;
  background: var(--primary);
  border-radius: var(--space-0-5);
}
.ws-cat-indicator-dot {
  width: var(--space-1-5);
  height: var(--space-1-5);
  border-radius: var(--radius-full);
  background: var(--primary);
  flex-shrink: 0;
}
/* a single category block */
.ws-category {
  margin-bottom: var(--space-3);
}
.ws-cat-header {
  display: flex;
  align-items: center;
  height: var(--space-6);
  padding: 0 var(--space-1);
  cursor: pointer;
  border-radius: var(--radius-md);
  font-size: var(--text-2xs, 0.6875rem);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-foreground);
  user-select: none;
}
.ws-cat-header[data-dragging] { opacity: 0.4; }
.ws-cat-header[data-cat-over] {
  background: color-mix(in oklch, var(--primary) 12%, transparent);
  color: var(--sidebar-foreground);
}
.ws-cat-header:hover { color: var(--sidebar-foreground); }
.ws-cat-chevron {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: var(--space-4);
  height: var(--space-4);
  flex-shrink: 0;
  margin-right: var(--space-0-5);
  transition: transform var(--dur-fast) var(--ease-1);
}
.ws-cat-chevron.collapsed {
  transform: rotate(-90deg);
}
.ws-cat-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ws-cat-count {
  color: var(--muted-foreground);
}
.ws-cat-actions {
  display: flex;
  gap: var(--space-0-5);
  opacity: 0;
  margin-left: auto;
}
.ws-cat-header:hover .ws-cat-actions { opacity: 1; }
.ws-cat-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: var(--space-5);
  height: var(--space-5);
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--muted-foreground);
  cursor: pointer;
  font-size: var(--text-xs);
}
.ws-cat-action:hover {
  background: var(--sidebar-accent);
  color: var(--sidebar-accent-foreground);
}
/* channels wrapper: drop target */
.ws-channels {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 1px;
  margin-top: var(--space-0-5);
  margin-left: var(--space-1);
  border-radius: var(--radius-md);
  max-height: 2000px;
  overflow: hidden;
  transition: max-height var(--dur-slow) var(--ease-2), opacity var(--dur-fast) var(--ease-1);
}
.ws-channels.collapsed {
  max-height: 0;
  opacity: 0;
  margin-top: 0;
}
.ws-channels[data-over] {
  background: color-mix(in oklch, var(--primary) 5%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in oklch, var(--primary) 20%, transparent);
}
.ws-ch-indicator {
  position: absolute;
  left: 0;
  right: 0;
  height: var(--space-0-5);
  display: none;
  align-items: center;
  padding: 0 var(--space-1);
  pointer-events: none;
  z-index: var(--z-sticky);
}
.ws-ch-indicator-line {
  height: var(--space-0-5);
  flex: 1;
  background: var(--primary);
  border-radius: var(--space-0-5);
}
.ws-ch-indicator-dot {
  width: var(--space-1-5);
  height: var(--space-1-5);
  border-radius: var(--radius-full);
  background: var(--primary);
  flex-shrink: 0;
}
.ws-empty-hint {
  padding: var(--space-1) var(--space-2);
  font-size: var(--text-xs);
  color: var(--muted-foreground);
}
.ws-uncategorized {
  margin-bottom: var(--space-3);
}
.ws-uncat-header {
  display: flex;
  align-items: center;
  height: var(--space-6);
  padding: 0 var(--space-1);
  font-size: var(--text-2xs, 0.6875rem);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-foreground);
}
/* delete dialog */
.ws-del-body {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}
.ws-del-desc {
  font-size: var(--text-sm);
  color: var(--muted-foreground);
  margin: 0;
}
.ws-del-list {
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: var(--space-2);
}
.ws-del-list-header {
  font-size: var(--text-xs);
  font-weight: 500;
  color: var(--muted-foreground);
  margin: 0 0 var(--space-1);
}
.ws-del-item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-sm);
  padding: var(--space-0-5) 0;
}
`;

/**
 * Workspace sidebar: light DOM for @atlaskit DnD compatibility.
 *
 * DnD architecture (follows container-drop-target pattern):
 *  - Category headers: draggable only
 *  - Channel items: draggable only (via plume-channel-item)
 *  - Each `.ws-channels` div: drop target for channel drops (intra + inter category)
 *  - `.ws-cats-wrap`: drop target for category reordering
 */
@localized()
@customElement("plume-workspace-sidebar")
export class PlumeWorkspaceSidebar extends LitElement {
  createRenderRoot() {
    return this;
  }

  #signals = new SignalController(this);

  @state()
  private _search = "";

  @state()
  private _collapsed: Record<string, boolean> = {};

  @state()
  private _deleteCategory: Conversation | null = null;

  @state()
  private _deleting = false;

  #catsWrapRef = createRef<HTMLDivElement>();
  #catIndicatorRef = createRef<HTMLDivElement>();
  #catDropCleanup?: () => void;

  // Per-category channel drop targets + indicators
  #channelDropCleanups = new Map<string, () => void>();

  // Per-category draggable cleanups
  #catDragCleanups = new Map<string, () => void>();

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(conversationList, activeConversation);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this.#catDropCleanup?.();
    for (const c of this.#channelDropCleanups.values()) c();
    this.#channelDropCleanups.clear();
    for (const c of this.#catDragCleanups.values()) c();
    this.#catDragCleanups.clear();
  }

  protected updated(_changed: Map<string, unknown>) {
    requestAnimationFrame(() => this.#wireDropTargets());
  }

  #wireDropTargets() {
    // Category drop target
    this.#catDropCleanup?.();
    const catsWrap = this.#catsWrapRef.value;
    const catIndicator = this.#catIndicatorRef.value;
    if (catsWrap && catIndicator) {
      this.#catDropCleanup = setupCategoryDropTarget(catsWrap, catIndicator);
    }

    // Wire category header draggables (done via inline refs in render)

    // Wire channel drop targets per category: always clean up old and create new
    for (const c of this.#channelDropCleanups.values()) c();
    this.#channelDropCleanups.clear();

    const containers = this.querySelectorAll<HTMLElement>(".ws-channels");
    containers.forEach((container) => {
      const catId = container.getAttribute("data-cat-id");
      const key = catId ?? "__uncat__";
      const indicator = container.querySelector<HTMLElement>(
        ".ws-ch-indicator",
      );
      if (indicator) {
        this.#channelDropCleanups.set(
          key,
          setupChannelDropTarget(container, indicator, catId),
        );
      }
    });
  }

  private _toggleCollapse(id: string) {
    this._collapsed = { ...this._collapsed, [id]: !this._collapsed[id] };
  }

  private _selectConv(conv: Conversation) {
    navigate(`/chat/${conv.id}`);
  }

  private _onCreateCategory() {
    showCreateCategory.value = true;
  }

  private _onCreateChannel(categoryId: string | null) {
    showCreateChannel.value = { open: true, categoryId };
  }

  private _onDeleteCategoryClick(cat: Conversation) {
    this._deleteCategory = cat;
  }

  private _onEditCategoryClick(cat: Conversation) {
    settingsConvId.value = cat.id;
    showChannelSettings.value = true;
    const url = new URL(window.location.href);
    url.searchParams.set("settings", "1");
    window.history.replaceState(null, "", url.pathname + url.search + url.hash);
  }

  private async _onConfirmDeleteCategory() {
    const cat = this._deleteCategory;
    if (!cat) return;
    this._deleting = true;
    try {
      await chatApi.deleteConversation(cat.id);
      conversationList.value = conversationList.value.filter(
        (c) => c.id !== cat.id,
      );
      if (activeConversation.value?.id === cat.id) {
        activeConversation.value = null;
      }
      this._deleteCategory = null;
    } catch {
      // ignore
    } finally {
      this._deleting = false;
    }
  }

  /* ref-based setup helpers (called from inline render refs) */

  #setupCatDraggable(el?: HTMLElement, cat?: Conversation) {
    if (!el || !cat?.id) return;
    this.#catDragCleanups.get(cat.id)?.();
    const drag = setupCategoryDraggable(el, cat);
    const drop = setupCategoryHeaderDropTarget(el, cat);
    this.#catDragCleanups.set(cat.id, () => {
      drag();
      drop();
    });
  }

  protected render() {
    const convs = conversationList.value;
    const activeId = activeConversation.value?.id;
    const q = this._search.toLowerCase();

    const workspaceConvs = convs.filter(
      (c) =>
        c.type === "channel" || c.type === "voice" || c.type === "category",
    );

    const categories = [...workspaceConvs.filter((c) => c.type === "category")]
      .sort((a, b) =>
        a.position_key < b.position_key
          ? -1
          : a.position_key > b.position_key
          ? 1
          : 0
      );

    let channels = workspaceConvs.filter((c) =>
      c.type === "channel" || c.type === "voice"
    );
    if (q) {
      channels = channels.filter((c) =>
        getConvDisplayName(c).toLowerCase().includes(q)
      );
    }

    const byCategory = (catId: string) =>
      [...channels.filter((c) => c.parent_id === catId)].sort((a, b) =>
        a.position_key < b.position_key
          ? -1
          : a.position_key > b.position_key
          ? 1
          : 0
      );

    const unparented = [...channels.filter((c) => !c.parent_id)].sort(
      (a, b) =>
        a.position_key < b.position_key
          ? -1
          : a.position_key > b.position_key
          ? 1
          : 0,
    );

    const delCat = this._deleteCategory;
    const delCatChannels = delCat ? byCategory(delCat.id) : [];

    return html`
      <style>
      ${WS_STYLES}
      </style>

      <div class="ws-header">
        <span class="ws-header-title">Chat</span>
        <button
          class="ws-header-btn"
          @click="${this._onCreateCategory}"
          title="${msg("Create category")}"
          aria-label=${msg("Create category")}
        >
          <plume-icon name="folder-plus" size="16"></plume-icon>
        </button>
      </div>

      <div class="ws-search-box">
        <input
          class="ws-search-input"
          .value="${this._search}"
          @input="${(
            e: Event,
          ) => (this._search = (e.target as HTMLInputElement).value)}"
          placeholder=${msg("Search channels...")}
        />
      </div>

      <div class="ws-scroll">
        ${categories.length === 0 && unparented.length === 0
          ? html`
            <div class="ws-empty-hint">
              ${msg("No channels yet. Create a category to get started.")}
            </div>
          `
          : html`
            <div ${ref(this.#catsWrapRef)} class="ws-cats-wrap">
              ${categories.map(
                (cat) => {
                  const catChannels = byCategory(cat.id);
                  return html`
                    <div class="ws-category">
                      <div
                        class="ws-cat-header"
                        data-category-id="${cat.id}"
                        ${ref((el?: Element) =>
                          this.#setupCatDraggable(
                            el as HTMLElement | undefined,
                            cat,
                          )
                        )}
                        @click="${() => this._toggleCollapse(cat.id)}"
                      >
                        <span class="ws-cat-chevron${this._collapsed[cat.id]
                          ? " collapsed"
                          : ""}">
                          <plume-icon name="chevron-down" size="14"></plume-icon>
                        </span>
                        <span class="ws-cat-name">${cat
                          .name} <span class="ws-cat-count"
                          >(${catChannels.length})</span></span>
                        <div class="ws-cat-actions">
                          <button
                            class="ws-cat-action"
                            @click="${(e: Event) => {
                              e.stopPropagation();
                              this._onEditCategoryClick(cat);
                            }}"
                            title="${msg("Edit category")}"
                            aria-label=${msg("Edit category")}
                          >
                            <plume-icon name="settings" size="12"></plume-icon>
                          </button>
                          <button
                            class="ws-cat-action"
                            @click="${(e: Event) => {
                              e.stopPropagation();
                              this._onCreateChannel(cat.id);
                            }}"
                            title="${msg("Add channel")}"
                            aria-label=${msg("Add channel")}
                          >
                            <plume-icon name="plus" size="12"></plume-icon>
                          </button>
                          <button
                            class="ws-cat-action"
                            @click="${(e: Event) => {
                              e.stopPropagation();
                              this._onDeleteCategoryClick(cat);
                            }}"
                            title="${msg("Delete category")}"
                            aria-label=${msg("Delete category")}
                          >
                            <plume-icon name="trash-2" size="12"></plume-icon>
                          </button>
                        </div>
                      </div>
                      <div class="ws-channels${this._collapsed[cat.id]
                        ? " collapsed"
                        : ""}" data-cat-id="${cat.id}">
                        ${catChannels.length === 0
                          ? html`
                            <div class="ws-empty-hint">No channels</div>
                          `
                          : catChannels.map(
                            (ch) =>
                              html`
                                <plume-channel-item
                                  .conv="${ch}"
                                  ?isActive="${activeId === ch.id}"
                                  @select="${() => this._selectConv(ch)}"
                                ></plume-channel-item>
                              `,
                          )}
                        <div class="ws-ch-indicator" style="display:none">
                          <span class="ws-ch-indicator-dot"></span>
                          <span class="ws-ch-indicator-line"></span>
                          <span class="ws-ch-indicator-dot"></span>
                        </div>
                      </div>
                    </div>
                  `;
                },
              )}
              <!-- category indicator -->
              <div ${ref(
                this.#catIndicatorRef,
              )} class="ws-cat-indicator" style="display:none">
                <span class="ws-cat-indicator-dot"></span>
                <span class="ws-cat-indicator-line"></span>
                <span class="ws-cat-indicator-dot"></span>
              </div>
            </div>

            ${unparented.length > 0 || categories.length > 0
              ? html`
                <div class="ws-uncategorized">
                  <div class="ws-uncat-header">
                    <span>Uncategorized (${unparented.length})</span>
                  </div>
                  <div class="ws-channels" data-cat-id="">
                    ${unparented.length === 0
                      ? html`
                        <div class="ws-empty-hint">Drop a channel here to remove its category</div>
                      `
                      : unparented.map(
                        (ch) =>
                          html`
                            <plume-channel-item
                              .conv="${ch}"
                              ?isActive="${activeId === ch.id}"
                              @select="${() => this._selectConv(ch)}"
                            ></plume-channel-item>
                          `,
                      )}
                    <div class="ws-ch-indicator" style="display:none">
                      <span class="ws-ch-indicator-dot"></span>
                      <span class="ws-ch-indicator-line"></span>
                      <span class="ws-ch-indicator-dot"></span>
                    </div>
                  </div>
                </div>
              `
              : nothing}
          `}
      </div>

      <!-- Delete category confirmation -->
      ${delCat
        ? html`
          <plume-dialog
            style="--dialog-w:28rem"
            .open="${true}"
            heading="Delete category"
            @close="${() => {
              this._deleteCategory = null;
            }}"
          >
            <div class="ws-del-body">
              <p class="ws-del-desc">
                This will permanently delete <strong>${delCat.name}</strong>
                and all ${delCatChannels.length > 0
                  ? `${delCatChannels.length} channel${
                    delCatChannels.length > 1 ? "s" : ""
                  }`
                  : "channels"} within it. This action cannot be undone.
              </p>
              ${delCatChannels.length > 0
                ? html`
                  <div class="ws-del-list">
                    <p class="ws-del-list-header">Channels to be deleted:</p>
                    ${delCatChannels.map(
                      (ch) =>
                        html`
                          <div class="ws-del-item">
                            <plume-icon name="hash" size="14"></plume-icon>
                            <span>${ch.name}</span>
                          </div>
                        `,
                    )}
                  </div>
                `
                : nothing}
            </div>
            <div
              slot="footer"
              style="display:flex;justify-content:flex-end;gap:var(--space-2);width:100%"
            >
              <plume-button variant="ghost" type="button" @click="${() => {
                this._deleteCategory = null;
              }}">
                Cancel
              </plume-button>
              <plume-button
                variant="destructive"
                ?disabled="${this._deleting}"
                @click="${this._onConfirmDeleteCategory}"
              >
                ${this._deleting ? "Deleting..." : "Delete category"}
              </plume-button>
            </div>
          </plume-dialog>
        `
        : nothing}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-workspace-sidebar": PlumeWorkspaceSidebar;
  }
}
