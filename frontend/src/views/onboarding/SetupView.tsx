import { useEffect, useId, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import type { SecretBackendName, SecretChoice } from '../../api/backend';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { Button } from '../../components/Button';
import { TextField } from '../../components/TextField';
import { setupCopy as copy } from '../../i18n/copy';
import { useSessionStore } from '../../store/session';
import { pushToast } from '../../store/toasts';
import { adoptReturnedStatus } from './adoptStatus';
import { OnboardingFrame } from './OnboardingFrame';

// Secrets (Ruling 101): the API secret and the passphrases live only in this component-local state. They are cleared
// after every backend call (finally) and vanish with the component on unmount; they are never put in a store, web
// storage, the URL, a log or a toast. Toasts and inline errors show only Go's reason, which never contains a secret.

function reportFailure(e: unknown): string {
  const reason = parseBackendError(e).reason;
  pushToast({ level: 'error', message: reason });
  return reason;
}

function CredentialsForm({ redirectUrl, secretBackend }: { redirectUrl: string; secretBackend: SecretBackendName }) {
  const backend = useBackend();
  const [apiKey, setApiKey] = useState('');
  const [secret, setSecret] = useState('');
  const [errors, setErrors] = useState<{ apiKey?: string; secret?: string; form?: string }>({});
  const [busy, setBusy] = useState(false);

  const copyRedirect = () => {
    if (!navigator.clipboard) {
      pushToast({ level: 'warning', message: copy.copyUnavailable });
      return;
    }
    navigator.clipboard.writeText(redirectUrl).then(
      () => pushToast({ level: 'success', message: copy.copied }),
      () => pushToast({ level: 'warning', message: copy.copyUnavailable }),
    );
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (busy) return;
    const next = { apiKey: apiKey.trim() ? undefined : copy.apiKeyRequired, secret: secret ? undefined : copy.secretRequired };
    setErrors(next);
    if (next.apiKey || next.secret) return;
    setBusy(true);
    try {
      adoptReturnedStatus(await backend.session.saveCredentials({ apiKey: apiKey.trim(), apiSecret: secret }));
    } catch (err) {
      setErrors({ form: reportFailure(err) });
    } finally {
      setSecret('');
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} noValidate>
      <p>{copy.intro}</p>
      <p>{copy.step1}</p>
      <div className="row">
        <TextField label={copy.redirectLabel} value={redirectUrl} readOnly hint={copy.redirectHint} />
        <Button icon="copy" onClick={copyRedirect}>{copy.copy}</Button>
      </div>
      <p>{secretBackend === 'file' ? copy.step2File : copy.step2}</p>
      <TextField label={copy.apiKey} value={apiKey} onChange={(e) => setApiKey(e.target.value)} error={errors.apiKey} autoComplete="off" spellCheck={false} />
      <TextField label={copy.apiSecret} type="password" value={secret} onChange={(e) => setSecret(e.target.value)} error={errors.secret} autoComplete="off" />
      {errors.form && <p className="field__error" role="alert">{errors.form}</p>}
      <Button type="submit" variant="primary" disabled={busy}>{copy.save}</Button>
      <Link to="/help/kite-setup">{copy.helpLink}</Link>
    </form>
  );
}

const CHOICE_HINTS: Record<SecretChoice, string> = { encrypted_file: copy.encfileHint, no_storage: copy.refuseHint };

/**
 * FR-ACCT-013: renders exactly the choices Go's `secrets.Detect` offers (`Session.SecretStorage().choices`). After a
 * refusal in this run Go reports mode "none" with no choices but still accepts UseEncryptedFile, so the encrypted
 * file stays offered there (the refused notice tells the user to choose it).
 */
function SecretStoreChoice({ refused }: { refused: boolean }) {
  const backend = useBackend();
  const hintIds: Record<SecretChoice, string> = { encrypted_file: useId(), no_storage: useId() };
  const [fetched, setFetched] = useState<SecretChoice[]>([]);
  const [choice, setChoice] = useState<SecretChoice>('encrypted_file');
  const [pass, setPass] = useState('');
  const [repeat, setRepeat] = useState('');
  const [errors, setErrors] = useState<{ pass?: string; repeat?: string; form?: string }>({});
  const [busy, setBusy] = useState(false);
  const choices: SecretChoice[] = fetched.length === 0 && refused ? ['encrypted_file'] : fetched;

  useEffect(() => {
    let live = true;
    backend.session.secretStorage().then(
      (info) => {
        if (live) setFetched(info.choices);
      },
      (err: unknown) => {
        if (live) setErrors({ form: reportFailure(err) });
      },
    );
    return () => {
      live = false;
    };
  }, [backend]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (busy) return;
    if (choice === 'encrypted_file') {
      // Go counts runes (secrets.MinPassphraseRunes = 8).
      const long = [...pass].length >= 8;
      const next = { pass: long ? undefined : copy.passphraseShort, repeat: long && pass !== repeat ? copy.passphraseMismatch : undefined };
      setErrors(next);
      if (next.pass || next.repeat) return;
    }
    setBusy(true);
    try {
      const status = choice === 'no_storage' ? await backend.session.refuseSecretStorage() : await backend.session.useEncryptedFile(pass);
      setErrors({});
      adoptReturnedStatus(status);
    } catch (err) {
      setErrors({ form: reportFailure(err) });
    } finally {
      setPass('');
      setRepeat('');
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} noValidate>
      <p>{copy.noKeyring}</p>
      {refused && <p className="onboarding__notice" role="status">{copy.refusedNotice}</p>}
      <fieldset>
        <legend>{copy.storeLegend}</legend>
        {choices.map((c) => (
          <div className="choice" key={c}>
            <label>
              <input type="radio" name="secret-store" value={c} checked={choice === c} onChange={() => setChoice(c)} aria-describedby={hintIds[c]} />
              {copy[c]}
            </label>
            <span id={hintIds[c]} className="muted">{CHOICE_HINTS[c]}</span>
          </div>
        ))}
      </fieldset>
      {choice === 'encrypted_file' && (
        <>
          <TextField label={copy.passphrase} type="password" value={pass} onChange={(e) => setPass(e.target.value)} error={errors.pass} autoComplete="new-password" />
          <TextField label={copy.passphraseRepeat} type="password" value={repeat} onChange={(e) => setRepeat(e.target.value)} error={errors.repeat} autoComplete="new-password" />
        </>
      )}
      {errors.form && <p className="field__error" role="alert">{errors.form}</p>}
      <Button type="submit" variant="primary" disabled={busy}>{copy.continue}</Button>
    </form>
  );
}

function UnlockForm() {
  const backend = useBackend();
  const [pass, setPass] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    try {
      adoptReturnedStatus(await backend.session.unlock(pass));
    } catch (err) {
      // Go's reason verbatim: it already says "wrong passphrase or damaged file" (Ruling 101).
      setError(parseBackendError(err).reason);
    } finally {
      setPass('');
      setBusy(false);
    }
  };
  return (
    <form onSubmit={submit} noValidate>
      <p>{copy.unlockIntro}</p>
      <TextField label={copy.passphrase} type="password" value={pass} onChange={(e) => setPass(e.target.value)} error={error} autoComplete="current-password" />
      <Button type="submit" variant="primary" disabled={busy}>{copy.unlock}</Button>
    </form>
  );
}

export function SetupView() {
  const session = useSessionStore((s) => s.session);
  if (!session) return null;
  if (session.state === 'locked') {
    return <OnboardingFrame title={copy.unlockTitle}><UnlockForm /></OnboardingFrame>;
  }
  // mode "choose": no keyring and no secrets file yet; mode "none": the user refused storage (Kite cannot be used).
  const needsStore = session.secrets.mode === 'choose' || session.secrets.mode === 'none';
  return (
    <OnboardingFrame title={needsStore ? copy.storeTitle : copy.title}>
      {needsStore
        ? <SecretStoreChoice refused={session.secrets.mode === 'none'} />
        : <CredentialsForm redirectUrl={session.credentials.redirectUrl} secretBackend={session.secrets.backend} />}
    </OnboardingFrame>
  );
}
