import { css, html, LitElement, nothing } from "lit";
import { customElement, state } from "lit/decorators.js";
import { keyed } from "lit/directives/keyed.js";
import { pageEnterStyles, tabContentStyles } from "@/styles/shared-animations";
import { SignalController } from "@/lib/signal-controller";
import { currentPath, matchRoute, navigate } from "@/routes/router";
import { isOrgElevatedRole } from "@/lib/permissions";
import { auth } from "@/store/auth";
import "../../components/ui/tabs.ts";
import "../../components/ui/spinner.ts";
import "../../components/ui/plume-icon.ts";
import "../../components/ui/button.ts";
import "../../components/ui/input.ts";
import "../../components/ui/field.ts";
import "../../layouts/app-layout.ts";
import "./organization-settings-page.ts";
import "./labels-settings-page.ts";
import "./audit-log-page.ts";
import { localized, msg } from "@lit/localize";

interface TabItem {
  id: string;
  label: string;
}

function getBaseTabs(): TabItem[] {
  return [
    { id: "general", label: msg("General") },
    { id: "labels", label: msg("Labels") },
  ];
}

type TabId = "general" | "labels" | "audit";

/**
 * Workspace settings page: a tabbed hub that embeds three existing
 * settings pages as tabs:
 *   - General (org name, message edit window, backup/restore, danger zone)
 *   - Labels (CRUD)
 *   - Audit log (elevated roles only)
 *
 * Tab routing is path-based: /settings/workspace (→ general),
 * /settings/workspace/labels, /settings/workspace/audit.
 */
@localized()
@customElement("plume-workspace-settings-page")
export class PlumeWorkspaceSettingsPage extends LitElement {
  static styles = [
    pageEnterStyles,
    tabContentStyles,
    css`
      :host {
        display: contents;
      }
      .page {
        display: flex;
        flex-direction: column;
        height: 100%;
      }
      .header {
        padding: var(--space-4) var(--space-6);
        border-bottom: 1px solid var(--border);
      }
      .header-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
      }
      .header-title {
        font-size: var(--text-lg);
        font-weight: 600;
        font-family: var(--font-heading, inherit);
        margin: 0;
      }
      .header-sub {
        font-size: var(--text-sm);
        color: var(--muted-foreground);
        margin: var(--space-1) 0 0;
      }
      .tab-bar {
        padding: var(--space-2) var(--space-6) 0;
      }
      .content {
        flex: 1;
        padding: var(--space-6);
        overflow: auto;
        min-height: 0;
      }
      .tab-content {
        height: 100%;
      }
    `,
  ];

  #signals = new SignalController(this);

  @state()
  private _tab: TabId = "general";

  constructor() {
    super();
    this.#signals.watch(currentPath, auth);
  }

  private _syncTabFromPath(): void {
    const path = currentPath.value;
    const match = matchRoute("/settings/workspace/:tab", path);
    const tab = match?.tab as TabId | undefined;
    if (tab === "labels") {
      this._tab = "labels";
    } else if (tab === "audit" && isOrgElevatedRole(auth.value.user?.role)) {
      this._tab = "audit";
    } else {
      this._tab = "general";
    }
  }

  protected willUpdate(): void {
    this._syncTabFromPath();
  }

  private get _tabs(): TabItem[] {
    const elevated = isOrgElevatedRole(auth.value.user?.role);
    return elevated
      ? [...getBaseTabs(), { id: "audit", label: msg("Audit log") }]
      : getBaseTabs();
  }

  private _onTabChange(e: CustomEvent): void {
    const id = e.detail as TabId;
    this._tab = id;
    if (id === "general") {
      navigate("/settings/workspace");
    } else {
      navigate(`/settings/workspace/${id}`);
    }
  }

  protected render(): unknown {
    return html`
      <plume-app-layout>
        <div class="page page-enter">
          <div class="header">
            <div class="header-row">
              <div>
                <h1 class="header-title">${msg("Workspace settings")}</h1>
                <p class="header-sub">${msg(
                  "Manage your workspace preferences",
                )}</p>
              </div>
            </div>
          </div>

          <div class="tab-bar">
            <plume-tabs
              .tabs="${this._tabs}"
              .value="${this._tab}"
              @change="${this._onTabChange}"
            ></plume-tabs>
          </div>

          <div class="content">
            ${keyed(
              this._tab,
              html`
                <div class="tab-content" role="tabpanel" aria-labelledby="tab-${this
                  ._tab}">
                  ${this._renderTab()}
                </div>
              `,
            )}
          </div>
        </div>
      </plume-app-layout>
    `;
  }

  private _renderTab(): unknown {
    switch (this._tab) {
      case "general":
        return html`<plume-organization-settings-page embedded></plume-organization-settings-page>`;
      case "labels":
        return html`<plume-labels-settings-page embedded></plume-labels-settings-page>`;
      case "audit":
        return html`<plume-audit-log-page embedded></plume-audit-log-page>`;
      default:
        return nothing;
    }
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-workspace-settings-page": PlumeWorkspaceSettingsPage;
  }
}
