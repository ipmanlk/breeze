# Store (global state)

## Location

`src/store/` contains module-level signal singletons using
`@preact/signals-core`.

## Modules

| File                  | Contains                                                                                                                             |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| `auth.ts`             | `auth` signal with user, isLoading, isAuthenticated. Actions: `fetchMe()`, `login()`, `logout()`.                                    |
| `theme.ts`            | `theme`, `colorTheme` signals. Actions: `initTheme()`, `toggleTheme()`, `setColorTheme()`. Persists to localStorage.                  |
| `setup.ts`            | `setupRequired` signal. Action: `checkSetup()`.                                                                                      |
| `ws.ts`               | Global WebSocket singleton (`wsClient`, `connectionStatus`, `wsUserId`). Auto-reconnect with exponential backoff. Connected at app level via `connectWs()`/`disconnectWs()`. |
| `workspaces.ts`       | `workspaces` signal + `activeOrgID`; actions `fetchWorkspaces()`, `switchActiveWorkspace()`. |
| `projects.ts`         | `projects` signal; actions `fetchProjects()`, `createProject()`.                          |
| `project-detail.ts`   | The open project's data (tasks, statuses, views) as a server-state cache.                  |
| `dashboard.ts`        | Dashboard data + widget reordering.                                                         |
| `preferences.ts`      | User preferences (language, notification toggles).                                          |
| `push.ts`             | Web Push registration state (service worker).                                               |
| `sidebar.ts`          | Sidebar collapsed/expanded + mobile state.                                                  |
| `motion.ts`           | UI motion/reduced-motion preferences.                                                       |

## Pattern

```ts
import { signal } from "@preact/signals-core";

export const myState = signal<MyState>(initialValue);

export async function doSomething(): Promise<void> {
  const { data } = await someApiCall({ throwOnError: true });
  myState.value = data; // triggers re-render
}
```

- Signals are exported as module-level singletons.
- Components use `effect()` or `signal.watch()` to subscribe.
- Async actions update the signal after the API call resolves.

## Server-state cache

For cacheable server data, signals are used as a minimal server-state cache (key
→ T), replacing React Query's role from v1.

## Signal bridge pattern (WS events)

Real-time features (chat, inbox) use a **signal bridge** to push WebSocket
events into component state:

```
WS onmessage → feature handler → wsMessageEvents signal (in feature store)
                                → components watch in willUpdate() / updated()
```

The global `ws.ts` singleton is connected/disconnected at the app shell level.
Each feature subscribes/unsubscribes to topics in
`connectedCallback`/`disconnectedCallback`. A single `wsMessageEvents` signal
carries typed event payloads (message_new, message_updated, message_deleted,
etc.) so multiple components can react without tight coupling.

## Future

- `query.ts`: generalized server-state cache utilities (future)
