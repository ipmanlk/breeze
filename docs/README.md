# Breeze Documentation

Breeze is a single Go binary with an embedded Lit/Vite SPA. The docs are
split into three sections so you can find the part you care about without
wading through the rest:

| Section          | Covers                                    | Start here                    |
| ---------------- | ----------------------------------------- | ----------------------------- |
| [`api/`](api/)   | Everything that runs in the Go binary     | [api/architecture.md](api/architecture.md) |
| [`ui/`](ui/)     | The frontend SPA (`ui/src`)               | [ui/overview.md](ui/overview.md) |
| [`ops/`](ops/)   | Deployment, configuration, self-hosting   | [ops/configuration.md](ops/configuration.md) |
| `i18n/`          | Internationalization (spans UI + backend) | [i18n.md](i18n.md)            |

`ROADMAP.md` (next to this file) lists planned directions that are not yet
built.

## Quick picks

| I want to…                              | Read                                    |
| --------------------------------------- | --------------------------------------- |
| understand the server architecture      | [api/architecture.md](api/architecture.md) |
| add or change an HTTP endpoint          | [api/architecture.md](api/architecture.md) + [api/build-commands.md](api/build-commands.md) |
| make a frontend change                  | [ui/overview.md](ui/overview.md) (then the doc matching the area) |
| run/deploy Breeze                       | [ops/setup.md](ops/setup.md) + [ops/configuration.md](ops/configuration.md) |
| add a language locale                   | [i18n.md](i18n.md) → [i18n/adding-a-language.md](i18n/adding-a-language.md) |

## Contributing to the docs

Frontend rules for *writing code* live in `../ui/AGENTS.md` (the authoritative
frontend rules). This directory is for *documentation*; keep each doc focused
on one topic and update links when you move or rename a page.