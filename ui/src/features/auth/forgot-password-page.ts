import { css, html, LitElement } from "lit";
import { customElement, state } from "lit/decorators.js";
import { live } from "lit/directives/live.js";
import { postAuthPasswordResetRequest } from "@/api";
import { pageEnterStyles } from "@/styles/shared-animations";
import { showToast } from "@/components/ui/toast-store";
import { localized, msg } from "@lit/localize";
import "../../components/ui/field.ts";
import "../../components/ui/input.ts";
import "../../components/ui/button.ts";
import "../../components/ui/spinner.ts";
import "../../components/ui/card.ts";
import "../../layouts/guest-layout.ts";

@localized()
@customElement("plume-forgot-password-page")
export class PlumeForgotPasswordPage extends LitElement {
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
      plume-card {
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
      .back {
        text-align: center;
        margin-top: var(--space-4);
      }
      .back a {
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
  submitting = false;
  @state()
  sent = false;

  protected render() {
    if (this.sent) {
      return html`
        <plume-guest-layout>
          <div class="page-enter">
            <plume-card>
              <div class="title">
                <h1>Email sent</h1>
                <p>If an account with that email exists, a reset link has been logged server-side. Check your server logs, or contact your administrator.</p>
              </div>
              <div class="back">
                <a @click=${() => (location.href = "/login")}>${msg(
                  "Back to login",
                )}</a>
              </div>
            </plume-card>
          </div>
        </plume-guest-layout>
      `;
    }

    return html`
      <plume-guest-layout>
        <div class="page-enter">
          <plume-card>
            <form @submit=${this.#onSubmit}>
              <div class="title">
                <h1>Forgot password</h1>
                <p>Enter your email to receive a reset link</p>
              </div>
              <plume-field label="Email">
                <plume-input
                  type="email"
                  placeholder="you@example.com"
                  .value=${live(this.email)}
                  @input=${(
                    e: Event,
                  ) => (this.email = (e.target as HTMLInputElement).value)}
                ></plume-input>
              </plume-field>
              <plume-button type="submit" fluid ?disabled=${this.submitting ||
                !this.email}>
                ${this.submitting
                  ? html`<plume-spinner></plume-spinner><span>Sending...</span>`
                  : "Send reset link"}
              </plume-button>
            </form>
            <div class="back">
              <a @click=${() => (location.href = "/login")}>${msg(
                "Back to login",
              )}</a>
            </div>
          </plume-card>
        </div>
      </plume-guest-layout>
    `;
  }

  async #onSubmit(e: SubmitEvent): Promise<void> {
    e.preventDefault();
    if (!this.email) return;
    this.submitting = true;
    try {
      await postAuthPasswordResetRequest({
        body: { email: this.email },
        throwOnError: true,
      });
      this.sent = true;
    } catch {
      showToast(msg("Failed to request password reset"), { variant: "error" });
    } finally {
      this.submitting = false;
    }
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-forgot-password-page": PlumeForgotPasswordPage;
  }
}
