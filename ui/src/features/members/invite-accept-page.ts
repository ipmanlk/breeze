import { css, html, LitElement } from "lit";
import { localized, msg } from "@lit/localize";
import { pageEnterStyles } from "@/styles/shared-animations";
import { customElement, state } from "lit/decorators.js";
import * as v from "valibot";
import { membersApi } from "../members/api";
import { navigate } from "@/routes/router";
import "@/components/ui/input.ts";
import "@/components/ui/button.ts";
import "@/components/ui/field.ts";
import "@/components/ui/card.ts";
import "@/components/ui/spinner.ts";
import "@/layouts/guest-layout.ts";

const AcceptInviteSchema = v.object({
  name: v.pipe(v.string(), v.nonEmpty("Name is required")),
  email: v.pipe(
    v.string(),
    v.nonEmpty("Email is required"),
    v.email("Invalid email"),
  ),
  password: v.pipe(
    v.string(),
    v.nonEmpty("Password is required"),
    v.minLength(8, "Password must be at least 8 characters"),
  ),
});

@localized()
@customElement("plume-invite-accept-page")
export class PlumeInviteAcceptPage extends LitElement {
  static styles = [
    pageEnterStyles,
    css`
      :host {
        display: contents;
      }
      plume-card {
        display: block;
        box-sizing: border-box;
        width: 100%;
        max-width: var(--container-sm);
      }
      .title {
        text-align: center;
        margin-bottom: var(--space-6);
      }
      .title .icon {
        display: block;
        margin: 0 auto var(--space-3);
        font-size: var(--space-8);
      }
      .title h1 {
        margin: 0;
        font-size: var(--text-2xl);
        font-weight: 600;
      }
      .title p {
        margin: var(--space-1) 0 0;
        font-size: var(--text-sm);
        color: var(--muted-foreground);
      }
      form {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
      }
      .form-error {
        font-size: var(--text-xs);
        font-weight: 500;
        color: var(--destructive);
      }
      .loading-wrap {
        display: flex;
        align-items: center;
        justify-content: center;
        min-height: 100svh;
      }
      .error-page {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        min-height: 100svh;
        padding: var(--space-6);
        text-align: center;
      }
      .error-page h1 {
        font-size: var(--text-lg);
        font-weight: 600;
        margin: var(--space-3) 0 var(--space-1);
      }
      .error-page p {
        font-size: var(--text-sm);
        color: var(--muted-foreground);
      }
    `,
  ];

  @state()
  private _validating = true;

  @state()
  private _valid = false;

  @state()
  private _error = "";

  @state()
  private _name = "";

  @state()
  private _email = "";

  @state()
  private _password = "";

  @state()
  private _fieldErrors: Record<string, string> = {};

  @state()
  private _submitting = false;

  private _token = "";

  connectedCallback(): void {
    super.connectedCallback();
    const params = new URLSearchParams(window.location.search);
    this._token = params.get("token") ?? "";

    if (!this._token) {
      this._validating = false;
      this._error = "No invite token provided.";
      return;
    }

    membersApi
      .validateInvite(this._token)
      .then(() => {
        this._valid = true;
        this._validating = false;
      })
      .catch(() => {
        this._error = "This invite link is invalid or has expired.";
        this._validating = false;
      });
  }

  async #onSubmit(e: SubmitEvent) {
    e.preventDefault();
    this._fieldErrors = {};
    this._error = "";

    const result = v.safeParse(AcceptInviteSchema, {
      name: this._name,
      email: this._email,
      password: this._password,
    });

    if (!result.success) {
      const issues: Record<string, string> = {};
      for (const issue of result.issues) {
        const key = issue.path?.[0]?.key ?? "form";
        issues[key as string] = issues[key as string] || issue.message;
      }
      this._fieldErrors = issues;
      return;
    }

    this._submitting = true;
    try {
      await membersApi.acceptInvite(this._token, result.output);
      navigate("/");
    } catch (err: unknown) {
      this._error = err instanceof Error
        ? err.message
        : msg("Failed to accept invite.");
    } finally {
      this._submitting = false;
    }
  }

  protected render() {
    if (this._validating) {
      return html`
        <div class="loading-wrap page-enter">
          <plume-spinner size="24"></plume-spinner>
        </div>
      `;
    }

    if (!this._valid) {
      return html`
        <div class="error-page page-enter">
          <span style="font-size:2.5rem">&#9888;</span>
          <h1>${msg("Invalid invite")}</h1>
          <p>${this._error}</p>
        </div>
      `;
    }

    return html`
      <plume-guest-layout>
        <div class="page-enter">
          <plume-card>
            <form @submit="${this.#onSubmit}" novalidate>
              <div class="title">
                <span class="icon">&#128279;</span>
                <h1>${msg("Join workspace")}</h1>
                <p>${msg("Accept your invite to get started.")}</p>
              </div>

              <plume-field
                label=${msg("Full name")}
                .error="${this._fieldErrors["name"] ?? ""}"
                ?invalid="${!!this._fieldErrors["name"]}"
              >
                <plume-input
                  id="name"
                  name="name"
                  placeholder=${msg("Jane Doe")}
                  .value="${this._name}"
                  @input="${(e: Event) => {
                    this._name = (e.target as HTMLInputElement).value;
                  }}"
                  ?invalid="${!!this._fieldErrors["name"]}"
                ></plume-input>
              </plume-field>

              <plume-field
                label=${msg("Email")}
                .error="${this._fieldErrors["email"] ?? ""}"
                ?invalid="${!!this._fieldErrors["email"]}"
              >
                <plume-input
                  id="email"
                  name="email"
                  type="email"
                  placeholder=${msg("you@example.com")}
                  .value="${this._email}"
                  @input="${(e: Event) => {
                    this._email = (e.target as HTMLInputElement).value;
                  }}"
                  ?invalid="${!!this._fieldErrors["email"]}"
                ></plume-input>
              </plume-field>

              <plume-field
                label=${msg("Password")}
                .error="${this._fieldErrors["password"] ?? ""}"
                ?invalid="${!!this._fieldErrors["password"]}"
              >
                <plume-input
                  id="password"
                  name="password"
                  type="password"
                  placeholder="${msg("At least 8 characters")}"
                  .value="${this._password}"
                  @input="${(e: Event) => {
                    this._password = (e.target as HTMLInputElement).value;
                  }}"
                  ?invalid="${!!this._fieldErrors["password"]}"
                ></plume-input>
              </plume-field>

              ${this._error
                ? html`
                  <div class="form-error">${this._error}</div>
                `
                : null}

              <plume-button type="submit" fluid ?disabled="${this
                ._submitting}">
                ${this._submitting
                  ? html`
                    <plume-spinner></plume-spinner> ${msg("Joining...")}
                  `
                  : msg("Join workspace")}
              </plume-button>
            </form>
          </plume-card>
        </div>
      </plume-guest-layout>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-invite-accept-page": PlumeInviteAcceptPage;
  }
}
