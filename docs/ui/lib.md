# Lib (pure utilities)

## Location

`src/lib/` contains framework-agnostic utilities. No Lit imports, no DOM access. Bottom
of the dependency graph.

## Modules

| Path                  | Description                          |
| --------------------- | ------------------------------------ |
| `lexorank.ts`         | Lexorank key generation for task ordering |
| `schemas/`            | valibot validation schemas           |
| `format/`             | Date/number/relative-time formatters (`Intl.*`) |
| `icons.ts`            | Icon registry + `<plume-icon>` data |
| `lucide-icons.json`   | Raw Lucide icon definitions          |
| `markdown.ts`         | Markdown → HTML rendering            |
| `sanitize.ts`         | HTML sanitizer (rich text output)    |
| `mentions.ts`         | Mention parsing / insertion helpers  |
| `shortcuts.ts`        | Keyboard shortcut registry           |
| `permissions.ts`      | Frontend permission helpers (org role / project role) |
| `signal-controller.ts`| `SignalController` reactive controller bridging signals to Lit |
| `outside-click-controller.ts` | Outside-click detection for popovers/dialogs |
| `sdk-helpers.ts`      | Small helpers around the generated SDK |
| `dnd-gap.ts`          | Drag-and-drop gap helpers            |
| `async.ts`            | Async utilities (debounce, etc.)     |
| `log.ts`              | Logger helper                        |

Note: `signal-controller.ts` and `outside-click-controller.ts` import Lit, so
`lib/` is not 100% framework-free in practice. Pure utilities (formatters,
sanitizers, schemas) stay at the very bottom of the dependency graph; the
controllers only import `lit` + signals, never higher layers.
