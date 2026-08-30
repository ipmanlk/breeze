import { msg } from "@lit/localize";

export interface NavItem {
  title: string;
  url: string;
  icon: string;
  badge?: string | number;
}

/**
 * Primary navigation items with localized titles. Returns fresh `msg()`
 * results each time it's called, so the nav re-renders correctly when the
 * locale changes. Call this at render time (not module scope).
 */
export function getPrimaryNav(): NavItem[] {
  return [
    { title: msg("Home"), url: "/", icon: "house" },
    { title: msg("Inbox"), url: "/inbox", icon: "inbox" },
    { title: msg("My Tasks"), url: "/my-tasks", icon: "list-checks" },
    { title: msg("Projects"), url: "/projects", icon: "folder" },
    { title: msg("Views"), url: "/views", icon: "chart-bar" },
    { title: msg("Chat"), url: "/chat", icon: "message-square-text" },
    { title: msg("Messages"), url: "/messages", icon: "mail" },
    { title: msg("Members"), url: "/members", icon: "users" },
  ];
}
