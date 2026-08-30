# Pagination

## Overview

Breeze uses **cursor-based pagination** for its list endpoints, so results stay
consistent even when items are inserted or deleted while paging. Cursors are
opaque tokens returned by the previous response (or, for newest-first lists
like chat messages and task comments, a `before` parameter carrying the oldest
loaded item; see `./chat-system.md`).

## How It Works

## How It Works

### Request

Paginated endpoints accept these query parameters (the exact set and caps vary
slightly per endpoint; see the Swagger annotations):

| Param    | Type     | Default           | Description                                          |
| -------- | -------- | ----------------- | ---------------------------------------------------- |
| `cursor` | `string` | `""` (first page) | Opaque continuation token from the previous response |
| `search` | `string` | `""`              | Filter items by name (substring match)               |
| `limit`  | `int`    | `20`              | Items per page (endpoint-specific max, 50 or 100)    |

### Response

Every paginated endpoint returns this envelope:

```json
{
    "items": [{...}, {...}, ...],
    "next_cursor": "eyJ...",
    "has_more": true
}
```

| Field         | Type     | Description                                       |
| ------------- | -------- | ------------------------------------------------- |
| `items`       | `[]T`    | The page of results                               |
| `next_cursor` | `string` | Opaque token for the next page (empty if no more) |
| `has_more`    | `bool`   | Whether there are more results                    |

### Client Flow

```
1. Call endpoint without cursor → get first page
2. If has_more is true, pass next_cursor to get the next page
3. Repeat until has_more is false
4. On search change → reset cursor to "" (start fresh)
```

## Layer Architecture

Each layer has a specific role in the pagination pipeline:

### Domain (`internal/domain/user.go`)

Defines the filter and result types. These are pure structs, no HTTP
concerns:

```go
type UserFilter struct {
    Cursor          string // opaque, empty = first page
    Search          string
    Role            string // optional role filter
    IncludeInactive bool   // include deactivated accounts
    Limit           int    // page size
}

type UserListResult struct {
    Users      []*User
    NextCursor string // opaque, empty = no more
    HasMore    bool
}
```

The `Cursor`/`NextCursor` fields are opaque `string` values. The domain does
not know or care about their internal structure; they are just tokens.

### Port (`internal/port/repo.go`)

The repository interface accepts `domain.UserFilter` and returns
`*domain.UserListResult`:

```go
type UserRepository interface {
    ListUsers(ctx context.Context, orgID string, filter UserFilter) (*UserListResult, error)
    // ...
}
```

The service interface mirrors this exactly (pass-through pattern):

```go
type UserService interface {
    ListUsers(ctx context.Context, orgID string, filter domain.UserFilter) (*domain.UserListResult, error)
    // ...
}
```

### Store (`internal/store/user_store.go`)

The store is the **only layer** that knows about cursor encoding. It provides
private helpers:

```go
// encodeCursor("Alice", "abc123") → "eyJ"
func encodeCursor(name, id string) string

// decodeCursor("eyJ") → ("Alice", "abc123", nil)
func decodeCursor(cursor string) (name, id string, err error)
```

**SQL query** uses the `@name` sqlc pattern for cursor parameters:

```sql
-- name: ListUsersPaginated :many
SELECT id, account_id, org_id, email, name, role, avatar_url, is_active, created_at, updated_at
FROM users
WHERE org_id = @org_id
  AND (@search IS NULL OR @search = '' OR instr(lower(name), lower(@search)) > 0)
  AND (@role IS NULL OR @role = '' OR role = @role)
  AND (@include_inactive = 1 OR is_active = 1)
  AND (
    (@cursor_name = '' AND @cursor_id = '')
    OR (name > @cursor_name)
    OR (name = @cursor_name AND id > @cursor_id)
  )
ORDER BY name ASC, id ASC
LIMIT @limit_val;
```

The store uses a **+1 overflow** technique to detect `has_more`:

1. Query with `limit = filter.Limit + 1`
2. If `len(rows) > filter.Limit`, `HasMore = true`; pop the last row and encode
   the next cursor from its sort values
3. Otherwise `HasMore = false`

### Service (`internal/service/user.go`)

Simple pass-through, no pagination logic:

```go
func (s *UserService) ListUsers(ctx context.Context, orgID string, filter domain.UserFilter) (*domain.UserListResult, error) {
    return s.userRepo.ListUsers(ctx, orgID, filter)
}
```

