import { useRef, useState } from 'react';
import { useBackend } from '../api/BackendContext';
import { parseBackendError } from '../api/errors';
import { Button } from '../components/Button';
import { diagnosticsCopy as copy } from '../i18n/copy';
import { beginJobClaim, endJobClaim } from '../store/jobNotices';
import { pushToast } from '../store/toasts';
import './shell.css';

const fail = (e: unknown) => pushToast({ level: 'error', message: parseBackendError(e).reason });

/**
 * FR-SHELL-018 entry points shared by Settings → Diagnostics and Help (Ruling 117 I2): open the logs folder and create
 * a support bundle. The bundle call runs inside a Ruling 86 claim window and is guarded while it runs.
 */
export function useSupportActions() {
  const backend = useBackend();
  const [bundling, setBundling] = useState(false);
  // Synchronous guard: two clicks in the same frame must not start two bundles.
  const running = useRef(false);

  const openLogs = () => void backend.diagnostics.openLogsFolder().catch(fail);

  const createBundle = async () => {
    if (running.current) return;
    running.current = true;
    setBundling(true);
    // Go runs the bundle in its single job slot (job.progress kind support_bundle) but returns the path, not a job
    // id: the claim window covers other jobs finishing meanwhile (Ruling 86); the call reports its own outcome.
    beginJobClaim();
    try {
      pushToast({ level: 'success', message: copy.bundleSaved(await backend.diagnostics.createSupportBundle()) });
    } catch (e) {
      fail(e);
    } finally {
      // No job id comes back, so the window always closes without one (the done flag of Ruling 86 picks the id).
      endJobClaim();
      running.current = false;
      setBundling(false);
    }
  };

  return { openLogs, createBundle, bundling };
}

/** The two support buttons; the bundle button is disabled and aria-busy while a bundle is being written. */
export function SupportActions() {
  const { openLogs, createBundle, bundling } = useSupportActions();
  return (
    <div className="settings__actions support-actions">
      <Button onClick={openLogs}>{copy.openLogs}</Button>
      <Button onClick={() => void createBundle()} disabled={bundling} aria-busy={bundling || undefined}>{copy.bundle}</Button>
    </div>
  );
}
