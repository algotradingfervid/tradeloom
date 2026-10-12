import { useCallback, useEffect, useId, useRef, useState, type FormEvent } from 'react';
import type { CredentialsInfo, SecretStorageInfo } from '../../api/backend';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { Button } from '../../components/Button';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { Dialog } from '../../components/Dialog';
import { TextField } from '../../components/TextField';
import { componentCopy, kiteSettingsCopy as copy } from '../../i18n/copy';
import { DASH } from '../../i18n/format';
import { useSecretsNoticeStore } from '../../store/secretsNotice';
import { useSessionStore } from '../../store/session';
import { pushToast } from '../../store/toasts';
import { Fact, NumberCommitField, SettingSection } from './SettingRow';

/**
 * The API secret lives only in this component's local state while the user types. It is cleared after every
 * submit (success or refusal) and discarded with the component when the dialog closes (it is mounted only while
 * open). It is never put in a store, web storage, the URL, a toast or a log, and the field never shows it back.
 *
 * `logoutFirst`: the user confirmed that changing credentials logs them out. Go's SaveCredentials refuses
 * `already_logged_in` while a session is ready, so the dialog logs out first (Ruling 97 I1).
 */
function ChangeCredentialsDialog({ logoutFirst, onClose }: { logoutFirst: boolean; onClose: () => void }) {
  const backend = useBackend();
  const formId = useId();
  const errorId = useId();
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
      if (logoutFirst && useSessionStore.getState().session?.state === 'ready') {
        const res = await backend.session.logout();
        if (res.message) pushToast({ level: 'warning', message: res.message });
      }
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
  const describedBy = error ? errorId : undefined;
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
        <TextField
          label={copy.apiKey}
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          autoComplete="off"
          spellCheck={false}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy}
        />
        <TextField
          label={copy.apiSecret}
          type="password"
          value={secret}
          onChange={(e) => setSecret(e.target.value)}
          autoComplete="new-password"
          spellCheck={false}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy}
        />
        {error && <p id={errorId} className="field__error" role="alert">{error}</p>}
      </form>
    </Dialog>
  );
}

