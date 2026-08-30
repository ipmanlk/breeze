import type { ReactiveController, ReactiveControllerHost } from "lit";
import { effect, type Signal } from "@preact/signals-core";

/**
 * Lit `ReactiveController` that bridges `@preact/signals-core` signals to
 * Lit's reactivity system.
 *
 * When any watched signal changes, the host element re-renders automatically.
 * Effects are created on `hostConnected` and disposed on `hostDisconnected`,
 * so there is no manual `effect()` / `dispose()` bookkeeping needed.
 *
 * @example
 * ```ts
 * class MyEl extends LitElement {
 *   #signals = new SignalController(this);
 *   constructor() {
 *     super();
 *     this.#signals.watch(auth, dashboard);
 *   }
 * }
 * ```
 */
export class SignalController implements ReactiveController {
  #host: ReactiveControllerHost;
  #signals: Signal<unknown>[] = [];
  #dispose?: () => void;

  constructor(host: ReactiveControllerHost) {
    this.#host = host;
    host.addController(this);
  }

  /** Register signals to track. Can be called before or after connection. */
  watch(...signals: Signal<unknown>[]): void {
    this.#signals.push(...signals);
    // If already connected, recreate the effect to pick up new signals.
    if (this.#dispose) {
      this.#start();
    }
  }

  hostConnected(): void {
    this.#start();
  }

  hostDisconnected(): void {
    this.#dispose?.();
    this.#dispose = undefined;
  }

  #start(): void {
    this.#dispose?.();
    this.#dispose = effect(() => {
      // Reading .value inside effect() registers the dependency.
      for (const sig of this.#signals) sig.value;
      this.#host.requestUpdate();
    });
  }
}
