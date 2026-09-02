/** Formatting helpers matching the reference's copy (PRD §5). */

const MONTHS = [
  "Jan", "Feb", "Mar", "Apr", "May", "Jun",
  "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
];

/** parseTime turns a Go RFC 3339 timestamp into a Date, or null if unset. */
export function parseTime(value: string | undefined): Date | null {
  if (!value) return null;
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return null;
  // Go marshals the zero time as year 1; treat that as "never".
  if (d.getUTCFullYear() < 1971) return null;
  return d;
}

/** formatDate renders "12 Aug 2026", the reference's date style. */
export function formatDate(value: string | undefined): string {
  const d = parseTime(value);
  if (!d) return "—";
  return `${String(d.getDate()).padStart(2, "0")} ${MONTHS[d.getMonth()]} ${d.getFullYear()}`;
}

/** formatRelative renders "4m ago", "2h ago", "1d ago". */
export function formatRelative(value: string | undefined, now = Date.now()): string {
  const d = parseTime(value);
  if (!d) return "never";
  const seconds = Math.max(0, Math.round((now - d.getTime()) / 1000));
  if (seconds < 45) return "just now";
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.round(hours / 24);
  if (days < 30) return `${days}d ago`;
  return formatDate(value);
}

/** formatBytes renders "6.2 MB". Sizes are decimal, as GitHub reports them. */
export function formatBytes(bytes: number | undefined): string {
  if (!bytes || bytes <= 0) return "—";
  const units = ["B", "KB", "MB", "GB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1000 && unit < units.length - 1) {
    value /= 1000;
    unit += 1;
  }
  const decimals = unit === 0 || value >= 100 ? 0 : 1;
  return `${value.toFixed(decimals)} ${units[unit]}`;
}

/** formatClock renders a log timestamp as "10:04:02". */
export function formatClock(value: string | undefined): string {
  const d = parseTime(value);
  if (!d) return "--:--:--";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

/** formatUptime renders "00:04:11" for the Running footer. */
export function formatUptime(startedAt: string, endedAt: string, now = Date.now()): string {
  const start = parseTime(startedAt);
  if (!start) return "00:00:00";
  const end = parseTime(endedAt)?.getTime() ?? now;
  const total = Math.max(0, Math.floor((end - start.getTime()) / 1000));
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(Math.floor(total / 3600))}:${pad(Math.floor((total % 3600) / 60))}:${pad(total % 60)}`;
}

/** plural renders "1 repo" / "3 repos". */
export function plural(count: number, singular: string, pluralForm?: string): string {
  return `${count} ${count === 1 ? singular : (pluralForm ?? `${singular}s`)}`;
}

/** errorMessage normalises whatever the bridge rejects with into a sentence. */
export function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  if (typeof err === "string") return err;
  if (err && typeof err === "object" && "message" in err) {
    return String((err as { message: unknown }).message);
  }
  return "Something went wrong.";
}
