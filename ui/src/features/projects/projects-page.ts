import { css, html, LitElement } from "lit";
import { pageEnterStyles } from "@/styles/shared-animations";
import { customElement, state } from "lit/decorators.js";
import { navigate } from "@/routes/router";
import {
  createProject,
  fetchProjects,
  fetchProjectsIncludingArchived,
  projects,
  unarchiveProject,
} from "@/store/projects";
import type { DtoCreateProjectRequest } from "@/api";
import { SignalController } from "@/lib/signal-controller";
import "../../layouts/app-layout.ts";
import "../../components/ui/breeze-icon.ts";
import "../../components/ui/button.ts";
import "../../components/ui/card.ts";
import "../../components/ui/input.ts";
import "../../components/ui/field.ts";
import "../../components/ui/switch.ts";
import { localized, msg } from "@lit/localize";

function getColors() {
  return [
    { value: "oklch(0.6 0.15 250)", label: msg("Blue") },
    { value: "oklch(0.55 0.15 265)", label: msg("Indigo") },
    { value: "oklch(0.55 0.2 290)", label: msg("Violet") },
    { value: "oklch(0.6 0.2 340)", label: msg("Pink") },
    { value: "oklch(0.6 0.2 10)", label: msg("Rose") },
    { value: "oklch(0.65 0.2 40)", label: msg("Orange") },
    { value: "oklch(0.7 0.2 80)", label: msg("Amber") },
    { value: "oklch(0.75 0.15 100)", label: msg("Lime") },
    { value: "oklch(0.6 0.2 150)", label: msg("Green") },
    { value: "oklch(0.6 0.15 170)", label: msg("Emerald") },
    { value: "oklch(0.6 0.1 190)", label: msg("Teal") },
    { value: "oklch(0.6 0.15 230)", label: msg("Sky") },
    { value: "oklch(0.65 0.1 280)", label: msg("Purple") },
    { value: "oklch(0.65 0.2 330)", label: msg("Fuchsia") },
  ];
}

