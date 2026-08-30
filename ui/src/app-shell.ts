import { localized, msg } from "@lit/localize";
import { logError } from "@/lib/log";
import { html, LitElement } from "lit";
import { customElement, state } from "lit/decorators.js";
import { currentPath, matchRoute, navigate } from "@/routes/router";
import { auth, fetchMe } from "@/store/auth";
import {
  desktopNotificationsEnabled,
  loadPreferences,
} from "@/store/preferences";
import { detectLocale, setLocale } from "@/i18n";
import { initPush } from "@/store/push";
import { checkSetup, setupRequired } from "@/store/setup";
import { initTheme, loadThemeFromPreferences } from "@/store/theme";
import { loadMotionFromPreferences } from "@/store/motion";
import { fetchProjects } from "@/store/projects";
import { fetchPinnedViews } from "@/features/views/store";
import {
  fetchUnreadCount,
  handleNotificationEvent,
} from "@/features/notifications/store";
import { fetchWorkspaces } from "@/store/workspaces";
import { connectWs, disconnectWs, wsClient } from "@/store/ws";
import type { DtoNotificationResponse } from "@/api";
import { SignalController } from "@/lib/signal-controller";
import { ensureToastHost } from "./components/ui/toast-host-mount";
import { showToast } from "./components/ui/toast-store";
import { installShortcuts } from "./lib/shortcuts";
import "./components/ui/spinner.ts";
import "./components/search/command-palette.ts";
import "./components/ui/shortcuts-dialog.ts";
import "./features/auth/login-page.ts";
import "./features/setup/setup-page.ts";
import "./features/members/invite-accept-page.ts";

/**
 * Light DOM: required for @atlaskit/pragmatic-drag-and-drop.
 *
 * If this component had a shadow root, every page component (and all their
 * descendants, including the kanban board) would be inside the shadow tree.
 * Native drag events would have `event.target` retargeted to this shadow host,
 * breaking the DnD library's `draggableRegistry.get(event.target)` lookup.
 *
 * Styles are injected via a `<style>` tag (global in light DOM). Class names
 * are prefixed with `app-` to avoid collisions.
 */

const APP_STYLES = `
breeze-app {
  display: block;
  min-height: 100svh;
}
.app-loader {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100svh;
}
/* skip-to-content: hidden until focused */
.skip-link {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0,0,0,0);
  white-space: nowrap;
  border: 0;
}
.skip-link:focus {
  position: fixed;
  top: var(--space-1);
  left: var(--space-1);
  width: auto;
  height: auto;
  padding: var(--space-2) var(--space-4);
  margin: 0;
  overflow: visible;
  clip: auto;
  white-space: normal;
  background: var(--background);
  color: var(--foreground);
  border: 2px solid var(--ring);
  border-radius: var(--radius-md);
  z-index: var(--z-toast);
  font-size: var(--text-sm);
  font-weight: 500;
  text-decoration: none;
  outline: none;
}
`;

const lazyPages = {
  dashboard: () => import("./features/dashboard/dashboard-page.ts"),
  projects: () => import("./features/projects/projects-page.ts"),
  projectDetail: () => import("./features/projects/project-detail-page.ts"),
  views: () => import("./features/views/views-page.ts"),
  viewDetail: () => import("./features/views/view-detail-page.ts"),
  inbox: () => import("./features/notifications/inbox-page.ts"),
  chatPage: () => import("./features/chat/chat-page.ts"),
  inboxChat: () => import("./features/chat/inbox-page.ts"),
  members: () => import("./features/members/members-page.ts"),
  myTasks: () => import("./features/my-tasks/my-tasks-page.ts"),
  settings: () => import("./features/settings/user-settings-page.ts"),
  workspaceSettings: () =>
    import("./features/settings/workspace-settings-page.ts"),
  notFound: () => import("./features/not-found-page.ts"),
  forgotPassword: () => import("./features/auth/forgot-password-page.ts"),
  resetPassword: () => import("./features/auth/reset-password-page.ts"),
} as const;

@customElement("breeze-app")
@localized()
export class BreezeApp extends LitElement {
  /** Light DOM: required for @atlaskit/pragmatic-drag-and-drop. */
  createRenderRoot() {
    return this;
  }

  @state()
  private _ready = new Set<string>();

