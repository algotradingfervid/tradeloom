import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { TextField } from '../../components/TextField';
import { settingsViewCopy } from '../../i18n/copy';
import { useSetting } from '../../store/settings';
import './settings.css';

export function SettingSection({ title, children }: { title: string; children: ReactNode }) {
  const id = useId();
  return (
    <section className="settings__section" aria-labelledby={id}>
      <h2 id={id}>{title}</h2>
      {children}
    </section>
  );
}

export function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="fact">
      <dt>{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}

export interface NumberCommitFieldProps { label: string; value: number; onCommit: (n: number) => Promise<string | null>; hint?: string }

/** Commits on blur or Enter; Escape reverts; a refusal reason is shown inline and the stored value is kept. */
export function NumberCommitField({ label, value, onCommit, hint }: NumberCommitFieldProps) {
  const [draft, setDraft] = useState(String(value));
  const [error, setError] = useState<string | null>(null);
  /** A commit is in flight: Enter followed by blur (or a second Enter) must not write twice (M2). */
  const pending = useRef(false);
  useEffect(() => setDraft(String(value)), [value]);

  const commit = async () => {
    if (pending.current) return;
    if (draft === String(value)) {
      setError(null);
      return;
    }
    const n = Number(draft);
    if (draft.trim() === '' || !Number.isInteger(n)) {
      setError(settingsViewCopy.wholeNumber);
      return;
    }
    pending.current = true;
    let reason: string | null;
    try {
      reason = await onCommit(n);
    } finally {
      pending.current = false;
    }
    setError(reason);
    if (reason) setDraft(String(value));
  };

  return (
    <TextField
      label={label}
      hint={hint}
      inputMode="numeric"
      value={draft}
      error={error}
      onChange={(e) => setDraft(e.target.value)}
      onBlur={() => void commit()}
      onKeyDown={(e) => {
        if (e.key === 'Enter') void commit();
        if (e.key === 'Escape') {
          setDraft(String(value));
          setError(null);
        }
      }}
    />
  );
}

export interface SettingNumberFieldProps { settingKey: string; label: string; fallback: number; hint?: string }

/** A `Settings.Set`-backed number (Go validates; the refusal reason is shown inline). */
export function SettingNumberField({ settingKey, label, fallback, hint }: SettingNumberFieldProps) {
  const [value, setValue] = useSetting<number>(settingKey, fallback);
  return <NumberCommitField label={label} value={value} hint={hint} onCommit={async (n) => (await setValue(n))?.reason ?? null} />;
}
