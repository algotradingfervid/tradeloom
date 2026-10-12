import { useId, useState, type KeyboardEvent } from 'react';
import type { ShortcutAction } from '../../api/backend';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { Button } from '../../components/Button';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { Dialog } from '../../components/Dialog';
import { componentCopy, shortcutsCopy as copy } from '../../i18n/copy';
import { bindingFromEvent, formatBinding, isSupportedBindingKey } from '../../shortcuts/registry';
import { reloadShortcuts } from '../../shortcuts/ShortcutProvider';
import { useShortcutStore } from '../../shortcuts/store';
import { pushToast } from '../../store/toasts';
import { SettingSection } from './SettingRow';
import '../../components/list.css';

export function RebindDialog({ action, onClose }: { action: ShortcutAction; onClose: () => void }) {
  const backend = useBackend();
  const [captured, setCaptured] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const errorId = useId();

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === 'Escape' || e.key === 'Tab') return; // left to the dialog
    e.preventDefault();
    e.stopPropagation(); // never reaches the global shortcut listener
    const binding = bindingFromEvent(e);
    if (!binding) return; // a modifier on its own
    if (isSupportedBindingKey(binding)) {
      setCaptured(binding);
      setError(null);
    } else {
      // Go would refuse it (NumpadAdd, IntlBackslash, dead and media keys): explain instead of offering Save.
      setCaptured(null);
      setError(copy.unsupportedKey);
    }
  };

  const save = async () => {
    if (!captured || saving) return;
    setSaving(true);
    try {
      await backend.settings.rebind(action.id, captured);
      // Go also publishes the full list (settings.changed "shortcuts"); a failed re-read shows in the boot notice.
      await reloadShortcuts(backend).catch(() => undefined);
      pushToast({ level: 'success', message: copy.saved });
      onClose();
    } catch (e) {
      const err = parseBackendError(e);
      setError(err.code === 'conflict' ? copy.conflict(err.reason) : err.reason);
      setSaving(false);
    }
  };

  return (
    <Dialog
      open
      title={copy.dialogTitle(action.label)}
      onClose={onClose}
      size="sm"
      footer={<><Button onClick={onClose}>{componentCopy.cancel}</Button><Button variant="primary" disabled={!captured || saving} onClick={() => void save()}>{copy.save}</Button></>}
    >
      <div
        role="textbox"
        aria-readonly="true"
        aria-label={copy.capture}
        aria-describedby={error ? errorId : undefined}
        tabIndex={0}
        className="capture"
        onKeyDown={onKeyDown}
      >
        {captured ? formatBinding(captured) : copy.pressKeys}
      </div>
      {error && <p id={errorId} className="field__error" role="alert">{error}</p>}
    </Dialog>
  );
}

export function ShortcutsGroup() {
  const backend = useBackend();
  const actions = useShortcutStore((s) => s.actions);
  const [editing, setEditing] = useState<ShortcutAction | null>(null);
  const [resetting, setResetting] = useState(false);
  const [busy, setBusy] = useState(false);

  const reset = async () => {
    if (busy) return;
    setBusy(true);
    try {
      useShortcutStore.getState().setActions(await backend.settings.resetShortcuts());
      pushToast({ level: 'success', message: copy.resetDone });
    } catch (e) {
      pushToast({ level: 'error', message: parseBackendError(e).reason });
    } finally {
      setBusy(false);
      setResetting(false);
    }
  };

  return (
    <SettingSection title={copy.title}>
      <p className="muted">{copy.intro}</p>
      <table className="data-table">
        <caption className="visually-hidden">{copy.title}</caption>
        <thead>
          <tr>
            <th scope="col">{copy.action}</th>
            <th scope="col">{copy.scope}</th>
            <th scope="col">{copy.binding}</th>
            <th scope="col"><span className="visually-hidden">{copy.change}</span></th>
          </tr>
        </thead>
        <tbody>
          {actions.map((a) => (
            <tr key={a.id}>
              <td>{a.label}</td>
              <td>{copy.scopes[a.scope]}</td>
              <td>{a.binding ? <kbd>{formatBinding(a.binding)}</kbd> : <span className="muted">{copy.unbound}</span>}</td>
              <td><Button aria-label={copy.changeFor(a.label)} onClick={() => setEditing(a)}>{copy.change}</Button></td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="settings__actions">
        <Button onClick={() => setResetting(true)}>{copy.reset}</Button>
      </div>
      {editing && <RebindDialog action={editing} onClose={() => setEditing(null)} />}
      <ConfirmDialog
        open={resetting}
        title={copy.resetTitle}
        message={copy.resetMessage}
        confirmLabel={copy.reset}
        busy={busy}
        onConfirm={() => void reset()}
        onCancel={() => setResetting(false)}
      />
    </SettingSection>
  );
}
