import { css, html, LitElement } from "lit";
import { customElement, state } from "lit/decorators.js";
import { live } from "lit/directives/live.js";
import {
  getAuthPasswordResetValidate,
  postAuthPasswordResetConfirm,
} from "@/api";
import { pageEnterStyles } from "@/styles/shared-animations";
import { showToast } from "@/components/ui/toast-store";
import { localized, msg } from "@lit/localize";
import { navigate } from "@/routes/router";
import "../../components/ui/field.ts";
import "../../components/ui/input.ts";
import "../../components/ui/button.ts";
import "../../components/ui/spinner.ts";
import "../../components/ui/card.ts";
import "../../layouts/guest-layout.ts";

/**
 * Reset password page. Expects a `token` query parameter from the reset link.
 */
@localized()
@customElement("breeze-reset-password-page")
export class BreezeResetPasswordPage extends LitElement {
  static styles = [
    pageEnterStyles,
    css`
      :host {
        display: contents;
      }
      .page-enter {
        flex: none;
        align-items: center;
        justify-content: center;
        width: 100%;
      }
      breeze-card {
        display: block;
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
    `,
  ];

  @state()
  newPassword = "";
  @state()
  confirmPassword = "";
  @state()
  submitting = false;
  @state()
  error = "";
  @state()
  validating = true;
  @state()
  tokenInvalid = false;

  get #token(): string {
    return new URLSearchParams(window.location.search).get("token") ?? "";
  }

  async connectedCallback(): Promise<void> {
    super.connectedCallback();
    const token = this.#token;
    if (!token) {
      this.validating = false;
      this.tokenInvalid = true;
      return;
    }
    try {
      const { data } = await getAuthPasswordResetValidate({
        query: { token },
        throwOnError: true,
      });
      this.tokenInvalid = !data?.valid;
    } catch {
      // Network error: default to showing the form optimistically so a
      // transient failure doesn't block the user from submitting.
      this.tokenInvalid = false;
    } finally {
      this.validating = false;
    }
  }

  protected render() {
    if (this.validating) {
      return html`
        <breeze-guest-layout>
          <div class="page-enter">
            <breeze-card>
              <div class="title">
                <breeze-spinner></breeze-spinner>
                <p>${msg("Validating reset link...")}</p>
              </div>
            </breeze-card>
          </div>
        </breeze-guest-layout>
      `;
    }

    if (this.tokenInvalid) {
      return html`
        <breeze-guest-layout>
          <div class="page-enter">
            <breeze-card>
              <div class="title">
                <h1>${msg("Invalid or expired reset link")}</h1>
                <p>${msg(
                  "This password reset link is invalid or has expired. Please request a new one.",
                )}</p>
              </div>
            </breeze-card>
          </div>
        </breeze-guest-layout>
      `;
    }

    return html`
      <breeze-guest-layout>
        <div class="page-enter">
          <breeze-card>
            <form @submit=${this.#onSubmit}>
              <div class="title">
                <h1>${msg("Set new password")}</h1>
                <p>${msg("Enter your new password")}</p>
              </div>
              <breeze-field label="${msg("New password")}" ?invalid=${!!this
                .error}>
                <breeze-input
                  type="password"
                  placeholder="${msg("At least 8 characters")}"
                  .value=${live(this.newPassword)}
                  @input=${(
                    e: Event,
                  ) => (this.newPassword =
                    (e.target as HTMLInputElement).value)}
                ></breeze-input>
              </breeze-field>
              <breeze-field label=${msg("Confirm new password")}>
                <breeze-input
                  type="password"
                  .value=${live(this.confirmPassword)}
                  @input=${(
                    e: Event,
                  ) => (this.confirmPassword =
                    (e.target as HTMLInputElement).value)}
                ></breeze-input>
              </breeze-field>
              ${this.error
                ? html`<div class="form-error">${this.error}</div>`
                : null}
              <breeze-button type="submit" fluid ?disabled=${this.submitting ||
                !this.newPassword || !this.confirmPassword}>
                ${this.submitting
                  ? html`<breeze-spinner></breeze-spinner><span>${
                    msg("Resetting...")
                  }</span>`
                  : msg("Reset password")}
              </breeze-button>
            </form>
          </breeze-card>
        </div>
      </breeze-guest-layout>
    `;
  }

  async #onSubmit(e: SubmitEvent): Promise<void> {
    e.preventDefault();
    this.error = "";
    if (this.newPassword.length < 8) {
      this.error = msg("Password must be at least 8 characters");
      return;
    }
    if (this.newPassword !== this.confirmPassword) {
      this.error = msg("Passwords do not match");
      return;
    }
    this.submitting = true;
    try {
      await postAuthPasswordResetConfirm({
        body: { token: this.#token, new_password: this.newPassword },
        throwOnError: true,
      });
      showToast(msg("Password reset successfully — please log in"));
      navigate("/login");
    } catch (err) {
      this.error = err instanceof Error
        ? err.message
        : msg("Failed to reset password");
    } finally {
      this.submitting = false;
    }
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-reset-password-page": BreezeResetPasswordPage;
  }
}
