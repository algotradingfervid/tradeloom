import { useEffect, useState } from 'react';
import type { CrashInfo } from '../api/backend';
import { useBackend } from '../api/BackendContext';
import { parseBackendError } from '../api/errors';
import { Button } from '../components/Button';
import { Icon } from '../components/Icon';
import { IconButton } from '../components/IconButton';
import { crashCopy } from '../i18n/copy';
import { formatDateTimeLong } from '../i18n/format';
import { pushToast } from '../store/toasts';

const fail = (e: unknown) => pushToast({ level: 'error', message: parseBackendError(e).reason });

/** FR-SHELL-018 AC2 (UI): the newest unseen crash report from the previous run (Diagnostics.LastCrash). */
export function CrashNotice() {
  const backend = useBackend();
  const [crash, setCrash] = useState<CrashInfo | null>(null);
  useEffect(() => {
    let alive = true;
    backend.diagnostics.lastCrash().then((c) => { if (alive) setCrash(c); }, () => undefined);
    return () => { alive = false; };
  }, [backend]);
  if (!crash || !crash.path) return null;
  return (
    <div className="banner banner--error" role="status">
      <Icon name="error" />
      <span>{crashCopy.message(formatDateTimeLong(crash.atMs))}</span>
      <Button onClick={() => void backend.diagnostics.openCrashReport().catch(fail)}>{crashCopy.view}</Button>
      <IconButton label={crashCopy.dismiss} icon="close" onClick={() => { setCrash(null); void backend.diagnostics.dismissCrash().catch(fail); }} />
    </div>
  );
}
