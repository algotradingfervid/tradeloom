import { useCallback, useEffect, useId, useState, type FormEvent } from 'react';
import type { CredentialsInfo, SecretStorageInfo } from '../../api/backend';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { Button } from '../../components/Button';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { Dialog } from '../../components/Dialog';
import { TextField } from '../../components/TextField';
import { componentCopy, kiteSettingsCopy as copy } from '../../i18n/copy';
import { DASH } from '../../i18n/format';
import { useSessionStore } from '../../store/session';
import { pushToast } from '../../store/toasts';
import { Fact, NumberCommitField, SettingSection } from './SettingRow';

/**
 * The API secret lives only in this component's local state while the user types. It is cleared after every
 * submit (success or refusal) and discarded with the component when the dialog closes (it is mounted only while
 * open). It is never put in a store, web storage, the URL, a toast or a log, and the field never shows it back.
 */
function ChangeCredentialsDialog({ onClose }: { onClose: () => void }) {
  const backend = useBackend();
  const formId = useId();
  const [apiKey, setApiKey] = useState('');
  const [secret, setSecret] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const submit = async (e?: FormEvent) => {
    e?.preventDefault();
    if (busy) return;
    setBusy(true);
    setError(null);
    let saved = false;
    try {
      await backend.session.saveCredentials({ apiKey: apiKey.trim(), apiSecret: secret });
      saved = true;
    } catch (err) {
      setError(parseBackendError(err).reason);
    } finally {
      setSecret('');
      setBusy(false);
    }
    if (saved) {
      pushToast({ level: 'success', message: copy.saved });
      onClose();
    }
  };
  return (
    <Dialog
      open
      title={copy.changeTitle}
      onClose={onClose}
      dismissible={!busy}
      footer={
        <>
          <Button onClick={onClose} disabled={busy}>{componentCopy.cancel}</Button>
          <Button variant="primary" type="submit" form={formId} disabled={busy}>{copy.save}</Button>
        </>
      }
    >
      <form id={formId} onSubmit={(e) => void submit(e)} noValidate className="settings__section">
        <TextField label={copy.apiKey} value={apiKey} onChange={(e) => setApiKey(e.target.value)} autoComplete="off" spellCheck={false} />
        <TextField
          label={copy.apiSecret}
          type="password"
          value={secret}
          onChange={(e) => setSecret(e.target.value)}
          autoComplete="new-password"
          spellCheck={false}
          error={error}
        />
      </form>
    </Dialog>
  );
}

export function KiteGroup() {
  const backend = useBackend();
  const session = useSessionStore((s) => s.session);
  const noticeId = useId();
  const warningId = useId();
  const [creds, setCreds] = useState<CredentialsInfo | null>(null);
  const [storage, setStorage] = useState<SecretStorageInfo | null>(null);
  const [changing, setChanging] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  /** Redirect URL to register after a port change (P0-3 Task 10 carry-forward). */
  const [portNotice, setPortNotice] = useState<string | null>(null);
  const [migrating, setMigrating] = useState(false);
  const [migrateError, setMigrateError] = useState<string | null>(null);
  /** MigrateToKeyring refused with file_not_removed: secrets moved, the old file is still there (Ruling 68). */
  const [leftoverFile, setLeftoverFile] = useState(false);

  const load = useCallback(async () => {
    try {
      const [c, s] = await Promise.all([backend.session.credentials(), backend.session.secretStorage()]);
      setCreds(c);
      setStorage(s);
    } catch (e) {
      pushToast({ level: 'error', message: parseBackendError(e).reason });
    }
  }, [backend]);

  // Every credentials/secrets change is followed by a "session.state" event, so re-read on each new session object.
  useEffect(() => {
    void load();
  }, [load, session]);

  if (!session || !creds) return null;

  const savePort = async (port: number): Promise<string | null> => {
    try {
      const next = await backend.session.setRedirectPort(port);
      setCreds(next);
      if (next.port !== creds.port) setPortNotice(next.redirectUrl);
      return null;
    } catch (e) {
      return parseBackendError(e).reason;
    }
  };

  const remove = async () => {
    setBusy(true);
    setError(null);
    try {
      await backend.session.removeCredentials();
      setRemoving(false);
    } catch (e) {
      setError(parseBackendError(e).reason);
    } finally {
      setBusy(false);
    }
  };

  // Also the retry after file_not_removed: Go then removes only the leftover file (Rulings 68, 72 (b)).
  const migrate = async () => {
    if (migrating) return;
    setMigrating(true);
    setMigrateError(null);
    try {
      setStorage(await backend.session.migrateToKeyring());
      setLeftoverFile(false);
      pushToast({ level: 'success', message: copy.migrated });
    } catch (e) {
      const err = parseBackendError(e);
      if (err.code === 'file_not_removed') {
        setLeftoverFile(true);
        // The refusal drops Go's SecretStorageInfo (the keyring is now the backend): read it again.
        void load();
      } else {
        setLeftoverFile(false);
        setMigrateError(err.reason);
      }
    } finally {
      setMigrating(false);
    }
  };

  // Ruling 68: always offer the move for the file backend; Go probes the keyring and refuses when it is unavailable.
  const canMigrate = storage?.backend === 'file' && !leftoverFile;

  return (
    <>
      <SettingSection title={copy.credentials}>
        <dl className="facts">
          <Fact label={copy.apiKey}>{creds.apiKeyMasked || DASH}</Fact>
          <Fact label={copy.dataPlan}>{copy.plan[session.dataPlan]}</Fact>
          <Fact label={copy.secretStorage}>{copy.backend[creds.secretBackend] ?? creds.secretBackend}</Fact>
        </dl>
        <div className="settings__actions">
          <Button onClick={() => setChanging(true)}>{copy.change}</Button>
          <Button variant="danger" onClick={() => setRemoving(true)}>{copy.remove}</Button>
          {canMigrate && <Button onClick={() => void migrate()} disabled={migrating}>{copy.migrate}</Button>}
        </div>
        {migrateError && <p className="field__error" role="alert">{migrateError}</p>}
        {leftoverFile && (
          <div className="settings__notice settings__notice--warn" role="alert" aria-labelledby={warningId}>
            <strong id={warningId}>{copy.fileNotRemovedTitle}</strong>
            <span>{copy.fileNotRemoved}</span>
            <Button onClick={() => void migrate()} disabled={migrating}>{copy.fileNotRemovedRetry}</Button>
          </div>
        )}
      </SettingSection>
      <SettingSection title={copy.login}>
        <TextField label={copy.redirectUrl} value={creds.redirectUrl} readOnly />
        <NumberCommitField label={copy.port} value={creds.port} hint={copy.portHint} onCommit={savePort} />
        {portNotice && (
          <div className="settings__notice" role="status" aria-labelledby={noticeId}>
            <strong id={noticeId}>{copy.portChangedTitle}</strong>
            <span>{copy.portChanged(portNotice)}</span>
          </div>
        )}
      </SettingSection>
      {changing && <ChangeCredentialsDialog onClose={() => setChanging(false)} />}
      <ConfirmDialog
        open={removing}
        title={copy.removeTitle}
        message={copy.removeMessage}
        confirmLabel={copy.remove}
        tone="danger"
        busy={busy}
        error={error}
        onConfirm={() => void remove()}
        onCancel={() => {
          setRemoving(false);
          setError(null);
        }}
      />
    </>
  );
}
