import { create } from 'zustand';
import type { JobProgress } from '../api/backend';
import { registerReset } from './reset';

/** The latest `job.progress` per job id, fed by the event bridge's single job.progress listener. */
interface JobsStore { jobs: Record<string, JobProgress>; apply(p: JobProgress): void }

export const useJobsStore = create<JobsStore>()((set) => ({
  jobs: {},
  apply: (p) => set((s) => ({ jobs: { ...s.jobs, [p.jobId]: p } })),
}));

// Not account data: a backup or restore runs on regardless of the Kite session (Ruling 85).
registerReset(() => useJobsStore.setState({ jobs: {} }), { onLogout: false, inspect: () => useJobsStore.getState() });
