import { css, html, LitElement } from "lit";
import {
  contentEnterStyles,
  pageEnterStyles,
} from "@/styles/shared-animations";
import { customElement, state } from "lit/decorators.js";
import { live } from "lit/directives/live.js";
import * as v from "valibot";
import { getSetupSchema } from "@/lib/schemas/setup";
import { postSetup } from "@/api";
import { setupRequired } from "@/store/setup";
import { navigate } from "@/routes/router";
import { localized, msg } from "@lit/localize";
import { extractAuthError } from "@/features/auth/errors";
import "../../components/ui/field.ts";
import "../../components/ui/input.ts";
import "../../components/ui/button.ts";
import "../../components/ui/stepper.ts";
import "../../components/ui/spinner.ts";
import "../../components/ui/card.ts";
import "../../layouts/guest-layout.ts";

@localized()
@customElement("breeze-setup-page")
export class BreezeSetupPage extends LitElement {
  static styles = [
    pageEnterStyles,
    contentEnterStyles,
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
      .wrap {
        display: flex;
        flex-direction: column;
        align-items: center;
        width: 100%;
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
      breeze-stepper {
        width: var(--space-24);
        margin-bottom: var(--space-6);
      }
      breeze-card {
        display: block;
        box-sizing: border-box;
        width: 100%;
        max-width: var(--container-md);
      }
      form {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
      }
      .done {
        text-align: center;
        max-width: var(--container-sm);
      }
      .done h1 {
        margin: 0;
        font-size: var(--text-2xl);
        font-weight: 600;
      }
      .done p {
        font-size: var(--text-sm);
        color: var(--muted-foreground);
        margin: var(--space-4) 0;
      }
      .row {
        display: flex;
        gap: var(--space-2);
      }
      .row > breeze-button {
        flex: 1;
      }
      .form-error {
        font-size: var(--text-xs);
        font-weight: 500;
        color: var(--destructive);
      }
    `,
  ];

  @state()
  step = 0;
  @state()
  orgName = "";
  @state()
  name = "";
  @state()
  email = "";
  @state()
  password = "";
  @state()
  errors: Partial<
    Record<"orgName" | "name" | "email" | "password" | "form", string>
  > = {};
  @state()
  submitting = false;

  protected render() {
    if (setupRequired.value === false) {
      return html`
        <breeze-guest-layout>
          <div class="page-enter">
            <breeze-card class="done">
              <h1>Breeze</h1>
              <p>${msg("This workspace has already been configured.")}</p>
              <breeze-button fluid @click="${() => navigate("/login")}">${msg(
                "Go to login",
              )}</breeze-button>
            </breeze-card>
          </div>
        </breeze-guest-layout>
      `;
    }

    return html`
      <breeze-guest-layout>
        <div class="page-enter">
          <div class="wrap">
            <div class="title">
              <h1>Breeze</h1>
              <p>${msg("Set up your workspace")}</p>
            </div>
            <breeze-stepper steps="2" current="${this.step}"></breeze-stepper>
            ${this.step === 0
              ? html`
                <div class="content-enter">${this.#renderStep0()}</div>
              `
              : html`
                <div class="content-enter">${this.#renderStep1()}</div>
              `}
          </div>
        </div>
      </breeze-guest-layout>
    `;
  }

  #renderStep0() {
    return html`
      <breeze-card>
        <form @submit="${this.#onContinue}" novalidate>
          <breeze-field
            label=${msg("Organization name")}
            .error="${this.errors.orgName ?? ""}"
            ?invalid="${!!this.errors.orgName}"
          >
            <breeze-input
              id="orgName"
              name="orgName"
              type="text"
              placeholder=${msg("Acme Corp")}
              .value="${live(this.orgName)}"
              @input="${(
                e: Event,
              ) => (this.orgName = (e.target as HTMLInputElement).value)}"
              ?invalid="${!!this.errors.orgName}"
            ></breeze-input>
          </breeze-field>
          <breeze-button type="submit" fluid>${msg("Continue")}</breeze-button>
        </form>
      </breeze-card>
    `;
  }

  #renderStep1() {
    return html`
      <breeze-card>
        <form @submit="${this.#onSubmit}" novalidate>
          <breeze-field
            label=${msg("Your name")}
            .error="${this.errors.name ?? ""}"
            ?invalid="${!!this.errors.name}"
          >
            <breeze-input
              id="name"
              name="name"
              type="text"
              placeholder=${msg("Jane Doe")}
              .value="${live(this.name)}"
              @input="${(
                e: Event,
              ) => (this.name = (e.target as HTMLInputElement).value)}"
              ?invalid="${!!this.errors.name}"
            ></breeze-input>
          </breeze-field>
          <breeze-field
            label=${msg("Email")}
            .error="${this.errors.email ?? ""}"
            ?invalid="${!!this.errors.email}"
          >
            <breeze-input
              id="email"
              name="email"
              type="email"
              placeholder=${msg("jane@acme.com")}
              .value="${live(this.email)}"
              @input="${(
                e: Event,
              ) => (this.email = (e.target as HTMLInputElement).value)}"
              ?invalid="${!!this.errors.email}"
            ></breeze-input>
          </breeze-field>
          <breeze-field
            label=${msg("Password")}
            .error="${this.errors.password ?? ""}"
            ?invalid="${!!this.errors.password}"
          >
            <breeze-input
              id="password"
              name="password"
              type="password"
              placeholder=${msg("Min 8 characters")}
              .value="${live(this.password)}"
              @input="${(
                e: Event,
              ) => (this.password = (e.target as HTMLInputElement).value)}"
              ?invalid="${!!this.errors.password}"
            ></breeze-input>
          </breeze-field>
          ${this.errors.form
            ? html`
              <div class="form-error">${this.errors.form}</div>
            `
            : null}
          <div class="row">
            <breeze-button variant="outline" type="button" @click="${() => (this
              .step = 0)}"
            >${msg("Back")}</breeze-button>
            <breeze-button type="submit" fluid ?disabled="${this.submitting}"
            >${this.submitting
              ? html`
                <breeze-spinner></breeze-spinner><span>${msg(
                  "Setting up...",
                )}</span>
              `
              : msg("Create workspace")}</breeze-button>
          </div>
        </form>
      </breeze-card>
    `;
  }

  #onContinue = async (e: SubmitEvent) => {
    e.preventDefault();
    this.errors = {};
    const r = v.safeParse(getSetupSchema(), {
      orgName: this.orgName,
      name: this.name,
      email: this.email,
      password: this.password,
    });
    if (!r.success) {
      const orgIssue = r.issues.find((i) => i.path?.[0]?.key === "orgName");
      if (orgIssue) {
        this.errors = { orgName: orgIssue.message };
        return;
      }
    }
    this.step = 1;
  };

  #onSubmit = async (e: SubmitEvent) => {
    e.preventDefault();
    this.errors = {};
    const r = v.safeParse(getSetupSchema(), {
      orgName: this.orgName,
      name: this.name,
      email: this.email,
      password: this.password,
    });
    if (!r.success) {
      const issues = r.issues.reduce<Record<string, string>>((acc, issue) => {
        const key = issue.path?.[0]?.key ?? "form";
        acc[key as string] = issue.message;
        return acc;
      }, {});
      this.errors = issues as Partial<
        Record<"orgName" | "name" | "email" | "password" | "form", string>
      >;
      return;
    }
    this.submitting = true;
    try {
      await postSetup({
        body: {
          org_name: r.output.orgName,
          name: r.output.name,
          email: r.output.email,
          password: r.output.password,
        },
        throwOnError: true,
      });
      setupRequired.value = false;
      navigate("/login");
    } catch (err) {
      this.errors = {
        ...this.errors,
        form: extractAuthError(err, "Setup failed. Please try again."),
      };
    } finally {
      this.submitting = false;
    }
  };
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-setup-page": BreezeSetupPage;
  }
}
