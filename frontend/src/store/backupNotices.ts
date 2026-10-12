import { create } from 'zustand';
import type { Backend } from '../api/backend';
import { dataCopy } from '../i18n/copy';
import { startClaimedJob } from './jobNotices';
import { useJobsStore } from './jobs';
import { registerReset } from './reset';
import { pushToast } from './toasts';

/**
 * Manual backups ("Back up now") report their result from here, not from the Data group, so the success or failure
 * toast survives the user leaving Settings → Data (Ruling 106). The job id is claimed (jobNotices does not toast it)
 * and tracked here until its finished job.progress arrives; each outcome is toasted exactly once.
 */
interface BackupNoticesStore { tracked: string[] }

export const useBackupNoticesStore = create<BackupNoticesStore>()(() => ({ tracked: [] }));

// Not account data: a backup started before a logout still reports its result (Ruling 85).
registerReset(() => useBackupNoticesStore.setState({ tracked: [] }), { onLogout: false, inspect: () => useBackupNoticesStore.getState() });

/** Toasts the result of a tracked job once it has finished; true when it did. */
function settle(jobId: string): boolean {
  const p = useJobsStore.getState().jobs[jobId];
  if (!p?.finished) return false;
  useBackupNoticesStore.setState((s) => ({ tracked: s.tracked.filter((id) => id !== jobId) }));
  if (p.error) pushToast({ level: 'error', message: dataCopy.backupFailed(p.error) });
  else pushToast({ level: 'success', message: dataCopy.backupSaved(p.resultPath) });
  return true;
}

useJobsStore.subscribe((s, prev) => {
  for (const id of useBackupNoticesStore.getState().tracked) if (s.jobs[id] !== prev.jobs[id]) settle(id);
});

/**
 * Starts a manual backup. The claim window and the tracking live here, outside any component, so nothing is left
 * claimed without someone to report it when the caller unmounts mid-call. Rejects with the backend's refusal.
 */
export async function startManualBackup(backend: Backend, folder: string, includeCandles: boolean): Promise<string> {
  const id = await startClaimedJob(() => backend.data.backupNow(folder, includeCandles));
  useBackupNoticesStore.setState((s) => ({ tracked: [...s.tracked, id] }));
  settle(id); // it may have finished before its id came back
  return id;
}
