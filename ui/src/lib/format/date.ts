const MONTHS = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "May",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Oct",
  "Nov",
  "Dec",
];

function toDate(date: Date | string): Date {
  return typeof date === "string" ? new Date(date) : date;
}

/** Short date: "Jul 18" (month abbreviation + day). */
export function fmtDate(date: Date | string): string {
  const d = toDate(date);
  return `${MONTHS[d.getMonth()]} ${d.getDate()}`;
}

/** Date with year: "Jul 18, 2026" (month abbreviation + day + year). */
export function fmtDateYear(date: Date | string): string {
  const d = toDate(date);
  return `${MONTHS[d.getMonth()]} ${d.getDate()}, ${d.getFullYear()}`;
}

/** Month + year: "Jul 2026" (month abbreviation + year). */
export function fmtMonth(date: Date | string): string {
  const d = toDate(date);
  return `${MONTHS[d.getMonth()]} ${d.getFullYear()}`;
}
