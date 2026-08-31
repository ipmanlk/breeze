import { css, html, LitElement } from "lit";
import { pageEnterStyles } from "@/styles/shared-animations";
import { customElement, state } from "lit/decorators.js";
import { live } from "lit/directives/live.js";
import * as v from "valibot";
import { getLoginSchema } from "@/lib/schemas/auth";
import { login } from "@/store/auth";
import { navigate } from "@/routes/router";
import { localized, msg } from "@lit/localize";
import { extractAuthError } from "./errors";
import "../../components/ui/field.ts";
import "../../components/ui/input.ts";
import "../../components/ui/button.ts";
import "../../components/ui/spinner.ts";
import "../../components/ui/card.ts";
import "../../layouts/guest-layout.ts";

@localized()
@customElement("plume-login-page")
export class PlumeLoginPage extends LitElement {
  static styles = [
    pageEnterStyles,
    css`
      :host {
        display: contents;
      }
      /* Override the .page-enter flex:1 from shared-animations.ts so the
        guest-layout's align/justify centering works correctly. */
      .page-enter {
        flex: none;
        align-items: center;
        justify-content: center;
        width: 100%;
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
      .forgot-link {
        text-align: center;
      }
      .forgot-link a {
        font-size: var(--text-sm);
        color: var(--muted-foreground);
        cursor: pointer;
        text-decoration: underline;
      }
    `,
  ];

  @state()
  email = "";
  @state()
  password = "";
  @state()
  errors: Partial<Record<"email" | "password" | "form", string>> = {};
  @state()
  submitting = false;

  protected render() {
    return html`
      <plume-guest-layout>
        <div class="page-enter">
          <plume-card>
            <form @submit="${this
              .#onSubmit}" novalidate class="${this.errors.form
              ? "shake"
              : ""}">
              <div class="title">
                <h1>Plume</h1>
                <p>${msg("Sign in to your workspace")}</p>
              </div>
              <plume-field
                label=${msg("Email")}
                .error="${this.errors.email ?? ""}"
                ?invalid="${!!this.errors.email}"
              >
                <plume-input
                  id="email"
                  name="email"
                  type="email"
                  placeholder=${msg("you@example.com")}
                  .value="${live(this.email)}"
                  @input="${(
                    e: Event,
                  ) => (this.email = (e.target as HTMLInputElement).value)}"
                  ?invalid="${!!this.errors.email}"
                ></plume-input>
              </plume-field>
              <plume-field
                label=${msg("Password")}
                .error="${this.errors.password ?? ""}"
                ?invalid="${!!this.errors.password}"
              >
                <plume-input
                  id="password"
                  name="password"
                  type="password"
                  placeholder=${msg("Your password")}
                  .value="${live(this.password)}"
                  @input="${(
                    e: Event,
                  ) => (this.password = (e.target as HTMLInputElement).value)}"
                  ?invalid="${!!this.errors.password}"
                ></plume-input>
              </plume-field>
              ${this.errors.form
                ? html`
                  <div class="form-error">${this.errors.form}</div>
                `
                : null}
              <plume-button type="submit" fluid ?disabled="${this.submitting}">
                ${this.submitting
                  ? html`
                    <plume-spinner></plume-spinner><span>${msg(
                      "Signing in...",
                    )}</span>
                  `
                  : msg("Sign in")}
              </plume-button>
              <div class="forgot-link">
                <a
                  href="/forgot-password"
                  @click=${(e: MouseEvent) => {
                    e.preventDefault();
                    navigate("/forgot-password");
                  }}
                >${msg("Forgot password?")}</a>
              </div>
            </form>
          </plume-card>
        </div>
      </plume-guest-layout>
    `;
  }

  #onSubmit = async (e: SubmitEvent) => {
    e.preventDefault();
    this.errors = {};
    const result = v.safeParse(getLoginSchema(), {
      email: this.email,
      password: this.password,
    });
    if (!result.success) {
      const issues = result.issues.reduce<Record<string, string>>(
        (acc, issue) => {
          const key = issue.path?.[0]?.key ?? "form";
          acc[key as string] = issue.message;
          return acc;
        },
        {},
      );
      this.errors = issues as Partial<
        Record<"email" | "password" | "form", string>
      >;
      return;
    }
    this.submitting = true;
    try {
      await login(result.output.email, result.output.password);
      const next = new URLSearchParams(window.location.search).get("next");
      // Only allow same-origin relative paths to avoid open-redirect
      if (next && next.startsWith("/") && !next.startsWith("//")) {
        navigate(next);
      } else {
        navigate("/");
      }
    } catch (err) {
      this.errors = {
        ...this.errors,
        form: extractAuthError(err),
      };
    } finally {
      this.submitting = false;
    }
  };
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-login-page": PlumeLoginPage;
  }
}