@localized()
@customElement("breeze-projects-page")
export class BreezeProjectsPage extends LitElement {
  static styles = [
    pageEnterStyles,
    css`
      :host {
        display: contents;
      }
      .page {
        display: flex;
        flex-direction: column;
        height: 100%;
      }
      .page-head {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--space-4);
        padding: var(--space-4) var(--space-6);
        border-bottom: 1px solid var(--border);
        flex-shrink: 0;
      }
      .page-head h1 {
        margin: 0;
        font-size: var(--text-lg);
        font-weight: 600;
        font-family: var(--font-heading, inherit);
        color: var(--foreground);
      }
      .page-head p {
        margin: var(--space-1) 0 0;
        font-size: var(--text-sm);
        color: var(--muted-foreground);
      }
      .page-content {
        flex: 1;
        min-height: 0;
        overflow-y: auto;
        padding: var(--space-6);
      }
      .grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(16rem, 1fr));
        gap: var(--space-4);
      }
      .project-card {
        display: flex;
        align-items: center;
        gap: var(--space-3);
        padding: var(--space-4);
        border: 1px solid var(--border);
        border-radius: var(--radius-lg);
        background: var(--card);
        color: var(--card-foreground);
        cursor: pointer;
        text-align: left;
        transition: background var(--dur-fast) var(--ease-1);
      }
      .project-card:hover {
        background: var(--accent);
      }
      .project-icon {
        display: flex;
        align-items: center;
        justify-content: center;
        width: var(--avatar-lg);
        height: var(--avatar-lg);
        border-radius: var(--radius-md);
        font-size: var(--text-lg);
        font-weight: 600;
        color: white;
        flex-shrink: 0;
      }
      .project-info {
        flex: 1;
        min-width: 0;
      }
      .project-name {
        font-size: var(--text-sm);
        font-weight: 600;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .project-desc {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        margin-top: var(--space-0-5);
      }
      .project-card.is-archived {
        opacity: 0.65;
      }
      .archived-badge {
        display: inline-block;
        margin-left: var(--space-1-5);
        padding: 0 var(--space-1-5);
        font-size: var(--text-2xs);
        font-weight: 500;
        text-transform: uppercase;
        letter-spacing: 0.04em;
        color: var(--muted-foreground);
        background: var(--muted);
        border-radius: var(--radius-full);
        vertical-align: middle;
      }
      .empty {
        display: flex;
        flex: 1;
        align-items: center;
        justify-content: center;
        padding: var(--space-8);
        font-size: var(--text-sm);
        color: var(--muted-foreground);
      }

      .skel-card {
        display: flex;
        align-items: center;
        gap: var(--space-3);
        padding: var(--space-4);
        border: 1px solid var(--border);
        border-radius: var(--radius-lg);
      }
      .skel-icon {
        width: var(--avatar-lg);
        height: var(--avatar-lg);
        border-radius: var(--radius-md);
        background: var(--muted);
      }
      .skel-body {
        flex: 1;
        min-width: 0;
      }
      .skel-line {
        background: var(--muted);
        border-radius: var(--radius-sm);
      }
      .skel-name {
        height: var(--space-3-5);
        width: 50%;
        margin-bottom: var(--space-1);
      }
      .skel-desc {
        height: var(--space-3);
        width: 70%;
      }

      /* Dialog overlay */
      .overlay {
        position: fixed;
        inset: 0;
        z-index: var(--z-dialog);
        display: flex;
        align-items: center;
        justify-content: center;
        background: rgba(0, 0, 0, 0.5);
      }
      .dialog {
        width: 100%;
        max-width: var(--container-md);
        max-height: 90vh;
        overflow-y: auto;
        background: var(--popover);
        color: var(--popover-foreground);
        border: 1px solid var(--border);
        border-radius: var(--radius-lg);
        box-shadow: var(--shadow-lg);
        padding: var(--space-6);
      }
      .dialog h2 {
        margin: 0 0 var(--space-4);
        font-size: var(--text-lg);
        font-weight: 600;
      }
      .dialog-actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-2);
        margin-top: var(--space-4);
      }
      .field-group {
        display: flex;
        flex-direction: column;
        gap: var(--space-3);
      }
      .cycle-toggle {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--space-3);
        padding: var(--space-3) var(--space-4);
        border: 1px solid var(--border);
        border-radius: var(--radius-lg);
      }
      .cycle-toggle-label {
        font-size: var(--text-sm);
        font-weight: 500;
      }
      .cycle-toggle-desc {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
      /* Inline color picker */
      .color-section {
        display: flex;
        flex-direction: column;
        gap: var(--space-3);
      }
      .color-section-label {
        font-size: var(--text-xs);
        font-weight: 500;
        color: var(--muted-foreground);
        text-transform: uppercase;
        letter-spacing: 0.05em;
      }
      .color-grid {
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-2);
        padding: var(--space-3);
        border: 1px solid var(--border);
        border-radius: var(--radius-lg);
        background: var(--muted);
      }
      .color-swatch {
        width: var(--space-7);
        height: var(--space-7);
        border-radius: var(--radius-full);
        border: 2px solid transparent;
        cursor: pointer;
        display: flex;
        align-items: center;
        justify-content: center;
        transition:
          transform var(--dur-fast) var(--ease-1),
          box-shadow var(--dur-fast) var(--ease-1);
        background-clip: padding-box;
      }
      .color-swatch:hover {
        transform: scale(1.1);
        box-shadow: 0 0 0 1px
          color-mix(in oklch, var(--foreground) 20%, transparent);
      }
      .color-swatch.selected {
        box-shadow: 0 0 0 2px var(--foreground);
        transform: scale(1.1);
      }
      .color-swatch .check {
        filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.3));
      }
      .color-selected-label {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        font-size: var(--text-sm);
        color: var(--foreground);
      }
      .color-selected-dot {
        width: var(--space-4);
        height: var(--space-4);
        border-radius: var(--radius-sm);
        flex-shrink: 0;
      }
      .advanced-toggle {
        display: flex;
        align-items: center;
        gap: var(--space-1);
        font-size: var(--text-xs);
        font-weight: 500;
        color: var(--muted-foreground);
        background: none;
        border: none;
        cursor: pointer;
        padding: 0;
        font-family: inherit;
      }
      .advanced-toggle:hover {
        color: var(--foreground);
      }
      .advanced-content {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
        margin-top: var(--space-3);
      }
      .error-msg {
        font-size: var(--text-sm);
        color: var(--destructive);
      }
    `,
  ];

  @state()
  private _showCreate = false;

  @state()
  private _showArchived = false;

  @state()
  private _name = "";

  @state()
  private _description = "";

  @state()
  private _enableCycles = false;

  @state()
  private _showAdvanced = false;