export function KiteGroup() {
  const backend = useBackend();
  const session = useSessionStore((s) => s.session);
  const leftoverFile = useSecretsNoticeStore((s) => s.leftoverFile);
  const setLeftoverFile = useSecretsNoticeStore((s) => s.setLeftoverFile);
  const noticeId = useId();
  const warningId = useId();
  const busyId = useId();
  const migrateErrorId = useId();
  const [creds, setCreds] = useState<CredentialsInfo | null>(null);
  const [storage, setStorage] = useState<SecretStorageInfo | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const [confirmingChange, setConfirmingChange] = useState(false);
  const [changing, setChanging] = useState<{ logoutFirst: boolean } | null>(null);
  const [removing, setRemoving] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  /** Redirect URL to register after a port change (P0-3 Task 10 carry-forward). */
  const [portNotice, setPortNotice] = useState<string | null>(null);
  const [migrating, setMigrating] = useState(false);
  const [migrateError, setMigrateError] = useState<string | null>(null);

  // M3: only the newest load may land, never after unmount; one error toast per failure streak.
  const loadSeq = useRef(0);
  const mounted = useRef(true);
  const failing = useRef(false);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  const load = useCallback(async () => {
    const seq = ++loadSeq.current;
    try {
      const [c, s] = await Promise.all([backend.session.credentials(), backend.session.secretStorage()]);
      if (!mounted.current || seq !== loadSeq.current) return;
      setCreds(c);
      setStorage(s);
      setLoadFailed(false);
      failing.current = false;
    } catch (e) {
      if (!mounted.current || seq !== loadSeq.current) return;
      setLoadFailed(true);
      if (!failing.current) {
        failing.current = true;
        pushToast({ level: 'error', message: parseBackendError(e).reason });
      }
    }
  }, [backend]);

  // Setup, login, logout, unlock and credential changes publish "session.state": re-read on each new session
  // object. MigrateToKeyring and SetRedirectPort publish nothing, so their handlers update or re-read directly.
  useEffect(() => {
    void load();
  }, [load, session]);

  if (!session) return null;
  if (!creds) {
    return loadFailed ? (
      <div className="settings__section">
        <p className="muted">{copy.loadFailed}</p>
        <div className="settings__actions">
          <Button onClick={() => void load()}>{copy.loadFailedRetry}</Button>
        </div>
      </div>
    ) : null;
  }

  const loggingIn = session.state === 'logging_in';

  const startChange = () => {
    if (session.state === 'ready') setConfirmingChange(true);
    else setChanging({ logoutFirst: false });
  };

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
      void load(); // CredentialsInfo.secretBackend changed too (I2)
    } catch (e) {
      const err = parseBackendError(e);
      if (err.code === 'file_not_removed') setLeftoverFile(true);
      else setMigrateError(err.reason);
      // The refusal drops Go's SecretStorageInfo (after file_not_removed the keyring is the backend): read it again.
      void load();
    } finally {
      setMigrating(false);
    }
  };

  // Ruling 68: always offer the move for the file backend; Go probes the keyring and refuses when it is unavailable.
  const canMigrate = storage?.backend === 'file' && !leftoverFile;
  const fileLingers = storage?.backend === 'keyring' && storage.fileExists && !leftoverFile;
  const migrateDescribedBy = migrateError ? migrateErrorId : undefined;

  return (
    <>
      <SettingSection title={copy.credentials}>
        <dl className="facts">
          <Fact label={copy.apiKey}>{creds.apiKeyMasked || DASH}</Fact>
          <Fact label={copy.dataPlan}>{copy.plan[session.dataPlan]}</Fact>
          <Fact label={copy.secretStorage}>{copy.backend[creds.secretBackend] ?? creds.secretBackend}</Fact>
        </dl>
        <div className="settings__actions">
          <Button onClick={startChange} disabled={loggingIn} aria-describedby={loggingIn ? busyId : undefined}>{copy.change}</Button>
          <Button variant="danger" onClick={() => setRemoving(true)}>{copy.remove}</Button>
          {canMigrate && (
            <Button onClick={() => void migrate()} disabled={migrating} aria-describedby={migrateDescribedBy}>{copy.migrate}</Button>
          )}
        </div>
        {loggingIn && <p id={busyId} className="muted">{copy.changeLoginBusy}</p>}
        {migrateError && <p id={migrateErrorId} className="field__error" role="alert">{migrateError}</p>}
        {fileLingers && <p className="muted">{copy.leftoverFileInfo}</p>}
        {leftoverFile && (
          <div className="settings__notice settings__notice--warn" role="alert" aria-labelledby={warningId}>
            <strong id={warningId}>{copy.fileNotRemovedTitle}</strong>
            <span>{copy.fileNotRemoved}</span>
            <Button onClick={() => void migrate()} disabled={migrating} aria-describedby={migrateDescribedBy}>{copy.fileNotRemovedRetry}</Button>
          </div>
        )}
      </SettingSection>
      <SettingSection title={copy.login}>
        <TextField label={copy.redirectUrl} value={creds.redirectUrl} readOnly />
        <NumberCommitField label={copy.port} value={creds.port} hint={copy.portHint} onCommit={savePort} />
        {/* Always mounted so the notice is announced when it appears (M5). */}
        <div role="status" aria-labelledby={portNotice ? noticeId : undefined} className={portNotice ? 'settings__notice' : undefined}>
          {portNotice && (
            <>
              <strong id={noticeId}>{copy.portChangedTitle}</strong>
              <span>{copy.portChanged(portNotice)}</span>
            </>
          )}
        </div>
      </SettingSection>
      <ConfirmDialog
        open={confirmingChange}
        title={copy.changeConfirmTitle}
        message={copy.changeConfirmMessage}
        confirmLabel={copy.changeConfirm}
        onConfirm={() => {
          setConfirmingChange(false);
          setChanging({ logoutFirst: true });
        }}
        onCancel={() => setConfirmingChange(false)}
      />
      {changing && <ChangeCredentialsDialog logoutFirst={changing.logoutFirst} onClose={() => setChanging(null)} />}
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
