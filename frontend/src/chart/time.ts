import { TickMarkType, type UTCTimestamp } from 'lightweight-charts';

/**
 * lightweight-charts renders UTCTimestamp in UTC. IST has no DST, so we feed it IST wall-clock seconds
 * (epoch + 19 800 s). The axis, day/month boundaries and labels are then IST whatever the OS zone is.
 */
export const IST_OFFSET_SECONDS = 19_800;
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const pad = (n: number) => String(n).padStart(2, '0');

export function toChartTime(ms: number): UTCTimestamp {
  return (Math.floor(ms / 1000) + IST_OFFSET_SECONDS) as UTCTimestamp;
}

export function fromChartTime(t: UTCTimestamp | number): number {
  return ((t as number) - IST_OFFSET_SECONDS) * 1000;
}

const wall = (t: number) => new Date((t as number) * 1000);

export function formatCrosshairTime(t: UTCTimestamp | number, intraday: boolean): string {
  const d = wall(t as number);
  const day = `${pad(d.getUTCDate())} ${MONTHS[d.getUTCMonth()]} '${pad(d.getUTCFullYear() % 100)}`;
  return intraday ? `${day} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}` : day;
}

export function formatTickMark(t: UTCTimestamp | number, type: TickMarkType): string {
  const d = wall(t as number);
  switch (type) {
    case TickMarkType.Year: return String(d.getUTCFullYear());
    case TickMarkType.Month: return MONTHS[d.getUTCMonth()];
    case TickMarkType.DayOfMonth: return pad(d.getUTCDate());
    case TickMarkType.Time: return `${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}`;
    case TickMarkType.TimeWithSeconds: return `${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}:${pad(d.getUTCSeconds())}`;
  }
}