  @state()
  private _color = getColors()[0].value;

  @state()
  private _apiError = "";

  @state()
  private _isSubmitting = false;

  #signals = new SignalController(this);

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(projects);
    fetchProjects();
    // Check if ?create=1 is present and auto-open dialog
    const params = new URLSearchParams(window.location.search);
    if (params.get("create") === "1") {
      this._showCreate = true;
    }
  }

  #resetForm() {
    this._name = "";
    this._description = "";
    this._enableCycles = false;
    this._showAdvanced = false;
    this._color = getColors()[0].value;
    this._apiError = "";
    this._isSubmitting = false;
  }

  #closeDialog() {
    this._showCreate = false;
    this.#resetForm();
    // Clear ?create=1 from URL if present
    const params = new URLSearchParams(window.location.search);
    if (params.get("create") === "1") {
      params.delete("create");
      const newSearch = params.toString();
      const newUrl = window.location.pathname +
        (newSearch ? "?" + newSearch : "");
      window.history.replaceState(null, "", newUrl);
    }
  }

  #renderColorPicker() {
    const selectedColor = getColors().find((c) => c.value === this._color) ??
      getColors()[0];
    return html`
      <div class="color-section">
        <div class="color-selected-label">
          <span class="color-selected-dot" style="background:${this
            ._color}"></span>
          ${selectedColor.label}
        </div>
        <div class="color-grid">
          ${getColors().map(
            (c) =>
              html`
                <button
                  type="button"
                  class="color-swatch ${this._color === c.value
                    ? "selected"
                    : ""}"
                  style="background:${c.value}"
                  title="${c.label}"
                  @click="${() => {
                    this._color = c.value;
                  }}"
                >
                  ${this._color === c.value
                    ? html`
                      <breeze-icon
                        class="check"
                        name="check"
                        size="16"
                        style="color:white"
                      ></breeze-icon>
                    `
                    : ""}
                </button>
              `,
          )}
        </div>
      </div>
    `;
  }

  async #onCreate() {
    if (!this._name.trim()) return;
    this._isSubmitting = true;
    this._apiError = "";

    const body: Record<string, unknown> = { name: this._name.trim() };
    if (this._description.trim()) {
      body.description = this._description.trim();
    }
    if (this._color !== getColors()[0].value) {
      body.color = this._color;
    }
    if (this._enableCycles) {
      body.cycle_duration = 14;
      body.auto_generate_cycles = true;
    }

    const result = await createProject(body as DtoCreateProjectRequest);
    if (result) {
      this.#closeDialog();
    } else {
      this._apiError = msg("Failed to create project. Please try again.");
      this._isSubmitting = false;
    }
  }

  protected render() {
    const { projects: list, isLoading } = projects.value;

    return html`
      <breeze-app-layout>
        <div class="page page-enter">
          <div class="page-head">
            <div>
              <h1>Projects</h1>
              <p>All projects in your workspace.</p>
            </div>
            <div style="display:flex;align-items:center;gap:var(--space-3)">
              <breeze-switch
                ?checked="${this._showArchived}"
                @change="${(e: CustomEvent) => {
                  this._showArchived =
                    (e.detail as { checked: boolean }).checked;
                  if (this._showArchived) {
                    fetchProjectsIncludingArchived();
                  } else {
                    fetchProjects();
                  }
                }}"
              ></breeze-switch>
              <span style="font-size:var(--text-sm);color:var(--muted-foreground)">Show archived</span>
              <breeze-button size="sm" @click="${() => {
                this._showCreate = true;
              }}">
                <breeze-icon name="plus" size="16"></breeze-icon>
                New project
              </breeze-button>
            </div>
          </div>

          <div class="page-content">
            ${isLoading
              ? html`
                <div class="grid">
                  ${[1, 2, 3].map(
                    () =>
                      html`
                        <div class="skel-card">
                          <div class="skel-icon"></div>
                          <div class="skel-body">
                            <div class="skel-line skel-name"></div>
                            <div class="skel-line skel-desc"></div>
                          </div>
                        </div>
                      `,
                  )}
                </div>
              `
              : list.length === 0
              ? html`
                <div class="empty">
                  ${this._showArchived
                    ? "No projects found (including archived)."
                    : "No projects yet. Create your first project to get started."}
                </div>
              `
              : html`
                <div class="grid">
                  ${list.map(
                    (p) =>
                      html`
                        <button
                          class="project-card ${p.is_archived
                            ? "is-archived"
                            : ""}"
                          @click="${() => navigate(`/projects/${p.slug}`)}"
                        >
                          <div
                            class="project-icon"
                            style="background:${p.color ??
                              getColors()[0].value}"
                          >
                            ${(p.icon ?? p.name ?? "?").charAt(0).toUpperCase()}
                          </div>
                          <div class="project-info">
                            <div class="project-name">
                              ${p.name}
                              ${p.is_archived
                                ? html`<span class="archived-badge">Archived</span>`
                                : ""}
                            </div>
                            <div class="project-desc">
                              ${p.description || "No description"}
                            </div>
                          </div>
                          ${p.is_archived
                            ? html`
                              <breeze-button
                                variant="outline"
                                size="sm"
                                @click="${async (e: Event) => {
                                  e.stopPropagation();
                                  if (p.id) await unarchiveProject(p.id);
                                }}"
                              >
                                Restore
                              </breeze-button>
                            `
                            : ""}
                        </button>
                      `,
                  )}
                </div>
              `}
          </div>
        </div>
        ${this._showCreate
          ? html`
            <div
              class="overlay"
              @click="${(e: Event) => {
                if (e.target === e.currentTarget) {
                  this.#closeDialog();
                }
              }}"
            >
              <div class="dialog">
                <h2>Create project</h2>
                <div class="field-group">
                  <breeze-field label="Name" .error="${!this._name.trim() &&
                      this._isSubmitting
                    ? "Name is required"
                    : ""}">
                    <breeze-input
                      name="name"
                      placeholder=${msg("e.g. Q4 Launch")}
                      .value="${this._name}"
                      @input="${(e: Event) => {
                        this._name = (e.target as HTMLInputElement).value;
                      }}"
                    ></breeze-input>
                  </breeze-field>

                  <div class="cycle-toggle">
                    <div>
                      <div class="cycle-toggle-label">Cycles</div>
                      <div class="cycle-toggle-desc">
                        Track work in time-boxed cycles
                      </div>
                    </div>
                    <label style="cursor:pointer;display:flex;align-items:center">
                      <input
                        type="checkbox"
                        ?checked="${this._enableCycles}"
                        @change="${(e: Event) => {
                          this._enableCycles =
                            (e.target as HTMLInputElement).checked;
                        }}"
                      />
                    </label>
                  </div>

                  <button
                    class="advanced-toggle"
                    @click="${() => {
                      this._showAdvanced = !this._showAdvanced;
                    }}"
                  >
                    <breeze-icon
                      name="chevron-down"
                      size="12"
                      style="transform:rotate(${this._showAdvanced
                        ? "0"
                        : "-90deg"});transition:transform var(--dur-fast) var(--ease-1)"
                    ></breeze-icon>
                    Advanced options
                  </button>

                  ${this._showAdvanced
                    ? html`
                      <div class="advanced-content">
                        <breeze-field label="Description">
                          <textarea
                            style="display:block;width:100%;min-height: var(--space-20);padding:var(--space-3);border:1px solid var(--input);border-radius:var(--radius-md);background:var(--background);color:var(--foreground);font-size:var(--text-sm);font-family:inherit;resize:vertical;box-sizing:border-box;outline:none"
                            placeholder=${msg("What is this project about?")}
                            .value="${this._description}"
                            @input="${(e: Event) => {
                              this._description =
                                (e.target as HTMLTextAreaElement).value;
                            }}"
                          ></textarea>
                        </breeze-field>

                        <breeze-field label="Color">
                          ${this.#renderColorPicker()}
                        </breeze-field>
                      </div>
                    `
                    : ""} ${this._apiError
                    ? html`
                      <div class="error-msg">${this._apiError}</div>
                    `
                    : ""}
                </div>

                <div class="dialog-actions">
                  <breeze-button
                    variant="outline"
                    size="sm"
                    @click="${this.#closeDialog}"
                  >
                    Cancel
                  </breeze-button>
                  <breeze-button
                    size="sm"
                    ?disabled="${this._isSubmitting || !this._name.trim()}"
                    @click="${this.#onCreate}"
                  >
                    ${this._isSubmitting ? "Creating..." : "Create project"}
                  </breeze-button>
                </div>
              </div>
            </div>
          `
          : ""}
      </breeze-app-layout>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-projects-page": BreezeProjectsPage;
  }
}
