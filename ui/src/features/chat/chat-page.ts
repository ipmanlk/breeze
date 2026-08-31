import { logError } from "@/lib/log";
import { html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";
import { SignalController } from "@/lib/signal-controller";
import { activeConversation, conversationList } from "./store";
import { chatApi } from "./api";
import { showToast } from "@/components/ui/toast-store";
import "@/layouts/app-layout.ts";
import "./components/chat-layout.ts";
import { localized, msg } from "@lit/localize";

const CP_STYLES = `
plume-chat-page { display: block; }
`;

/**
 * Light DOM: required for @atlaskit/pragmatic-drag-and-drop in the workspace
 * sidebar. `plume-app-layout` keeps its shadow DOM (content is slotted into
 * the light DOM tree), but this page must be light DOM so the chain from the
 * sidebar up to the document is unbroken. Styles are global, prefixed `cp-`.
 */
@localized()
@customElement("plume-chat-page")
export class PlumeChatPage extends LitElement {
  createRenderRoot() {
    return this;
  }

  @property()
  conversationId = "";

  #signals = new SignalController(this);
  #syncedId = "";

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(conversationList);
  }

  protected willUpdate(changed: Map<string, unknown>) {
    if (changed.has("conversationId")) {
      this.#syncUrlToSignal();
    }
  }

  async #syncUrlToSignal() {
    const id = this.conversationId;

    if (!id) {
      activeConversation.value = null;
      this.#syncedId = "";
      return;
    }

    if (id === this.#syncedId) return;

    const found = conversationList.value.find((c) => c.id === id);
    if (found) {
      activeConversation.value = found;
      this.#syncedId = id;
      return;
    }

    try {
      const conv = await chatApi.getConversation(id);
      activeConversation.value = conv;
      this.#syncedId = id;
    } catch (err) {
      logError("chat-page: failed to load conversation:", err);
      showToast(msg("Failed to load conversation"), { variant: "error" });
      activeConversation.value = null;
      this.#syncedId = "";
    }
  }

  protected render() {
    return html`
      <style>
      ${CP_STYLES}
      </style>
      <plume-app-layout data-fullscreen>
        <plume-chat-layout
          mode="workspace"
          conversationId="${this.conversationId}"
        ></plume-chat-layout>
      </plume-app-layout>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-chat-page": PlumeChatPage;
  }
}
