/**
 * Locale-aware date / time / number formatting.
 *
 * Replaces the hardcoded English in `lib/format/date.ts` (manual `MONTHS`
 * array) and `lib/format/time-ago.ts` ("just now", "X minutes ago") with
 * native `Intl.*` APIs that respect the active locale.
 *
 * The active locale is read via `getLocale()` from `@/i18n`, so every call site
 * automatically renders in the user's selected language with no locale arg to
 * pass around. `Intl.*` formatters are cached per-locale for performance.
 *
 * These functions intentionally keep the same call signatures as the
 * `lib/format/*` helpers they replace (a `Date | string` in, a string out) so
 * migration is a 1-line import swap. See PLAN.md §4.4.
 */
import { getLocale } from "./index.ts";

type DateTimeFormatter = (d: Date) => string;

/** Cache `Intl.DateTimeFormat` instances per (locale, options) key. */
const dateTimeCache = new Map<string, Intl.DateTimeFormat>();
function dateTimeFormatter(
  locale: string,
  options: Intl.DateTimeFormatOptions,
): DateTimeFormatter {
  const key = `${locale}|${JSON.stringify(options)}`;
  let fmt = dateTimeCache.get(key);
  if (!fmt) {
    fmt = new Intl.DateTimeFormat(locale, options);
    dateTimeCache.set(key, fmt);
  }
  return (d: Date) => fmt!.format(d);
}

function toDate(date: Date | string): Date {
  return typeof date === "string" ? new Date(date) : date;
}

/** Short date: "Jul 18" / "18 juil." (month abbreviation + day). */
export function fmtDate(date: Date | string): string {
  const d = toDate(date);
  return dateTimeFormatter(getLocale(), { month: "short", day: "numeric" })(d);
}

/** Date with year: "Jul 18, 2026" / "18 juil. 2026". */
export function fmtDateYear(date: Date | string): string {
  const d = toDate(date);
  return dateTimeFormatter(getLocale(), {
    month: "short",
    day: "numeric",
    year: "numeric",
  })(d);
}

/** Month + year: "Jul 2026" / "juil. 2026". */
export function fmtMonth(date: Date | string): string {
  const d = toDate(date);
  return dateTimeFormatter(getLocale(), { month: "short", year: "numeric" })(d);
}

/** Full date + time: "Jul 18, 2026, 2:30 PM" / "18 juil. 2026 à 14:30". */
export function fmtDateTime(date: Date | string): string {
  const d = toDate(date);
  return dateTimeFormatter(getLocale(), {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  })(d);
}

/**
 * Long-format relative time using `Intl.RelativeTimeFormat`:
 * "just now", "5 minutes ago" / "il y a 5 minutes".
 *
 * `Intl.RelativeTimeFormat` does not produce "just now" for the <60s case
 * (it would say "0 seconds ago"), so we special-case it. The "just now"
 * string is localized via `msg()` so it translates too.
 */
import { msg } from "@lit/localize";

const relativeTimeCache = new Map<string, Intl.RelativeTimeFormat>();
function relativeTimeFormatter(locale: string): Intl.RelativeTimeFormat {
  let fmt = relativeTimeCache.get(locale);
  if (!fmt) {
    fmt = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
    relativeTimeCache.set(locale, fmt);
  }
  return fmt;
}

const SECONDS_MINUTE = 60;
const SECONDS_HOUR = 3600;
const SECONDS_DAY = 86400;
const SECONDS_MONTH = 2592000; // 30 days
const SECONDS_YEAR = 31536000; // 365 days

/** "just now", "5 minutes ago", "3 hours ago", "2 days ago", "1 month ago", "1 year ago". */
export function timeAgo(date: Date | string): string {
  const d = toDate(date);
  const seconds = Math.floor((Date.now() - d.getTime()) / 1000);
  if (seconds < SECONDS_MINUTE) return msg("just now");
  const fmt = relativeTimeFormatter(getLocale());
  if (seconds < SECONDS_HOUR) {
    return fmt.format(-Math.floor(seconds / SECONDS_MINUTE), "minute");
  }
  if (seconds < SECONDS_DAY) {
    return fmt.format(-Math.floor(seconds / SECONDS_HOUR), "hour");
  }
  if (seconds < SECONDS_MONTH) {
    return fmt.format(-Math.floor(seconds / SECONDS_DAY), "day");
  }
  if (seconds < SECONDS_YEAR) {
    return fmt.format(-Math.floor(seconds / SECONDS_MONTH), "month");
  }
  return fmt.format(-Math.floor(seconds / SECONDS_YEAR), "year");
}

/**
 * Compact relative time: "just now", "5m ago", "3h ago", "2d ago",
 * "1mo ago", "1y ago". Returns "" for invalid dates.
 *
 * When `dateFallbackDays` is provided, dates older than that many days
 * are formatted via `fmtDateYear()` (absolute) instead of a relative label.
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
  if (seconds < SECONDS_MINUTE) return msg("just now");
  const fmt = relativeTimeFormatter(getLocale());
  if (seconds < SECONDS_HOUR) {
    return fmt.format(-Math.floor(seconds / SECONDS_MINUTE), "minute");
  }
  if (seconds < SECONDS_DAY) {
    return fmt.format(-Math.floor(seconds / SECONDS_HOUR), "hour");
  }
  const days = Math.floor(seconds / SECONDS_DAY);
  if (dateFallbackDays && days >= dateFallbackDays) {
    return fmtDateYear(d);
  }
  if (seconds < SECONDS_MONTH) {
    return fmt.format(-days, "day");
  }
  if (seconds < SECONDS_YEAR) {
    return fmt.format(-Math.floor(seconds / SECONDS_MONTH), "month");
  }
  return fmt.format(-Math.floor(seconds / SECONDS_YEAR), "year");
}

/**
 * Format a number per locale (grouping separators, decimal point).
 *   en: 1,234.5  ·  fr: 1 234,5
 */
export function fmtNumber(
  value: number,
  options?: Intl.NumberFormatOptions,
): string {
  return new Intl.NumberFormat(getLocale(), options).format(value);
}

/** Format a byte size with units (e.g. "1.5 MB"). Locale-aware unit + decimal. */
export function fmtBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${
    fmtNumber(bytes / Math.pow(1024, i), {
      maximumFractionDigits: 1,
    })
  } ${units[i]}`;
}
