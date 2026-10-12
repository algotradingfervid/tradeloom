import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useBackend } from '../../api/BackendContext';
import { logOut } from '../../app/logout';
import { Avatar } from '../../app/ProfileMenu';
import { useNow } from '../../app/useNow';
import { useProfile } from '../../app/useProfile';
import { Button } from '../../components/Button';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { ListPage } from '../../components/ListPage';
import { Skeleton } from '../../components/Skeleton';
import { TextField } from '../../components/TextField';
import { profileCopy as copy } from '../../i18n/copy';
import { formatDateTimeLong, formatExpiry } from '../../i18n/format';
import { useSessionStore } from '../../store/session';
import '../../components/list.css';
import './profile.css';

/**
 * FR-ACCT-001 profile (read-only, from Go's kite.Profile, which carries no credential) and FR-ACCT-004 log out.
 * The profile lives only in the session store, which drops it whenever the session leaves `ready` (Ruling 98);
 * useProfile drops late responses (Ruling 90).
 */
export function ProfileView() {
  const backend = useBackend();
  const navigate = useNavigate();
  const { profile, failed, reload } = useProfile();
  const session = useSessionStore((s) => s.session);
  const ready = session?.state === 'ready';
  const now = useNow();
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const logout = async () => {
    setBusy(true);
    setError(null);
    // The shared logout (Ruling 108 M12) toasts Go's note when Kite could not invalidate the token, and the reason
    // when Go refused. On success Go publishes logged_out: the session gate replaces this view with /login, whose
    // heading takes focus (Ruling 108 I1), so focus is not stranded on the removed dialog.
    const failure = await logOut(backend);
    setBusy(false);
    if (failure) setError(failure.reason);
    else setConfirming(false);
  };

  let body;
  if (!ready) {
    // Only `expired` reaches here (the gate sends every other non-ready state to setup/login).
    body = (
      <div className="profile-notice" role="status">
        <p>{copy.expired}</p>
        <Button variant="primary" onClick={() => void navigate('/login')}>{copy.logIn}</Button>
      </div>
    );
  } else if (profile) {
    body = (
      <>
        <div className="card profile-card">
          <Avatar profile={profile} size={56} />
          <div>
            <strong>{profile.name}</strong>
            <div className="muted">{profile.userId}<span className="chip">{profile.broker}</span></div>
          </div>
        </div>
        <div className="profile-fields">
          <TextField label={copy.fields.name} value={profile.name} readOnly />
          <TextField label={copy.fields.email} value={profile.email} readOnly />
          <TextField label={copy.fields.clientId} value={profile.userId} readOnly />
          <TextField label={copy.fields.broker} value={profile.broker} readOnly />
          <TextField label={copy.fields.exchanges} value={profile.exchanges.join(', ')} readOnly />
          <TextField label={copy.fields.products} value={profile.products.join(', ')} readOnly />
          <TextField label={copy.fields.orderTypes} value={profile.orderTypes.join(', ')} readOnly />
        </div>
      </>
    );
  } else if (failed) {
    body = (
      <div className="profile-notice" role="status">
        <p>{copy.loadFailed}</p>
        <Button icon="refresh" onClick={() => void reload()}>{copy.retry}</Button>
      </div>
    );
  } else {
    body = <Skeleton rows={4} label={copy.loading} />;
  }

  return (
    <ListPage title={copy.title} description={copy.description} onRefresh={ready ? () => void reload() : undefined}>
      {body}
      {ready && (
        <>
          <section className="profile-session" aria-labelledby="profile-session">
            <h2 id="profile-session">{copy.session}</h2>
            <p>{copy.loggedInAt(formatDateTimeLong(session.loggedInAtMs))}</p>
            <p>{copy.expires(formatExpiry(session.expiresAtMs, now))}</p>
          </section>
          <div className="profile-actions">
            <Button variant="danger" icon="logout" onClick={() => setConfirming(true)}>{copy.logOut}</Button>
          </div>
        </>
      )}
      <ConfirmDialog
        open={confirming && ready}
        title={copy.confirmTitle}
        message={copy.confirmMessage}
        confirmLabel={copy.logOut}
        tone="danger"
        busy={busy}
        error={error}
        onConfirm={() => void logout()}
        onCancel={() => {
          setConfirming(false);
          setError(null);
        }}
      />
    </ListPage>
  );
}
