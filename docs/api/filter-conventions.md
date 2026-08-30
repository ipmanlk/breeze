# Filter Conventions

This doc documents how filtering works across the stack and the conventions that prevent bugs.

## Architecture

Filters flow through three layers: **URL → Handler → SQL**.

```
Browser URL                    Handler                      SQL
───────────                    ───────                      ───
?search=foo&                    r.URL.Query().Get("q")      title LIKE '%foo%'
&status_id=<uuid>               handler → *string       *status_id IS NULL OR t.status_id = *status_id
&assignee_id=<uuid>             handler → *string       *assignee_id IS NULL OR EXISTS (...)
&cycle_id=<uuid>                handler → *string       (@cycle_id IS NULL OR ... __backlog__ ... OR = )
&priority=urgent                r.URL.Query().Get()         = '' sentinel
```

## URL Key Naming

| URL Key        | Backend Param | ViewFilters Key | Purpose                    |
| -------------- | ------------- | --------------- | -------------------------- |
| `search`       | `q`           | `search`        | Text search across titles  |
| `status_id`    | `status_id`   | `status_id`     | Filter by task status      |
| `assignee_id`  | `assignee_id` | `assignee_id`   | Filter by assignee         |
| `cycle_id`     | `cycle_id`    | `cycle_id`      | Filter by cycle/backlog    |
| `priority`     | `priority`    | `priority`      | Filter by priority level   |

Frontend URL keys are user-facing (`search`). The project detail page
(`ui/src/features/projects/project-detail-page.ts`) maps them to backend param
names (`search` → `q`) when calling the API. `urlToViewFilters()` reads URL
params back into a `ViewFilters` object (see [Saved Views](#saved-views)).

## Optional ID Filters (`*string`)

All optional record-ID filter fields use `*string` (pointer to string) in the domain model:

```go
type TaskFilter struct {
    StatusID   *string
    AssigneeID *string
    CycleID    *string
    Priority   string   // not an ID; uses empty-string sentinel
    Search     string   // not an ID; uses empty-string sentinel
}
```

- `nil` = no filter (return all)
- non-nil = filter by that value
- Never use empty string `""` as a sentinel for "no filter" on ID fields. This mixes up "no filter" with "filter by empty string".

### Why not just `string`?

The old code used `string` with an `= ''` sentinel in SQL:

```sql
-- OLD: fragile
AND (status_id = @status_id OR @status_id = '')
```

This works only if the parameter is always `""` for "no filter". If a helper converts `""` to `nil` (like `nilIfEmpty` in `internal/store/helpers.go`), the SQL condition becomes `NULL = ''` which is `UNKNOWN`, silently excluding all rows.

```sql
-- NEW: robust
AND (@status_id IS NULL OR status_id = @status_id)
```

This handles both `nil` and non-nil correctly, regardless of how the caller converts empty strings.

## Sentinel Constants

Defined in `internal/domain/filter.go`:

```go
const (
    CycleAll     = "__all__"
    CycleBacklog = "__backlog__"
)
```

- `__backlog__` is understood by both frontend and backend SQL as "tasks with no cycle"
- `__all__` is frontend-only; the handler converts it to `nil` before passing to SQL

## SQL Patterns

### Optional ID filter (preferred)
```sql
AND (@param IS NULL OR column = @param)
```

### Optional ID filter with sentinel value
```sql
AND (
    @param IS NULL
    OR (@param = '__backlog__' AND column IS NULL)
    OR (column = @param)
)
```

### Optional string/priority filter
```sql
AND (column = @param OR @param = '')
```

### Optional search
```sql
AND (@search IS NULL OR @search = '' OR column LIKE '%' || @search || '%')
```

## Handler Pattern

```go
var param *string
if v := r.URL.Query().Get("param"); v != "" {
    param = &v
}
// pass param to filter struct; nil means "no filter"
```

Always handle empty query params by leaving the pointer as `nil`. Never pass `""`.

## Saved Views

- `ViewFilters` keys match the URL keys (`search`, `status_id`, `assignee_id`,
  `cycle_id`, `priority`, `label_ids`); see `ui/src/features/views/types.ts`.
- Saved view filters are stored as JSON in the database.
- The URL is the source of truth for the currently applied filters:
  - `urlToViewFilters()` in `project-detail-page.ts` builds a `ViewFilters`
    object from the current URL search params.
  - The save-view dialog (`save-view-dialog.ts`, a Lit element) receives that
    `filters` object as a property and posts it to the views API. There is no
    separate URL-reading step; the parent page owns the URL-to-filters mapping.
  - Loading a view pushes its filters into the URL (same keys), so the page's
    `urlToViewFilters()` reads them back on load.

## Common Pitfalls

1. **`nilIfEmpty("")` returns `nil`**: The store helper `nilIfEmpty` converts empty strings to `nil`. Always pair it with SQL that checks `IS NULL`, not `= ''`. The split is handled at the handler level (each handler builds its own `*string` and leaves it `nil` for "no filter"); `nilIfEmpty` is used when binding optional sqlc params to nullable columns.
3. **URL key ≠ backend param**: The browser URL key (`search`) can differ from the API param (`q`). The project page maps URL keys to API params when calling the SDK (`search` → `q`). When saving views, read from the URL keys.
