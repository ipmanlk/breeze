# Issues

"My Issues" is a cross-project, user-centric view of all tasks assigned to you
across every project in the workspace. An "issue" is a task viewed from the
assignee's perspective rather than from within a specific project.

## Terminology

| Term      | Context                                       |
| --------- | --------------------------------------------- |
| **Task**  | Within a project (Board, List, Timeline)      |
| **Issue** | Cross-project, assignee-centric ("My Issues") |

A task becomes an "issue" when viewed outside its project context. The same
entity is a "task" inside a project and an "issue" in the cross-project view.

## Data Model

Issues are `Task` rows enriched with project metadata:

```
EnrichedTask (domain.Task + ProjectName, ProjectSlug, ProjectColor, StatusName, StatusColor)
```

The backend query (`ListByUser`) joins `tasks` with `projects` and
`task_statuses` to produce enriched results. This avoids N+1 queries on the
frontend.

## API

| Method | Path          | Description                                         |
| ------ | ------------- | -------------------------------------------------- |
| GET    | `/api/tasks`  | List issues (tasks) assigned to the current user   |

> **Note on naming:** the route is `/api/tasks` because the underlying entity
> is a `Task` and the handler is `taskHandler.ListTasks`. The *feature* and UI
> concept is "My Issues" (cross-project, assignee-centric), matching the
> `Task` rows returned. There is no `/api/issues` route; a previous version of
> this doc documented one, which was incorrect.

Query parameters:

| Parameter        | Type   | Description                                                    |
| ---------------- | ------ | -------------------------------------------------------------- |
| `q`              | string | Search in title                                                |
| `priority`       | string | Filter by priority (`urgent`, `high`, `medium`, `low`, `none`) |
| `status_id`      | string | Filter by status UUID                                          |
| `group_by`       | string | Group results (`status`, `priority`, `project`)                |
| `show_completed` | bool   | Include completed tasks                                        |
| `limit`          | int    | Max results (default 20, max 50)                               |

## Features

- **Filters**: Search, priority, status, completed toggle
- **Grouping**: Group by status (shows status name), priority
  (Urgent/High/Medium/Low/No priority), or project (shows project name)
- **Sorting**: Sort by priority, due date, title, project, or last updated
- **Navigation**: Clicking an issue navigates to `/projects/{slug}?task={id}`
  (the project detail page with the task dialog open)

## Design Decisions

1. **Slug-based URLs**: Project links always use the project slug
   (`/projects/backend-api-platform?task=abc`), never the raw UUID. This ensures
   notifications, issue links, and grouped items all resolve correctly. A
   frontend fallback in `ui/src/features/projects/task-detail-dialog.ts`
   (opened via the `?task=` param handled in `project-detail-page.ts`) handles
   legacy UUID-based links.

2. **Group by labels are humanized**: Status groups show the status name (e.g.
   "In Progress"), priority groups show "Urgent"/"High"/etc, and project groups
   show the project name. Never raw IDs.

3. **"All priorities" is in the dropdown**: The priority filter dropdown
   includes an "All priorities" option as the first selectable item, following
   standard select UX patterns.
