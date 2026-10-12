> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 15.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 15: Settings → Shortcuts and Diagnostics; crash notice

**Files:**
- Create: `frontend/src/views/settings/ShortcutsGroup.tsx`, `frontend/src/views/settings/DiagnosticsGroup.tsx`, `frontend/src/app/CrashNotice.tsx`
- Modify: `frontend/src/views/settings/SettingsView.tsx` (`GROUP_COMPONENTS`), `frontend/src/app/ShellBanners.tsx`, `frontend/src/i18n/copy.ts`
- Test: `frontend/src/views/settings/ShortcutsGroup.test.tsx`, `frontend/src/views/settings/DiagnosticsGroup.test.tsx`

**Interfaces:**
- Consumes: Task 6 `useShortcutStore`, `reloadShortcuts`, `bindingFromEvent`, `formatBinding`; Task 2 `Backend.settings.{shortcuts,rebind,resetShortcuts}` (`ShortcutAction{id, label, scope, defaultBinding, binding}`; `Rebind` returns the updated action, `ResetShortcuts` the whole list), `Backend.diagnostics.{openLogsFolder,createSupportBundle,lastCrash,openCrashReport,dismissCrash}` (`CrashInfo{path, atMs}`, empty `path` = none), `useSetting` on `log.level` (`"info"` or `"debug"`); Task 14 `SettingSection`, `GROUP_COMPONENTS`; canonical facts A1 (`conflict` refusal carries the other action's label) and A8.
- Produces: `ShortcutsGroup()`, `RebindDialog({ action: ShortcutAction; onClose(): void })`, `DiagnosticsGroup()`, `CrashNotice()` (rendered by `ShellBanners`; FR-SHELL-018 AC2 UI).

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/views/settings/ShortcutsGroup.test.tsx`:

```tsx
import { screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderApp } from '../../test/render';

const row = (label: string) => screen.findByRole('row', { name: new RegExp(`^${label}`) });

describe('FR-SHELL-019 Settings → Shortcuts', () => {
  it('lists every registry action with its scope and binding', async () => {
    renderApp({ route: '/settings/shortcuts' });
    expect(await row('Go to Home')).toHaveTextContent('Alt+1');
    expect(await row('Trend line tool')).toHaveTextContent('Chart');
    expect(await row('Stop all trading')).toHaveTextContent('Ctrl+Shift+K');
  });
  it('AC2: a binding used in an overlapping scope is refused, naming the other action', async () => {
    const { user } = renderApp({ route: '/settings/shortcuts' });
    await user.click(await screen.findByRole('button', { name: 'Change shortcut for Stop all trading' }));
    const dialog = screen.getByRole('dialog', { name: 'Change shortcut: Stop all trading' });
    expect(within(dialog).getByLabelText('New shortcut')).toHaveFocus();
    await user.keyboard('{Alt>}1{/Alt}');
    expect(within(dialog).getByLabelText('New shortcut')).toHaveTextContent('Alt+1');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Already used by Go to Home');
  });
  it('AC1 (settings part): remapping shows the new binding and the old one disappears', async () => {
    const { user, backend } = renderApp({ route: '/settings/shortcuts' });
    await user.click(await screen.findByRole('button', { name: 'Change shortcut for Stop all trading' }));
    await user.keyboard('{Control>}{Shift>}x{/Shift}{/Control}');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(await row('Stop all trading')).toHaveTextContent('Ctrl+Shift+X');
    expect(backend.callsTo('settings.rebind')[0].args).toEqual(['kill.switch', 'Ctrl+Shift+X']);
  });
  it('resets all bindings to their defaults after confirmation', async () => {
    const { user, backend } = renderApp({ route: '/settings/shortcuts' });
    await backend.settings.rebind('kill.switch', 'Ctrl+Shift+X');
    await user.click(await screen.findByRole('button', { name: 'Reset to defaults' }));
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Reset to defaults' }));
    expect(await row('Stop all trading')).toHaveTextContent('Ctrl+Shift+K');
  });
});
```

Create `frontend/src/views/settings/DiagnosticsGroup.test.tsx`:

```tsx
import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { demoSeed, FIXED_NOW } from '../../api/mock/fixtures';
import { createMockBackend } from '../../api/mock/index';
import { renderApp } from '../../test/render';

describe('FR-SHELL-018 diagnostics (UI)', () => {
  it('switches the log level at runtime', async () => {
    const { user, backend } = renderApp({ route: '/settings/diagnostics' });
    await user.selectOptions(await screen.findByLabelText('Log detail'), 'debug');
    await waitFor(() => expect(backend.state.settings['log.level']).toBe('"debug"'));
  });
  it('opens the logs folder and creates a support bundle', async () => {
    const { user, backend } = renderApp({ route: '/settings/diagnostics' });
    await user.click(await screen.findByRole('button', { name: 'Open logs folder' }));
    expect(backend.callsTo('diagnostics.openLogsFolder')).toHaveLength(1);
    await user.click(screen.getByRole('button', { name: 'Create support bundle' }));
    expect(await screen.findByText(/Support bundle saved to .*support-20261012-103000\.zip/)).toBeInTheDocument();
  });
  it('AC2 (UI): after a crash the next start shows "The app crashed last time" with View report', async () => {
    const seed = demoSeed();
    seed.crash = { path: '/home/asha/.local/share/tradeloom/logs/crash-20261012-101500.json', atMs: FIXED_NOW - 900_000 };
    const { user, backend } = renderApp({ backend: createMockBackend(seed) });
    expect(await screen.findByText(/The app crashed last time/)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'View report' }));
    expect(backend.callsTo('diagnostics.openCrashReport')).toHaveLength(1);
    await user.click(screen.getByRole('button', { name: 'Dismiss crash notice' }));
    expect(screen.queryByText(/The app crashed last time/)).toBeNull();
    expect(backend.callsTo('diagnostics.dismissCrash')).toHaveLength(1);
  });
  it('shows no crash notice after a clean exit', async () => {
    renderApp();
    await screen.findByRole('button', { name: 'Account menu' });
    expect(screen.queryByText(/The app crashed last time/)).toBeNull();
  });
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd frontend && npx vitest run src/views/settings/ShortcutsGroup.test.tsx src/views/settings/DiagnosticsGroup.test.tsx`
Expected: FAIL — `/settings/shortcuts` and `/settings/diagnostics` render "Page not found"; no crash notice.

- [ ] **Step 3: Implement the groups and the crash notice**

Append to `frontend/src/i18n/copy.ts`:

```ts
export const shortcutsCopy = {
  title: 'Keyboard shortcuts',
  intro: 'Every shortcut can be changed. A key combination can be used only once within overlapping areas of the app.',
  action: 'Action',
  scope: 'Where',
  binding: 'Shortcut',
  scopes: { global: 'Everywhere', view: 'Current view', chart: 'Chart', builder: 'Builder' },
  unbound: 'Not set',
  change: 'Change',
  changeFor: (label: string) => `Change shortcut for ${label}`,
  dialogTitle: (label: string) => `Change shortcut: ${label}`,
  capture: 'New shortcut',
  pressKeys: 'Press the new key combination',
  save: 'Save',
  saved: 'Shortcut saved',
  conflict: (label: string) => `Already used by ${label}`,
  reset: 'Reset to defaults',
  resetTitle: 'Reset to defaults',
  resetMessage: 'All shortcuts return to their original keys.',
  resetDone: 'Shortcuts reset to defaults',
} as const;

export const diagnosticsCopy = {
  title: 'Logs and support',
  level: 'Log detail',
  // Values are the Go `log.level` setting (P0-1): lower-case "info" | "debug".
  levels: [{ value: 'info', label: 'Normal (info)' }, { value: 'debug', label: 'Detailed (debug)' }],
  levelHint: 'Changes apply immediately. Secrets are always removed from logs.',
  openLogs: 'Open logs folder',
  bundle: 'Create support bundle',
  bundleHint: 'The bundle contains logs, settings without secrets, version information, the market calendar version, the last 200 order-journal rows and recent deployment log excerpts.',
  bundleSaved: (path: string) => `Support bundle saved to ${path}`,
} as const;

export const crashCopy = {
  message: (when: string) => `The app crashed last time (${when}).`,
  view: 'View report',
  dismiss: 'Dismiss crash notice',
} as const;
```

Create `frontend/src/views/settings/ShortcutsGroup.tsx`:

```tsx
import { useState, type KeyboardEvent } from 'react';
import type { ShortcutAction } from '../../api/backend';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { Button } from '../../components/Button';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { Dialog } from '../../components/Dialog';
import { componentCopy, shortcutsCopy as copy } from '../../i18n/copy';
import { bindingFromEvent, formatBinding } from '../../shortcuts/registry';
import { reloadShortcuts } from '../../shortcuts/ShortcutProvider';
import { useShortcutStore } from '../../shortcuts/store';
import { pushToast } from '../../store/toasts';
import { SettingSection } from './SettingRow';
import '../../components/list.css';

export function RebindDialog({ action, onClose }: { action: ShortcutAction; onClose: () => void }) {
  const backend = useBackend();
  const [captured, setCaptured] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === 'Escape' || e.key === 'Tab') return; // left to the dialog
    e.preventDefault();
    e.stopPropagation(); // never reaches the global shortcut listener
    const binding = bindingFromEvent(e);
    if (binding) {
      setCaptured(binding);
      setError(null);
    }
  };

  const save = async () => {
    if (!captured) return;
    try {
      await backend.settings.rebind(action.id, captured);
      await reloadShortcuts(backend);
      pushToast({ level: 'success', message: copy.saved });
      onClose();
    } catch (e) {
      const err = parseBackendError(e);
      setError(err.code === 'conflict' ? copy.conflict(err.reason) : err.reason);
    }
  };

  return (
    <Dialog
      open
      title={copy.dialogTitle(action.label)}
      onClose={onClose}
      size="sm"
      footer={<><Button onClick={onClose}>{componentCopy.cancel}</Button><Button variant="primary" disabled={!captured} onClick={() => void save()}>{copy.save}</Button></>}
    >
      <div role="textbox" aria-readonly="true" aria-label={copy.capture} tabIndex={0} className="capture" onKeyDown={onKeyDown}>
        {captured ? formatBinding(captured) : copy.pressKeys}
      </div>
      {error && <p className="field__error" role="alert">{error}</p>}
    </Dialog>
  );
}