### Transport (`internal/transport/handler/user.go`)

The handler:

1. Parses query params into `domain.UserFilter`
2. Calls service
3. Converts domain result into DTO

```go
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
    orgID, _ := transport.OrgIDFromContext(r.Context())

    filter := domain.UserFilter{
        Cursor: r.URL.Query().Get("cursor"),
        Search: r.URL.Query().Get("search"),
        Role:   r.URL.Query().Get("role"),
        Limit:  20,
    }
    if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 100 {
        filter.Limit = l
    }
    if r.URL.Query().Get("include_inactive") == "true" {
        filter.IncludeInactive = true
    }

    result, err := h.svc.ListUsers(r.Context(), orgID, filter)
    if err != nil {
        transport.ServerError(w, r, h.log, err)
        return
    }

    resp := dto.PaginatedUsersResponse{
        Items:      make([]*dto.UserResponse, len(result.Users)),
        NextCursor: result.NextCursor,
        HasMore:    result.HasMore,
    }
    for i, u := range result.Users {
        resp.Items[i] = dto.NewUserResponse(u)
    }
    transport.JSON(w, r, http.StatusOK, resp)
}
```

### Swagger Annotation

```go
// @Success 200 {object} dto.PaginatedUsersResponse
```

The DTO:

```go
type PaginatedUsersResponse struct {
    Items      []*UserResponse `json:"items"`
    NextCursor string          `json:"next_cursor,omitempty"`
    HasMore    bool            `json:"has_more"`
}
```

### Frontend

Lit components call the paginated API and accumulate pages in component state.
The cursor comes straight from the response's `next_cursor` field. The canonical
example is `ui/src/features/members/members-page.ts` (see also
`../ui/infinite-scroll.md`), which uses a **"Load more" button** below the
list:

```ts
@customElement("breeze-members-page")
export class BreezeMembersPage extends LitElement {
  @state() private _cursor: string | undefined = undefined;
  @state() private _hasMore = false;
  @state() private _loadingMore = false;
  @state() private _members: User[] = [];

  async #loadMembers(append = false) {
    try {
      const result = await membersApi.list({
        cursor: append ? this._cursor : undefined,
        search: this._search || undefined,
        limit: 50,
      });
      if (append) {
        this._members = [...this._members, ...(result.items ?? [])];
      } else {
        this._members = result.items ?? [];
      }
      this._hasMore = result.has_more ?? false;
      this._cursor = result.next_cursor || undefined;
    } catch {
      this._hasMore = false;
    }
  }

  // In the template, when _hasMore is true:
  //   <breeze-button @click="${() => this.#loadMembers(true)}">Load more</breeze-button>
}
```

When the user clicks "Load more", the next page is fetched with the current
cursor and appended. The button stays visible while `has_more` is true. Some
lists (chat messages) instead use an `IntersectionObserver` sentinel for
infinite scroll; see `../ui/infinite-scroll.md` for both patterns.

## When to Use Pagination

Every list endpoint should use cursor-based pagination. The current set of
paginated endpoints:

- `GET /api/users`: paginated
- `GET /api/projects/{id}/members`: paginated
- `GET /api/projects/{id}/tasks`: paginated (task boards can have hundreds)
- `GET /api/tasks` (My Issues): paginated (limit default 20, max 50)
- `GET /api/notifications`: paginated
- `GET /api/conversations`: paginated (cursor for conversation list)
- `GET /api/conversations/{id}/messages` and `.../replies`: paginated via
  `before` (descending, newest first)
- `GET /api/comments` (task comments): paginated via `before`

Non-paginated list endpoints (small, bounded sets):

- `GET /api/projects`: returns the full org project list (add pagination if
  projects grow)
- `GET /api/projects/{id}/statuses`: statuses are few, pagination optional
- `GET /api/projects/{id}/cycles`: cycles are few, pagination optional

## Adding Pagination to a New Endpoint

1. Add `Filter` and `Result` types to the domain package
2. Update the port repository interface
3. Add the SQL query with cursor params (use `@cursor_name`, `@cursor_id`
   pattern)
4. Implement the store method with cursor encode/decode
5. Implement the service passthrough
6. Update the handler to parse query params and build the paginated DTO
7. Update Swagger annotations
8. Run `make swagger-gen && make api-types`
