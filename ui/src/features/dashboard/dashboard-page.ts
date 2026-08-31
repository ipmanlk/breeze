import { css, html, LitElement } from "lit";
import { pageEnterStyles } from "@/styles/shared-animations";
import { customElement } from "lit/decorators.js";
import { auth } from "@/store/auth";
import { dashboard, fetchDashboard } from "@/store/dashboard";
import { SignalController } from "@/lib/signal-controller";
import "../../layouts/app-layout.ts";
import "./components/dashboard-home.ts";
import { localized } from "@lit/localize";

@localized()
@customElement("plume-dashboard-page")
export class PlumeDashboardPage extends LitElement {
  static styles = [
    pageEnterStyles,
    css`
      *,
      *::before,
      *::after {
        box-sizing: border-box;
      }
      :host {
        display: contents;
      }
    `,
  ];

  #signals = new SignalController(this);
  #fetched = false;

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(auth, dashboard);
    if (!this.#fetched) {
      this.#fetched = true;
      fetchDashboard();
    }
  }

  protected render() {
    const { sections, isLoading } = dashboard.value;

    return html`
      <plume-app-layout>
        <div class="page-enter">
          <plume-dashboard-home
            .sections="${sections}"
            .isLoading="${isLoading}"
          ></plume-dashboard-home>
        </div>
      </plume-app-layout>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-dashboard-page": PlumeDashboardPage;
  }
}
