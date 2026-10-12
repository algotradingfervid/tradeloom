import { describe, expect, it } from 'vitest';
import type { JobProgress } from '../api/backend';
import { claimJob, notifyUnclaimedJob } from './jobNotices';
import { resetOnLogout } from './reset';
import { pushToast, useToastStore } from './toasts';

// Ruling 85: toasts and job claims are not account data, so signing out must not clear them.
describe('logout reset (Ruling 85)', () => {
  it('keeps visible toasts', () => {
    pushToast({ level: 'error', message: 'The scheduled backup failed' });
    resetOnLogout();
    expect(useToastStore.getState().visible.map((t) => t.message)).toEqual(['The scheduled backup failed']);
  });

  it('keeps job claims, so a claimed job that fails after logout is not toasted', () => {
    claimJob('job-1');
    resetOnLogout();
    notifyUnclaimedJob({ jobId: 'job-1', finished: true, error: 'disk full' } as JobProgress);
    expect(useToastStore.getState().visible).toEqual([]);
  });
});
