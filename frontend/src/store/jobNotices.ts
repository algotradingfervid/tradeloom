import type { JobProgress } from '../api/backend';
import { registerReset } from './reset';
import { pushToast } from './toasts';

/** Job ids the UI started itself (it shows their progress and errors in place); forgotten when the job finishes. */
const claimed = new Set<string>();
/** Calls that start a job and have not returned its id yet. */
let pending = 0;
/** Finished unknown jobs seen while a starting call was pending; they may belong to that call. */
let held: JobProgress[] = [];

// Ruling 85: exempt from the logout reset; only resetAllStores (tests) clears it.
registerReset(() => {
  claimed.clear();
  pending = 0;
  held = [];
}, { onLogout: false, inspect: () => ({ claimed, pending, held }) });

function show(p: JobProgress): void {
  pushToast({ level: 'error', message: p.error });
}

/** Claims a job id the UI started; its failure is not toasted. */
export function claimJob(jobId: string): void {
  const before = held.length;
  held = held.filter((p) => p.jobId !== jobId);
  if (held.length === before) claimed.add(jobId); // otherwise it already finished: nothing left to track
}

/**
 * Task 16 / Task 15 usage around a call that starts a job (Go runs one job at a time) — the done-flag pattern
 * (Ruling 86), so the window is closed exactly once whatever the call does:
 *   let done = false;
 *   beginJobClaim();
 *   try { const id = await backend.data.backupNow(folder, candles); done = true; endJobClaim(id); }
 *   finally { if (!done) endJobClaim(); }
 * A job that finishes before the id comes back is then still treated as the caller's.
 */
export function beginJobClaim(): void {
  pending += 1;
}

/**
 * Runs `start` (a call that returns a job id) inside a claim window with the done-flag pattern (Ruling 86): the
 * window is closed exactly once, whether the call resolves, is refused or its caller has gone away.
 */
export async function startClaimedJob(start: () => Promise<string>): Promise<string> {
  let done = false;
  beginJobClaim();
  try {
    const id = await start();
    done = true;
    endJobClaim(id);
    return id;
  } finally {
    if (!done) endJobClaim();
  }
}

/** Ends a beginJobClaim window; `jobId` is the started job (omit it when the call was refused). */
export function endJobClaim(jobId?: string): void {
  if (jobId !== undefined) claimJob(jobId);
  pending = Math.max(0, pending - 1);
  if (pending === 0) {
    const rest = held;
    held = [];
    rest.filter((p) => p.error).forEach(show);
  }
}

/**
 * A job the UI did not start — the scheduled backup (P0-3 Task 30) — reports a failure only on job.progress;
 * it shows as an error toast. The reason is shown as given: the backend redacts it (Ruling 74).
 */
export function notifyUnclaimedJob(p: JobProgress): void {
  if (!p.finished) return;
  // Diagnostics.CreateSupportBundle returns the path or the refusal to the UI call that started it (Go returns no
  // job id), and Go publishes job.progress on its own goroutine, so the event can land after the call returned.
  // The caller already reports the outcome (Task 15); toasting the event too would report a failure twice.
  if (p.kind === 'support_bundle') return;
  if (claimed.delete(p.jobId)) return;
  if (pending > 0) held.push(p);
  else if (p.error) show(p);
}
