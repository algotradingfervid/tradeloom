import { describe, expect, it } from 'vitest';
import { startEventBridge } from '../api/events';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { claimJob } from './jobNotices';
import { useToastStore } from './toasts';

const messages = () => useToastStore.getState().visible.map((t) => `${t.level}: ${t.message}`);
const job = { kind: 'backup' as const, stage: 'write', done: 2, total: 4, resultPath: '' };

describe('job.progress notices (P0-3 Ruling 71/74, Task 30)', () => {
  it('a failed job the UI did not start (scheduled backup) shows its redacted reason as an error toast', () => {
    const backend = createMockBackend(demoSeed());
    const stop = startEventBridge(backend, { navigate: () => {} });
    backend.emit('job.progress', { ...job, jobId: 'job-9', finished: false, error: '' });
    backend.emit('job.progress', { ...job, jobId: 'job-9', finished: true, error: 'The scheduled backup failed: folder not writable' });
    expect(messages()).toEqual(['error: The scheduled backup failed: folder not writable']);
    stop();
    expect(backend.listenerCount('job.progress')).toBe(0);
  });
  it('stays silent for jobs the UI claimed and for unknown jobs that succeed', () => {
    const backend = createMockBackend(demoSeed());
    const stop = startEventBridge(backend, { navigate: () => {} });
    claimJob('job-1');
    backend.emit('job.progress', { ...job, jobId: 'job-1', finished: true, error: 'Disk full' });
    backend.emit('job.progress', { ...job, jobId: 'job-2', finished: true, error: '', resultPath: '/backups/a.zip' });
    expect(messages()).toEqual([]);
    stop();
  });
});
