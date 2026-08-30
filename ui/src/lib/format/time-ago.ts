const SECONDS_MINUTE = 60;
const SECONDS_HOUR = 3600;
const SECONDS_DAY = 86400;
const SECONDS_MONTH = 2592000; // 30 days
const SECONDS_YEAR = 31536000; // 365 days

function toDate(date: Date | string): Date {
  return typeof date === "string" ? new Date(date) : date;
}

/**
 * Long-format relative time: "just now", "5 minutes ago", "3 hours ago",
 * "2 days ago", "1 month ago", "1 year ago".
 */
export function timeAgo(
  date: Date | string,
): string {
  const d = toDate(date);
  const seconds = Math.floor((Date.now() - d.getTime()) / 1000);
  if (seconds < SECONDS_MINUTE) return "just now";
  const intervals: [number, string][] = [
    [SECONDS_YEAR, "year"],
    [SECONDS_MONTH, "month"],
    [SECONDS_DAY, "day"],
    [SECONDS_HOUR, "hour"],
    [SECONDS_MINUTE, "minute"],
  ];
  for (const [secs, label] of intervals) {
    const count = Math.floor(seconds / secs);
    if (count > 0) return `${count} ${label}${count > 1 ? "s" : ""} ago`;
  }
  return "just now";
}

/**
 * Compact relative time: "just now", "5m ago", "3h ago", "2d ago",
 * "1mo ago", "1y ago". Returns "" for invalid dates.
 *
 * When `dateFallbackDays` is provided, dates older than that many days
 * are formatted via `toLocaleDateString()` instead of a relative label.
 * This matches the dashboard/audit-log pattern of showing absolute dates
 * for items older than a threshold.
 */
export function timeAgoShort(
  date: Date | string | undefined,
  dateFallbackDays?: number,
): string {
  if (date === undefined) return "";
  const d = toDate(date);
  if (Number.isNaN(d.getTime())) return "";
  const seconds = Math.max(0, Math.floor((Date.now() - d.getTime()) / 1000));
  if (seconds < SECONDS_MINUTE) return "just now";
  const mins = Math.floor(seconds / SECONDS_MINUTE);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  if (dateFallbackDays && days >= dateFallbackDays) {
    return d.toLocaleDateString();
  }
  if (days < 30) return `${days}d ago`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${months}mo ago`;
  return `${Math.floor(months / 12)}y ago`;
}