  private async _ensure(
    tag: string,
    loader: () => Promise<unknown>,
  ): Promise<void> {
    if (this._ready.has(tag)) return;
    try {
      await Promise.all([loader(), customElements.whenDefined(tag)]);
      this._ready = new Set(this._ready).add(tag);
    } catch (err) {
      logError(`Failed to load page chunk for ${tag}:`, err);
      showToast(msg("Failed to load page — please refresh"), {
        variant: "error",
      });
      // Retry once after a short delay to recover from transient failures.
      setTimeout(() => {
        loader()
          .then(() => customElements.whenDefined(tag))
          .then(() => {
            this._ready = new Set(this._ready).add(tag);
          })
          .catch(() => {
            // Give up: user must refresh manually.
          });
      }, 1500);
    }
  }

  #signals = new SignalController(this);
  #sidebarLoaded = false;
  #wsConnected = false;
  #themeLoaded = false;
  #wsPrev: WebSocket | null = null;
  #wsMessageHandler: ((e: MessageEvent) => void) | null = null;

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(currentPath, setupRequired, auth, wsClient);
    initTheme();
    checkSetup();
    fetchMe();
    ensureToastHost();
    installShortcuts();
    // Pick a locale for unauthenticated routes (login/setup) before prefs
    // load. For authenticated users, loadPreferences() overrides this with
    // the saved language once prefs arrive.
    void setLocale(detectLocale());
  }

  protected willUpdate(): void {
    const path = currentPath.value;
    // Load sidebar data (projects, pinned views, unread count) once the
    // user is authenticated. app-shell is persistent across navigation, so
    // this runs once per session.
    if (
      !this.#sidebarLoaded &&
      !auth.value.isLoading &&
      auth.value.isAuthenticated &&
      !setupRequired.value
    ) {
      this.#sidebarLoaded = true;
      fetchProjects();
      fetchPinnedViews();
      fetchUnreadCount();
      fetchWorkspaces();
    }
    // Load theme from server preferences once authenticated
    if (
      !this.#themeLoaded &&
      !auth.value.isLoading &&
      auth.value.isAuthenticated &&
      !setupRequired.value
    ) {
      this.#themeLoaded = true;
      loadThemeFromPreferences();
      loadMotionFromPreferences();
      loadPreferences();
    }
    // Connect WebSocket once authenticated (stays connected across navigation)
    if (
      !this.#wsConnected &&
      !auth.value.isLoading &&
      auth.value.isAuthenticated &&
      !setupRequired.value
    ) {
      this.#wsConnected = true;
      connectWs();
      // Register the service worker + push subscription (best-effort; no-op
      // if VAPID isn't configured or desktop notifications are off).
      initPush();
    } else if (
      this.#wsConnected &&
      (!auth.value.isAuthenticated || setupRequired.value)
    ) {
      this.#wsConnected = false;
      disconnectWs();
    }

    // Attach the global notification listener whenever the WebSocket
    // (re)connects, so the bell + inbox update live for mentions,
    // assignments, and comments.
    const ws = wsClient.value;
    if (ws && ws !== this.#wsPrev) {
      this.#attachWs(ws);
    } else if (!ws && this.#wsPrev) {
      this.#detachWs();
    }
    if (
      setupRequired.value || auth.value.isLoading || !auth.value.isAuthenticated
    ) return;

    // Redirect old settings routes to new workspace-settings page.
    if (path === "/settings/organization") {
      navigate("/settings/workspace");
      return;
    }
    if (path === "/settings/labels") {
      navigate("/settings/workspace/labels");
      return;
    }
    if (path === "/settings/audit-log") {
      navigate("/settings/workspace/audit");
      return;
    }

    // Preload dashboard chunk: the default post-login landing.
    // Browser-memoized, so subsequent calls are no-ops once loaded.
    lazyPages.dashboard();

    if (path === "/projects") {
      this._ensure("breeze-projects-page", lazyPages.projects);
    } else if (matchRoute("/projects/:slug", path)) {
      this._ensure("breeze-project-detail-page", lazyPages.projectDetail);
    } else if (path === "/views") {
      this._ensure("breeze-views-page", lazyPages.views);
    } else if (matchRoute("/views/:id", path)) {
      this._ensure("breeze-view-detail-page", lazyPages.viewDetail);
    } else if (path === "/inbox") {
      this._ensure("breeze-inbox-page", lazyPages.inbox);
    } else if (matchRoute("/chat/:conversationId", path) || path === "/chat") {
      this._ensure("breeze-chat-page", lazyPages.chatPage);
    } else if (
      matchRoute("/messages/:conversationId", path) || path === "/messages"
    ) {
      this._ensure("breeze-inbox-chat-page", lazyPages.inboxChat);
    } else if (path === "/members" || matchRoute("/members/:tab", path)) {
      this._ensure("breeze-members-page", lazyPages.members);
    } else if (path === "/my-tasks") {
      this._ensure("breeze-my-tasks-page", lazyPages.myTasks);
    } else if (path === "/preferences") {
      this._ensure("breeze-user-settings-page", lazyPages.settings);
    } else if (
      path === "/settings/workspace" ||
      matchRoute("/settings/workspace/:tab", path)
    ) {
      this._ensure(
        "breeze-workspace-settings-page",
        lazyPages.workspaceSettings,
      );
    } else if (path === "/") {
      this._ensure("breeze-dashboard-page", lazyPages.dashboard);
    } else {
      this._ensure("breeze-not-found-page", lazyPages.notFound);
    }
  }

  /**
   * renderPage wraps page content with the shared APP_STYLES <style> and the
   * standard loader fallback used while a lazy page chunk is loading.
   * `tag` is the custom element name that must be defined before we render the
   * real page; `content` is the page markup (incl. <breeze-command-palette>
   * when appropriate). Collapses the duplicated <style>${APP_STYLES}</style>
   * + loader scaffold that was repeated per-route.
   *
   * `lazy` controls whether the loader is shown until the chunk is ready.
   * Eagerly-imported pages (login, setup, invite-accept, and the initial
   * loader itself) pass `lazy = false` so their content renders immediately;
   * those tags are never added to `_ready` (only `_ensure` does that for lazy
   * routes), so the `_ready.has(tag)` gate would otherwise spin forever.
   */
  private renderPage(
    tag: string,
    content: ReturnType<typeof html>,
    lazy = true,
  ): ReturnType<typeof html> {
    const body = (!lazy || this._ready.has(tag)) ? content : html`
      <div class="app-loader">
        <breeze-spinner></breeze-spinner>
      </div>
    `;
    return html`
      <style>${APP_STYLES}</style><a href="#main-content"
        class="skip-link">Skip to content</a><main
        id="main-content">${body}</main>
    `;
  }

  private pageWithPalette(tag: string, page: ReturnType<typeof html>) {
    return this.renderPage(
      tag,
      html`${page}<breeze-command-palette></breeze-command-palette><breeze-shortcuts-dialog></breeze-shortcuts-dialog>`,
    );
  }

  // WebSocket: live notifications.
  // The backend broadcasts `notification_new` on the user's room for every
  // mention/assignment/comment. We listen here (app-level, persistent) so the
  // unread badge + inbox update live regardless of which page is open.
  #attachWs(ws: WebSocket): void {
    this.#detachWs();
    this.#wsPrev = ws;
    this.#wsMessageHandler = (e: MessageEvent) => {
      let data: { type?: string; payload?: DtoNotificationResponse };
      try {
        data = JSON.parse(e.data);
      } catch {
        return;
      }
      if (data.type === "notification_new" && data.payload) {
        handleNotificationEvent(data.payload as DtoNotificationResponse);
        this.#maybeShowDesktopNotification(
          data.payload as DtoNotificationResponse,
        );
      }
    };
    ws.addEventListener("message", this.#wsMessageHandler);
  }

  #detachWs(): void {
    if (this.#wsPrev && this.#wsMessageHandler) {
      this.#wsPrev.removeEventListener("message", this.#wsMessageHandler);
    }
    this.#wsPrev = null;
    this.#wsMessageHandler = null;
  }

  /**
   * Fire an OS-level desktop notification for an incoming WS event when the
   * user has `desktop_notifications` enabled AND the tab is hidden (so we
   * don't spam a notification for something the user is already looking at).
   * Permission is requested lazily on the first eligible event.
   */
  #maybeShowDesktopNotification(
    n: DtoNotificationResponse,
  ): void {
    if (!desktopNotificationsEnabled()) return;
    if (typeof Notification === "undefined") return;
    // Only notify when the app isn't focused: avoids duplicate alerts for
    // the in-app toast + badge the user already sees.
    if (!document.hidden) return;
    const fire = (): void => {
      try {
        const notif = new Notification(n.title ?? "Breeze", {
          body: n.body ?? "",
          tag: n.id ?? undefined,
        });
        notif.onclick = () => {
          window.focus();
          if (n.link) navigate(n.link);
          notif.close();
        };
      } catch {
        // Some browsers throw if the service worker registration is missing;
        // silently ignore: the in-app notification still landed.
      }
    };
    if (Notification.permission === "granted") {
      fire();
    } else if (Notification.permission === "default") {
      Notification.requestPermission().then((perm) => {
        if (perm === "granted") fire();
      });
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this.#detachWs();
  }

  protected render() {
    const path = currentPath.value;
    const setup = setupRequired.value;
    const { isLoading, isAuthenticated } = auth.value;

    if (setup === null || isLoading) {
      return this.renderPage(
        "__loader__",
        html`
          <div class="app-loader">
            <breeze-spinner></breeze-spinner>
          </div>
        `,
        false,
      );
    }

    if (setup && (path === "/" || path === "/login")) {
      return this.renderPage(
        "__setup__",
        html`<breeze-setup-page></breeze-setup-page>`,
        false,
      );
    }

    if (!isAuthenticated) {
      if (path === "/setup") {
        return this.renderPage(
          "__setup__",
          html`<breeze-setup-page></breeze-setup-page>`,
          false,
        );
      }
      if (path === "/join") {
        return this.renderPage(
          "__join__",
          html`<breeze-invite-accept-page></breeze-invite-accept-page>`,
          false,
        );
      }
      if (path === "/forgot-password") {
        return this.renderPage(
          "breeze-forgot-password-page",
          html`<breeze-forgot-password-page></breeze-forgot-password-page>`,
          false,
        );
      }
      if (path.startsWith("/reset-password")) {
        return this.renderPage(
          "breeze-reset-password-page",
          html`<breeze-reset-password-page></breeze-reset-password-page>`,
          false,
        );
      }
      return this.renderPage(
        "__login__",
        html`<breeze-login-page></breeze-login-page>`,
        false,
      );
    }

    if (path === "/setup") {
      return this.renderPage(
        "__setup__",
        html`<breeze-setup-page></breeze-setup-page>`,
        false,
      );
    }

    if (path === "/projects") {
      return this.pageWithPalette(
        "breeze-projects-page",
        html`<breeze-projects-page></breeze-projects-page>`,
      );
    }

    if (matchRoute("/projects/:slug", path)) {
      return this.pageWithPalette(
        "breeze-project-detail-page",
        html`<breeze-project-detail-page></breeze-project-detail-page>`,
      );
    }

    if (path === "/views") {
      return this.pageWithPalette(
        "breeze-views-page",
        html`<breeze-views-page></breeze-views-page>`,
      );
    }

    if (matchRoute("/views/:id", path)) {
      return this.pageWithPalette(
        "breeze-view-detail-page",
        html`<breeze-view-detail-page></breeze-view-detail-page>`,
      );
    }

    if (path === "/inbox") {
      return this.pageWithPalette(
        "breeze-inbox-page",
        html`<breeze-inbox-page></breeze-inbox-page>`,
      );
    }

    const chatMatch = matchRoute("/chat/:conversationId", path);
    if (chatMatch || path === "/chat") {
      const convId = chatMatch?.conversationId ?? "";
      return this.pageWithPalette(
        "breeze-chat-page",
        html`<breeze-chat-page conversationId="${convId}"></breeze-chat-page>`,
      );
    }

    const messagesMatch = matchRoute("/messages/:conversationId", path);
    if (messagesMatch || path === "/messages") {
      const convId = messagesMatch?.conversationId ?? "";
      return this.pageWithPalette(
        "breeze-inbox-chat-page",
        html`<breeze-inbox-chat-page conversationId="${convId}"></breeze-inbox-chat-page>`,
      );
    }

    if (path === "/members" || matchRoute("/members/:tab", path)) {
      return this.pageWithPalette(
        "breeze-members-page",
        html`<breeze-members-page></breeze-members-page>`,
      );
    }

    if (path === "/my-tasks") {
      return this.pageWithPalette(
        "breeze-my-tasks-page",
        html`<breeze-my-tasks-page></breeze-my-tasks-page>`,
      );
    }

    if (path === "/preferences") {
      return this.renderPage(
        "breeze-user-settings-page",
        html`<breeze-user-settings-page></breeze-user-settings-page>`,
      );
    }

    if (
      path === "/settings/workspace" ||
      matchRoute("/settings/workspace/:tab", path)
    ) {
      return this.pageWithPalette(
        "breeze-workspace-settings-page",
        html`<breeze-workspace-settings-page></breeze-workspace-settings-page>`,
      );
    }

    if (path === "/") {
      return this.pageWithPalette(
        "breeze-dashboard-page",
        html`<breeze-dashboard-page></breeze-dashboard-page>`,
      );
    }

    return this.pageWithPalette(
      "breeze-not-found-page",
      html`<breeze-not-found-page></breeze-not-found-page>`,
    );
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-app": BreezeApp;
  }
}
