# Controllers (ReactiveControllers)

## Location

`src/hooks/` is currently empty. In practice, reactive controllers live:

- in `src/lib/`: `SignalController` (`src/lib/signal-controller.ts`) and
  `OutsideClickController` (`src/lib/outside-click-controller.ts`)
- colocated inside feature folders where they are only used there (e.g.
  `src/features/chat/…`)

## Pattern

```
use-<thing>.ts → class <Thing>Controller
```

## Conventions

- One controller per file.
- Controllers subscribe to signals and expose reactive state to Lit elements.
- They can also contain plain utility functions (no controller class required).

## SignalController

`SignalController` is the general-purpose bridge: it watches one or more
`@preact/signals-core` signals and re-runs the host element's `willUpdate`/
`render` when any of them change. It is the primary mechanism for connecting
signal state to Lit components.

```ts
import { LitElement } from "lit";
import { SignalController } from "@/lib/signal-controller";
import { auth } from "@/store/auth";

class MyElement extends LitElement {
  #signals = new SignalController(this);

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(auth);
  }

  render() {
    return auth.value.isAuthenticated
      ? html`<p>Welcome</p>`
      : html`<p>Login</p>`;
  }
}
```

Feature-specific controllers (auth, theme, WS) are mostly unnecessary today
because `SignalController` handles the generic wiring; where a controller does
more (e.g. managing a WebSocket lifecycle), it lives next to the feature that
owns it.