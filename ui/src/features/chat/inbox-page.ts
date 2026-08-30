import { html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";
import { SignalController } from "@/lib/signal-controller";
import { activeConversation, conversationList } from "./store";
import { chatApi } from "./api";
import "@/layouts/app-layout.ts";
import "./components/chat-layout.ts";
import "./components/create-dm-dialog.ts";
import { localized } from "@lit/localize";

const IC_STYLES = `
breeze-inbox-chat-page {
  display: block;
  height: 100svh;
  overflow: hidden;
}
`;

/**
 * Light DOM: required for @atlaskit/pragmatic-drag-and-drop in the chat
 * sidebar. Styles are global, prefixed `ic-`.
 */
@localized()
@customElement("breeze-inbox-chat-page")
export class BreezeInboxChatPage extends LitElement {
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
    } catch {
      activeConversation.value = null;
      this.#syncedId = "";
    }
  }

  protected render() {
    return html`
      <style>
      ${IC_STYLES}
      </style>
      <breeze-app-layout data-fullscreen>
        <breeze-chat-layout
          mode="dms"
          conversationId="${this.conversationId}"
        ></breeze-chat-layout>
        <breeze-create-dm-dialog></breeze-create-dm-dialog>
      </breeze-app-layout>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-inbox-chat-page": BreezeInboxChatPage;
  }
}
