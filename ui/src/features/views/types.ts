/**
 * Views feature types: saved filtered views of tasks.
 */

export type ViewLayout = "board" | "list";

export interface ViewFilters {
  search?: string;
  priority?: string;
  status_id?: string;
  assignee_id?: string;
  cycle_id?: string;
  label_ids?: string[];
}

export interface View {
  id: string;
  project_id?: string;
  project_slug?: string;
  project_name?: string;
  created_by: string;
  name: string;
  layout: ViewLayout;
  filters: ViewFilters;
  created_at: string;
  updated_at: string;
}

export const PRIORITY_LABELS: Record<string, string> = {
  urgent: "Urgent",
  high: "High",
  medium: "Medium",
  low: "Low",
  none: "None",
};

export function humanizeKey(key: string): string {
  const map: Record<string, string> = {
    priority: "Priority",
    status_id: "Status",
    assignee_id: "Assignee",
    cycle_id: "Cycle",
    search: "Search",
    label_ids: "Labels",
  };
  return map[key] ?? key.replace(/_/g, " ");
}

export function humanizeValue(key: string, value: string): string {
  if (key === "priority") return PRIORITY_LABELS[value] ?? value;
  if (key === "cycle_id") {
    if (value === "__backlog__") return "Backlog (no cycle)";
    if (value === "__all__") return "All tasks";
    return value;
  }
  return value;
}

export function activeFilterEntries(filters: ViewFilters): [string, string][] {
  const entries: [string, string][] = [];
  if (filters.search) entries.push(["search", filters.search]);
  if (filters.priority) entries.push(["priority", filters.priority]);
  if (filters.status_id) entries.push(["status_id", filters.status_id]);
  if (filters.assignee_id) entries.push(["assignee_id", filters.assignee_id]);
  if (filters.cycle_id) entries.push(["cycle_id", filters.cycle_id]);
  if (filters.label_ids && filters.label_ids.length > 0) {
    entries.push(["label_ids", `${filters.label_ids.length} label(s)`]);
  }
  return entries;
}
