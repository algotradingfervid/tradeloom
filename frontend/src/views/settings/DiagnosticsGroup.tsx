import { useState } from 'react';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { Button } from '../../components/Button';
import { SelectField } from '../../components/SelectField';
import { diagnosticsCopy as copy } from '../../i18n/copy';
import { beginJobClaim, endJobClaim } from '../../store/jobNotices';
import { useSetting } from '../../store/settings';
import { pushToast } from '../../store/toasts';
import { SettingSection } from './SettingRow';

const fail = (e: unknown) => pushToast({ level: 'error', message: parseBackendError(e).reason });

export function DiagnosticsGroup() {
  const backend = useBackend();
  const [level, setLevel] = useSetting<string>('log.level', 'info');
  const [levelError, setLevelError] = useState<string | null>(null);
  const [bundling, setBundling] = useState(false);

  const bundle = async () => {
    if (bundling) return;
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
      setBundling(false);
    }
  };

  return (
    <SettingSection title={copy.title}>
      <SelectField
        label={copy.level}
        options={[...copy.levels]}
        value={level}
        error={levelError}
        onChange={async (e) => setLevelError((await setLevel(e.target.value))?.reason ?? null)}
      />
      <p className="muted">{copy.levelHint}</p>
      <div className="settings__actions">
        <Button onClick={() => void backend.diagnostics.openLogsFolder().catch(fail)}>{copy.openLogs}</Button>
        <Button onClick={() => void bundle()} disabled={bundling}>{copy.bundle}</Button>
      </div>
      <p className="muted">{copy.bundleHint}</p>
    </SettingSection>
  );
}
