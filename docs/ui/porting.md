# Porting from v1 (React) to v2 (Lit)

| v1 (React)           | v2 (Lit)                                                                   |
| -------------------- | -------------------------------------------------------------------------- |
| `function Component` | `class extends LitElement`                                                 |
| `useState`           | `@property` or `signal`                                                    |
| `useEffect`          | `connectedCallback` / `firstUpdated`                                       |
| `useMemo`            | `computed()` (signal)                                                      |
| `useCallback`        | not needed; methods are stable                                            |
| `useRef`             | private field `#foo`                                                       |
| Context API          | `@lit/context`                                                             |
| React Query          | signal stores (feature-local `store.ts`)                                              |
| Jotai                | `@preact/signals-core`                                                     |
| react-hook-form      | native `<form>` + `FormData`                                               |
| wouter               | custom `popstate` router                                                       |
| Tailwind             | CSS tokens + scoped styles                                                 |
| shadcn/Radix         | hand-rolled `<breeze-*>` + native `<dialog>`/`popover`                     |
| `@tiptap/react`      | `@tiptap/core` + Lit wrapper                                               |
| `WsManager` (React)  | Global `ws.ts` singleton + feature-local signal bridge (`wsMessageEvents`) |

## Port order

1. `lib/`: pure utils, schemas
2. `store/`: signals, query cache
3. `api/`: generated SDK + http wrapper
4. `components/ui/`: UI primitives (button, input, dialog, etc.)
5. `routes/` + router
6. `features/setup/` + `features/auth/`
7. `features/dashboard/`: simplest authed page
8. `features/projects/`: list, detail, kanban, settings
9. `features/my-tasks/` + `members/` + `notifications/`
10. `features/chat/`: heaviest (WS, rich text)
11. `features/voice/`: WebRTC, optional