export function ShortcutsGroup() {
  const backend = useBackend();
  const actions = useShortcutStore((s) => s.actions);
  const [editing, setEditing] = useState<ShortcutAction | null>(null);
  const [resetting, setResetting] = useState(false);

  const reset = async () => {
    try {
      useShortcutStore.getState().setActions(await backend.settings.resetShortcuts());
      pushToast({ level: 'success', message: copy.resetDone });
    } catch (e) {
      pushToast({ level: 'error', message: parseBackendError(e).reason });
    } finally {
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
      <ConfirmDialog open={resetting} title={copy.resetTitle} message={copy.resetMessage} confirmLabel={copy.reset} onConfirm={() => void reset()} onCancel={() => setResetting(false)} />
    </SettingSection>
  );
}
```

Create `frontend/src/views/settings/DiagnosticsGroup.tsx`:

```tsx
import { useState } from 'react';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { Button } from '../../components/Button';
import { SelectField } from '../../components/SelectField';
import { diagnosticsCopy as copy } from '../../i18n/copy';
import { useSetting } from '../../store/settings';
import { pushToast } from '../../store/toasts';
import { SettingSection } from './SettingRow';

const fail = (e: unknown) => pushToast({ level: 'error', message: parseBackendError(e).reason });

export function DiagnosticsGroup() {
  const backend = useBackend();
  const [level, setLevel] = useSetting<string>('log.level', 'info');
  const [bundling, setBundling] = useState(false);

  const bundle = async () => {
    setBundling(true);
    try {
      pushToast({ level: 'success', message: copy.bundleSaved(await backend.diagnostics.createSupportBundle()) });
    } catch (e) {
      fail(e);
    } finally {
      setBundling(false);
    }
  };

  return (
    <SettingSection title={copy.title}>
      <SelectField label={copy.level} options={copy.levels} value={level} onChange={(e) => void setLevel(e.target.value)} />
      <p className="muted">{copy.levelHint}</p>
      <div className="settings__actions">
        <Button onClick={() => void backend.diagnostics.openLogsFolder().catch(fail)}>{copy.openLogs}</Button>
        <Button onClick={() => void bundle()} disabled={bundling}>{copy.bundle}</Button>
      </div>
      <p className="muted">{copy.bundleHint}</p>
    </SettingSection>
  );
}
```

Create `frontend/src/app/CrashNotice.tsx`:

```tsx
import { useEffect, useState } from 'react';
import type { CrashInfo } from '../api/backend';
import { useBackend } from '../api/BackendContext';
import { Button } from '../components/Button';
import { Icon } from '../components/Icon';
import { IconButton } from '../components/IconButton';
import { crashCopy } from '../i18n/copy';
import { formatDateTimeLong } from '../i18n/format';

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
      <Button onClick={() => void backend.diagnostics.openCrashReport()}>{crashCopy.view}</Button>
      <IconButton label={crashCopy.dismiss} icon="close" onClick={() => { setCrash(null); void backend.diagnostics.dismissCrash(); }} />
    </div>
  );
}
```

In `frontend/src/app/ShellBanners.tsx` add `import { CrashNotice } from './CrashNotice';` and render `<CrashNotice />` as the first child of the returned fragment.

In `frontend/src/views/settings/SettingsView.tsx` add the imports `import { DiagnosticsGroup } from './DiagnosticsGroup';` and `import { ShortcutsGroup } from './ShortcutsGroup';`, and add `shortcuts: ShortcutsGroup,` and `diagnostics: DiagnosticsGroup,` to `GROUP_COMPONENTS`.

- [ ] **Step 4: Run the suite to verify it passes**

Run: `cd frontend && npx vitest run && npm run typecheck`
Expected: PASS (39 test files), `tsc` exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): shortcut remapping with conflict refusal, diagnostics, crash notice (P0-4 Task 15)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
