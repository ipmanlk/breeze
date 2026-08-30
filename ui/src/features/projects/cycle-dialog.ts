import { css, html, LitElement, nothing } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import type { DtoCycleResponse, DtoProjectResponse } from "@/api";
import { createCycle, updateCycle } from "./cycles-store";
import "../../components/ui/dialog.ts";
import "../../components/ui/input.ts";
import "../../components/ui/field.ts";
import "../../components/ui/button.ts";
import { localized, msg } from "@lit/localize";

function toInputDate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

/**
 * Cycle dialog: create or edit a cycle. `cycle` is `null` for create, or the
 * cycle to edit. Uses native `<input type="date">` (no date-picker dependency).
 *
 * Properties: `open`, `project`, `cycle` (null = create).
 * Events: `close` (on dismiss), `saved` (after a successful create/update).
 */
@localized()
@customElement("breeze-cycle-dialog")
export class BreezeCycleDialog extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    .body {
      display: flex;
      flex-direction: column;
      gap: var(--space-4);
    }
    .grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: var(--space-3);
    }
    .error {
      color: var(--destructive);
      font-size: var(--text-sm);
    }
    .footer {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
    }
    .spacer {
      flex: 1;
    }
  `;

  @property({ type: Boolean })
  open = false;

  @property({ type: Object, attribute: false })
  project: DtoProjectResponse | null = null;

  @property({ type: Object, attribute: false })
  cycle: DtoCycleResponse | null = null;

  @state()
  private _name = "";
  @state()
  private _goal = "";
  @state()
  private _startsAt = "";
  @state()
  private _endsAt = "";
  @state()
  private _saving = false;
  @state()
  private _error = "";

  private _prevOpen = false;

  protected updated(changed: Map<string, unknown>) {
    if (changed.has("open") && this.open && !this._prevOpen) {
      this._reset();
    }
    this._prevOpen = this.open;
  }

  private _reset() {
    if (this.cycle) {
      this._name = this.cycle.name ?? "";
      this._goal = this.cycle.goal ?? "";
      this._startsAt = this.cycle.starts_at
        ? toInputDate(new Date(this.cycle.starts_at))
        : "";
      this._endsAt = this.cycle.ends_at
        ? toInputDate(new Date(this.cycle.ends_at))
        : "";
    } else {
      this._name = "";
      this._goal = "";
      const now = new Date();
      this._startsAt = toInputDate(now);
      this._endsAt = toInputDate(addDays(now, 14));
    }
    this._error = "";
    this._saving = false;
  }

  private async _submit(e: Event) {
    e.preventDefault();
    const pid = this.project?.id;
    if (!pid) return;
    if (!this._startsAt || !this._endsAt) {
      this._error = "Start and end dates are required";
      return;
    }
    const start = new Date(this._startsAt);
    const end = new Date(this._endsAt);
    if (end <= start) {
      this._error = "End date must be after start date";
      return;
    }
    this._saving = true;
    this._error = "";
    const body = {
      name: this._name.trim() || undefined,
      goal: this._goal.trim() || undefined,
      starts_at: start.toISOString(),
      ends_at: end.toISOString(),
    };
    try {
      if (this.cycle?.id) {
        await updateCycle(pid, this.cycle.id, body);
      } else {
        await createCycle(pid, body);
      }
      this.dispatchEvent(
        new CustomEvent("saved", { bubbles: true, composed: true }),
      );
      this.open = false;
    } catch (err) {
      this._error = err instanceof Error
        ? err.message
        : msg("Failed to save cycle");
    } finally {
      this._saving = false;
    }
  }

  protected render() {
    const isEdit = !!this.cycle;
    return html`
      <breeze-dialog
        .open="${this.open}"
        heading="${isEdit ? "Edit cycle" : "New cycle"}"
        @close="${() => {
          this.dispatchEvent(
            new CustomEvent("close", { bubbles: true, composed: true }),
          );
        }}"
      >
        <form class="body" @submit="${this._submit}" id="cycle-form">
          <breeze-field label="Name">
            <breeze-input
              placeholder=${msg("Cycle name (auto if empty)")}
              .value="${this._name}"
              @input="${(
                e: Event,
              ) => (this._name = (e.target as HTMLInputElement).value)}"
            ></breeze-input>
          </breeze-field>
          <breeze-field label="Goal">
            <breeze-input
              placeholder=${msg("What is the goal of this cycle?")}
              .value="${this._goal}"
              @input="${(
                e: Event,
              ) => (this._goal = (e.target as HTMLInputElement).value)}"
            ></breeze-input>
          </breeze-field>
          <div class="grid">
            <breeze-field label="Start">
              <breeze-input
                type="date"
                .value="${this._startsAt}"
                @input="${(
                  e: Event,
                ) => (this._startsAt = (e.target as HTMLInputElement).value)}"
              ></breeze-input>
            </breeze-field>
            <breeze-field label="End">
              <breeze-input
                type="date"
                .value="${this._endsAt}"
                @input="${(
                  e: Event,
                ) => (this._endsAt = (e.target as HTMLInputElement).value)}"
              ></breeze-input>
            </breeze-field>
          </div>
          ${this._error
            ? html`
              <span class="error">${this._error}</span>
            `
            : nothing}
        </form>

        <div class="footer" slot="footer">
          <span class="spacer"></span>
          <breeze-button
            variant="ghost"
            @click="${() => (this.open = false)}"
          >Cancel</breeze-button>
          <breeze-button
            ?disabled="${this._saving}"
            @click="${this._submit}"
          >${this._saving
            ? "Saving..."
            : isEdit
            ? "Save"
            : "Create"}</breeze-button>
        </div>
      </breeze-dialog>
    `;
  }
}

function addDays(d: Date, n: number): Date {
  const r = new Date(d);
  r.setDate(r.getDate() + n);
  return r;
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-cycle-dialog": BreezeCycleDialog;
  }
}
