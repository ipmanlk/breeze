# Layouts

## Location

`src/layouts/` contains composite layout elements that assemble navigation + content
areas.

## Current layouts

| File              | Element                 | Use case                |
| ----------------- | ----------------------- | ----------------------- |
| `app-layout.ts`   | `<plume-app-layout>`   | Authenticated app shell |
| `guest-layout.ts` | `<plume-guest-layout>` | Public/unauthenticated  |

## Pattern

Layouts use named `<slot>` elements for the router to project content into:

```html
<plume-app-layout>
  <plume-sidebar slot="nav"></plume-sidebar>
  <div slot="content">
    <!-- routed page -->
  </div>
</plume-app-layout>
```

## Rules

- Layouts assemble, they don't own state.
- No business logic, purely structural.
- Named slots allow the router to swap page content without re-rendering chrome.

## `data-fullscreen` attribute

`<plume-app-layout>` supports a `data-fullscreen` attribute for pages that need
a fixed viewport height (chat, kanban). When present, `.main` uses
`height: 100svh; min-height: 0` instead of the default `min-height: 100svh`,
preventing flex content from growing the container and pushing elements (like
chat input) off-screen.

Set it on the layout element: `<plume-app-layout data-fullscreen>`. Pages
without this attribute use the default scroll-until-full behavior.
