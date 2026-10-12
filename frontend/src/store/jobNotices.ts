import type { JobProgress } from '../api/backend';
import { registerReset } from './reset';
import { pushToast } from './toasts';

/** Job ids the UI started itself (it shows their progress and errors in place). */
const claimed = new Set<string>();

registerReset(() => claimed.clear());

/** Callers that start a job (Data.BackupNow / Restore, Diagnostics.CreateSupportBundle) claim its id. */
export function claimJob(jobId: string): void {
  claimed.add(jobId);
}

/**
 * A job the UI did not start — the scheduled backup (P0-3 Task 30) — reports a failure only on job.progress;
 * it shows as an error toast. The reason is shown as given: the backend redacts it (Ruling 74).
 */
export function notifyUnclaimedJob(p: JobProgress): void {
  if (!p.finished || !p.error || claimed.has(p.jobId)) return;
  pushToast({ level: 'error', message: p.error });
}
