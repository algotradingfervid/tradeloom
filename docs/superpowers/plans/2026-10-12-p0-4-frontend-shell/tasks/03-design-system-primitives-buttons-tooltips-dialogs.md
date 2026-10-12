> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 3.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 3: Design-system primitives — buttons, tooltips, dialogs, menus, fields

**Files:**
- Create: `frontend/src/components/Button.tsx`, `IconButton.tsx`, `Tooltip.tsx`, `Dialog.tsx`, `ConfirmDialog.tsx`, `MenuButton.tsx`, `Toggle.tsx`, `TextField.tsx`, `SelectField.tsx`, `Skeleton.tsx`, `focus.ts`, `components.css` (all under `frontend/src/components/`)
- Modify: `frontend/src/app/Sidebar.tsx` (tooltips when collapsed, IconButton for the toggle), `frontend/src/i18n/copy.ts`
- Test: `frontend/src/components/Tooltip.test.tsx`, `Dialog.test.tsx`, `MenuButton.test.tsx`, `Fields.test.tsx`

**Interfaces:**
- Consumes: Task 1 `Icon`, `IconName`.
- Produces:
  - `Button(props: ComponentPropsWithRef<'button'> & { variant?: 'primary' | 'secondary' | 'danger' | 'ghost'; icon?: IconName })` — `type` defaults to `"button"`.
  - `IconButton(props: Omit<ComponentPropsWithRef<'button'>, 'children' | 'aria-label'> & { label: string; icon: IconName; tooltip?: string; size?: number })` — accessible name = `label`; tooltip = `tooltip ?? label` (FR-SHELL-013).
  - `Tooltip({ label: string; children: ReactElement; placement?: 'bottom' | 'top' | 'right' })`, `TOOLTIP_DELAY_MS = 500`.
  - `Dialog({ open: boolean; title: string; description?: string; onClose(): void; children?: ReactNode; footer?: ReactNode; size?: 'sm' | 'md' | 'lg'; initialFocus?: RefObject<HTMLElement | null>; closeLabel?: string; dismissible?: boolean })` — portal, `role="dialog"`, `aria-modal`, Escape = `onClose`, focus trap, focus restore, always renders the × `IconButton`.
  - `ConfirmDialog({ open; title; message: ReactNode; confirmLabel: string; cancelLabel?: string; tone?: 'default' | 'danger'; typedConfirmation?: string; busy?: boolean; error?: string | null; onConfirm(): void; onCancel(): void })`.
  - `type MenuItem = { kind: 'item'; id; label; icon?; onSelect() } | { kind: 'checkbox'; id; label; checked; onChange(v) } | { kind: 'separator'; id } | { kind: 'static'; id; content: ReactNode }`; `MenuButton({ label: string; trigger: ReactNode; items: MenuItem[]; align?: 'start' | 'end' })`.
  - `Toggle({ label: string; checked: boolean; onChange(v: boolean): void; description?: string; disabled?: boolean })` (`role="switch"`).
  - `TextField(props: Omit<ComponentPropsWithRef<'input'>, 'id'> & { label: string; error?: string | null; hint?: string })`, `SelectField(props: Omit<ComponentPropsWithRef<'select'>, 'id' | 'children'> & { label: string; options: { value: string; label: string }[]; error?: string | null })`.
  - `Skeleton({ rows?: number; label: string })` (`role="status"`).
  - `focus.ts`: `FOCUSABLE` selector, `focusables(root): HTMLElement[]`, `firstFocusable(root): HTMLElement | null`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/components/Tooltip.test.tsx`:

```tsx
import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IconButton } from './IconButton';

afterEach(() => vi.useRealTimers());

describe('FR-SHELL-013 tooltips', () => {
  it('AC1: an icon-only button exposes its label as the accessible name', () => {
    render(<IconButton label="Refresh chart data" icon="refresh" />);
    expect(screen.getByRole('button', { name: 'Refresh chart data' })).toBeInTheDocument();
  });
  it('appears after 500 ms of hover, not before', () => {
    vi.useFakeTimers();
    render(<IconButton label="Refresh chart data" icon="refresh" />);
    fireEvent.mouseEnter(screen.getByRole('button'));
    act(() => vi.advanceTimersByTime(499));
    expect(screen.queryByRole('tooltip')).toBeNull();
    act(() => vi.advanceTimersByTime(1));
    expect(screen.getByRole('tooltip')).toHaveTextContent('Refresh chart data');
    fireEvent.mouseLeave(screen.getByRole('button'));
    expect(screen.queryByRole('tooltip')).toBeNull();
  });
  it('AC2: appears immediately on keyboard focus and hides on Escape', () => {
    render(<IconButton label="Refresh chart data" icon="refresh" />);
    const button = screen.getByRole('button');
    act(() => button.focus());
    expect(screen.getByRole('tooltip')).toHaveTextContent('Refresh chart data');
    expect(button).toHaveAccessibleDescription('Refresh chart data');
    fireEvent.keyDown(button, { key: 'Escape' });
    expect(screen.queryByRole('tooltip')).toBeNull();
  });
});
```

Create `frontend/src/components/Dialog.test.tsx`:

```tsx
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { Button } from './Button';
import { ConfirmDialog } from './ConfirmDialog';
import { Dialog } from './Dialog';
import { TextField } from './TextField';

function Host() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button onClick={() => setOpen(true)}>Open</button>
      <Dialog open={open} title="Rename" onClose={() => setOpen(false)} footer={<Button onClick={() => setOpen(false)}>Cancel</Button>}>
        <TextField label="Name" defaultValue="Main" />
      </Dialog>
    </>
  );
}

describe('FR-SHELL-015 dialogs', () => {
  it('AC2: Escape closes the dialog and focus returns to the opener', async () => {
    const user = userEvent.setup();
    render(<Host />);
    await user.click(screen.getByText('Open'));
    expect(screen.getByRole('dialog', { name: 'Rename' })).toHaveAttribute('aria-modal', 'true');
    expect(screen.getByLabelText('Name')).toHaveFocus();
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(screen.getByText('Open')).toHaveFocus();
  });
  it('AC3: always renders a visible close × button', async () => {
    const user = userEvent.setup();
    render(<Host />);
    await user.click(screen.getByText('Open'));
    await user.click(screen.getByRole('button', { name: 'Close' }));
    expect(screen.queryByRole('dialog')).toBeNull();
  });
  it('traps Tab and Shift+Tab inside the dialog', async () => {
    const user = userEvent.setup();
    render(<Host />);
    await user.click(screen.getByText('Open'));
    const dialog = screen.getByRole('dialog');
    await user.tab();
    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole('button', { name: 'Close' })).toHaveFocus();
    await user.tab({ shift: true });
    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus();
    expect(dialog).toContainElement(document.activeElement as HTMLElement);
  });
  it('keeps focus inside when the focused control disappears', async () => {
    function Shrinking() {
      const [show, setShow] = useState(true);
      return (
        <Dialog open title="Status" onClose={() => {}}>
          <p>details</p>
          {show && <button onClick={() => setShow(false)}>Vanish</button>}
        </Dialog>
      );
    }
    const user = userEvent.setup();
    render(<Shrinking />);
    await user.click(screen.getByText('Vanish'));
    await waitFor(() => expect(screen.getByRole('dialog')).toHaveFocus());
  });
  it('ConfirmDialog enables confirm only after the exact typed phrase; Escape cancels', async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    render(<ConfirmDialog open title="Restore backup" message="Replaces your data." confirmLabel="Restore" typedConfirmation="RESTORE" onConfirm={onConfirm} onCancel={onCancel} />);
    const confirm = screen.getByRole('button', { name: 'Restore' });
    expect(confirm).toBeDisabled();
    await user.type(screen.getByLabelText('Type RESTORE to confirm'), 'restore');
    expect(confirm).toBeDisabled();
    await user.clear(screen.getByLabelText('Type RESTORE to confirm'));
    await user.type(screen.getByLabelText('Type RESTORE to confirm'), 'RESTORE');
    await user.click(confirm);
    expect(onConfirm).toHaveBeenCalledTimes(1);
    await user.keyboard('{Escape}');
    expect(onCancel).toHaveBeenCalledTimes(1);
  });
});
```

Create `frontend/src/components/MenuButton.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { MenuButton, type MenuItem } from './MenuButton';

function setup() {
  const onProfile = vi.fn();
  const onDark = vi.fn();
  const items: MenuItem[] = [
    { kind: 'static', id: 'who', content: <strong>Asha Rao</strong> },
    { kind: 'separator', id: 's1' },
    { kind: 'item', id: 'profile', label: 'Profile', onSelect: onProfile },
    { kind: 'item', id: 'help', label: 'Help', onSelect: vi.fn() },
    { kind: 'checkbox', id: 'dark', label: 'Dark theme', checked: false, onChange: onDark },
  ];
  render(<><MenuButton label="Account menu" trigger={<span>AR</span>} items={items} /><button>outside</button></>);
  return { onProfile, onDark, user: userEvent.setup() };
}

describe('MenuButton (FR-SHELL-010 keyboard behaviour)', () => {
  it('opens with focus on the first item; arrows cycle; Escape closes and refocuses the trigger', async () => {
    const { user } = setup();
    const trigger = screen.getByRole('button', { name: 'Account menu' });
    await user.click(trigger);
    expect(screen.getByRole('menuitem', { name: 'Profile' })).toHaveFocus();
    await user.keyboard('{ArrowDown}{ArrowDown}');
    expect(screen.getByRole('menuitemcheckbox', { name: 'Dark theme' })).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('menuitem', { name: 'Profile' })).toHaveFocus();
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('menu')).toBeNull();
    expect(trigger).toHaveFocus();
  });
  it('activates items with Enter and closes', async () => {
    const { user, onProfile } = setup();
    await user.click(screen.getByRole('button', { name: 'Account menu' }));
    await user.keyboard('{Enter}');
    expect(onProfile).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('menu')).toBeNull();
  });
  it('reports checkbox changes and closes on outside click', async () => {
    const { user, onDark } = setup();
    await user.click(screen.getByRole('button', { name: 'Account menu' }));
    await user.click(screen.getByRole('menuitemcheckbox', { name: 'Dark theme' }));
    expect(onDark).toHaveBeenCalledWith(true);
    await user.click(screen.getByRole('button', { name: 'Account menu' }));
    await user.click(screen.getByText('outside'));
    expect(screen.queryByRole('menu')).toBeNull();
  });
});
```

Create `frontend/src/components/Fields.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { SelectField } from './SelectField';
import { TextField } from './TextField';
import { Toggle } from './Toggle';

describe('form primitives', () => {
  it('TextField shows an inline error wired to the input', () => {
    render(<TextField label="Port" error="Port must be a whole number between 1024 and 65535" defaultValue="80" />);
    const input = screen.getByLabelText('Port');
    expect(input).toHaveAttribute('aria-invalid', 'true');
    expect(input).toHaveAccessibleDescription('Port must be a whole number between 1024 and 65535');
  });
  it('SelectField is labelled and lists its options', () => {
    render(<SelectField label="Level" options={[{ value: 'INFO', label: 'Info' }, { value: 'DEBUG', label: 'Debug' }]} defaultValue="INFO" />);
    expect(screen.getByLabelText('Level')).toHaveValue('INFO');
    expect(screen.getAllByRole('option')).toHaveLength(2);
  });
  it('Toggle is a labelled switch operable by click and Space', async () => {
    function Host() {
      const [on, setOn] = useState(false);
      return <Toggle label="Dark theme" checked={on} onChange={setOn} />;
    }
    const user = userEvent.setup();
    render(<Host />);
    const sw = screen.getByRole('switch', { name: 'Dark theme' });
    await user.click(sw);
    expect(sw).toHaveAttribute('aria-checked', 'true');
    await user.keyboard(' ');
    expect(sw).toHaveAttribute('aria-checked', 'false');
  });
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd frontend && npx vitest run src/components`
Expected: FAIL — `Failed to resolve import "./IconButton"` (and `./Dialog`, `./MenuButton`, `./TextField`, …).

- [ ] **Step 3: Implement the primitives**

Append to `frontend/src/i18n/copy.ts`:

```ts
export const componentCopy = {
  close: 'Close',
  cancel: 'Cancel',
  typeToConfirm: (phrase: string) => `Type ${phrase} to confirm`,
} as const;
```

Create `frontend/src/components/focus.ts`:

```ts
export const FOCUSABLE = [
  'a[href]', 'button:not([disabled])', 'input:not([disabled]):not([type="hidden"])', 'select:not([disabled])',
  'textarea:not([disabled])', '[tabindex]:not([tabindex="-1"])',
].join(',');

export function focusables(root: Element | null): HTMLElement[] {
  return root ? [...root.querySelectorAll<HTMLElement>(FOCUSABLE)] : [];
}

export function firstFocusable(root: Element | null): HTMLElement | null {
  return focusables(root)[0] ?? null;
}
```

Create `frontend/src/components/Tooltip.tsx`:

```tsx
import { Children, cloneElement, useEffect, useId, useRef, useState, type FocusEvent, type KeyboardEvent, type MouseEvent, type ReactElement } from 'react';

export const TOOLTIP_DELAY_MS = 500;

interface TriggerProps {
  'aria-describedby'?: string;
  onMouseEnter?: (e: MouseEvent<HTMLElement>) => void;
  onMouseLeave?: (e: MouseEvent<HTMLElement>) => void;
  onFocus?: (e: FocusEvent<HTMLElement>) => void;
  onBlur?: (e: FocusEvent<HTMLElement>) => void;
  onKeyDown?: (e: KeyboardEvent<HTMLElement>) => void;
}

export interface TooltipProps { label: string; children: ReactElement<TriggerProps>; placement?: 'bottom' | 'top' | 'right' }

/** Shows after 500 ms hover or immediately on focus; hides on leave, blur or Escape (FR-SHELL-013). */
export function Tooltip({ label, children, placement = 'bottom' }: TooltipProps) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const timer = useRef<number | undefined>(undefined);
  const clear = () => {
    if (timer.current !== undefined) window.clearTimeout(timer.current);
    timer.current = undefined;
  };
  useEffect(() => clear, []);
  const child = Children.only(children);
  const p = child.props;
  const trigger = cloneElement(child, {
    'aria-describedby': open ? id : p['aria-describedby'],
    onMouseEnter: (e: MouseEvent<HTMLElement>) => {
      p.onMouseEnter?.(e);
      clear();
      timer.current = window.setTimeout(() => setOpen(true), TOOLTIP_DELAY_MS);
    },
    onMouseLeave: (e: MouseEvent<HTMLElement>) => {
      p.onMouseLeave?.(e);
      clear();
      setOpen(false);
    },
    onFocus: (e: FocusEvent<HTMLElement>) => {
      p.onFocus?.(e);
      clear();
      setOpen(true);
    },
    onBlur: (e: FocusEvent<HTMLElement>) => {
      p.onBlur?.(e);
      clear();
      setOpen(false);
    },
    onKeyDown: (e: KeyboardEvent<HTMLElement>) => {
      if (e.key === 'Escape') setOpen(false); // not stopped: an enclosing dialog still closes
      p.onKeyDown?.(e);
    },
  });
  return (
    <span className={`tooltip-anchor tooltip-anchor--${placement}`}>
      {trigger}
      {open && <span role="tooltip" id={id} className="tooltip">{label}</span>}
    </span>
  );
}
```

Create `frontend/src/components/Button.tsx`:

```tsx
import type { ComponentPropsWithRef } from 'react';
import { Icon, type IconName } from './Icon';
import './components.css';

export type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'ghost';

export interface ButtonProps extends ComponentPropsWithRef<'button'> { variant?: ButtonVariant; icon?: IconName }

export function Button({ variant = 'secondary', icon, className, type = 'button', children, ...rest }: ButtonProps) {
  return (
    <button type={type} className={['btn', `btn--${variant}`, className].filter(Boolean).join(' ')} {...rest}>
      {icon && <Icon name={icon} size={16} />}
      {children}
    </button>
  );
}
```

Create `frontend/src/components/IconButton.tsx`:

```tsx
import type { ComponentPropsWithRef } from 'react';
import { Icon, type IconName } from './Icon';
import { Tooltip } from './Tooltip';
import './components.css';

export interface IconButtonProps extends Omit<ComponentPropsWithRef<'button'>, 'children' | 'aria-label'> {
  label: string;
  icon: IconName;
  tooltip?: string;
  size?: number;
}

export function IconButton({ label, icon, tooltip, size = 18, className, type = 'button', ...rest }: IconButtonProps) {
  return (
    <Tooltip label={tooltip ?? label}>
      <button type={type} aria-label={label} className={['icon-btn', className].filter(Boolean).join(' ')} {...rest}>
        <Icon name={icon} size={size} />
      </button>
    </Tooltip>
  );
}
```

Create `frontend/src/components/Dialog.tsx`:

```tsx
import { useEffect, useId, useLayoutEffect, useRef, type KeyboardEvent, type ReactNode, type RefObject } from 'react';
import { createPortal } from 'react-dom';
import { componentCopy } from '../i18n/copy';
import { firstFocusable, focusables } from './focus';
import { IconButton } from './IconButton';
import './components.css';

export interface DialogProps {
  open: boolean;
  title: string;
  description?: string;
  onClose: () => void;
  children?: ReactNode;
  footer?: ReactNode;
  size?: 'sm' | 'md' | 'lg';
  initialFocus?: RefObject<HTMLElement | null>;
  closeLabel?: string;
  /** false while an irreversible operation runs: Escape, overlay click and × do nothing. */
  dismissible?: boolean;
}

export function Dialog(props: DialogProps) {
  if (!props.open) return null;
  return createPortal(<DialogPanel {...props} />, document.body);
}

function DialogPanel({ title, description, onClose, children, footer, size = 'md', initialFocus, closeLabel, dismissible = true }: DialogProps) {
  const panel = useRef<HTMLDivElement>(null);
  const titleId = useId();
  const descId = useId();

  useLayoutEffect(() => {
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const root = panel.current;
    const target =
      initialFocus?.current ??
      firstFocusable(root?.querySelector('.dialog__body') ?? null) ??
      firstFocusable(root?.querySelector('.dialog__footer') ?? null) ??
      root;
    target?.focus();
    return () => {
      if (previous && document.contains(previous)) previous.focus();
    };
    // focus is captured once per opening
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // If live updates remove the focused control (e.g. an event storm re-renders the body), keep focus in the dialog.
  useEffect(() => {
    const root = panel.current;
    if (!root) return;
    const observer = new MutationObserver(() => {
      if (document.activeElement === null || document.activeElement === document.body) root.focus();
    });
    observer.observe(root, { childList: true, subtree: true });
    return () => observer.disconnect();
  }, []);

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      if (dismissible) onClose();
      return;
    }
    if (e.key !== 'Tab') return;
    const items = focusables(panel.current);
    if (items.length === 0) {
      e.preventDefault();
      return;
    }
    const first = items[0];
    const last = items[items.length - 1];
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  };

  return (
    <div className="dialog-overlay" onMouseDown={(e) => { if (e.target === e.currentTarget && dismissible) onClose(); }}>
      <div
        ref={panel}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={description ? descId : undefined}
        className={`dialog dialog--${size}`}
        tabIndex={-1}
        onKeyDown={onKeyDown}
      >
        <div className="dialog__header">
          <h2 id={titleId} className="dialog__title">{title}</h2>
          <IconButton label={closeLabel ?? componentCopy.close} icon="close" onClick={onClose} disabled={!dismissible} />
        </div>
        {description && <p id={descId} className="dialog__description">{description}</p>}
        <div className="dialog__body">{children}</div>
        {footer && <div className="dialog__footer">{footer}</div>}
      </div>
    </div>
  );
}
```

Create `frontend/src/components/ConfirmDialog.tsx`:

```tsx
import { useState, type ReactNode } from 'react';
import { componentCopy } from '../i18n/copy';
import { Button } from './Button';
import { Dialog } from './Dialog';
import { TextField } from './TextField';

export interface ConfirmDialogProps {
  open: boolean;
  title: string;
  message: ReactNode;
  confirmLabel: string;
  cancelLabel?: string;
  tone?: 'default' | 'danger';
  typedConfirmation?: string;
  busy?: boolean;
  error?: string | null;
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmDialog(props: ConfirmDialogProps) {
  if (!props.open) return null;
  return <ConfirmDialogBody {...props} />;
}

function ConfirmDialogBody({ title, message, confirmLabel, cancelLabel, tone = 'default', typedConfirmation, busy = false, error, onConfirm, onCancel }: ConfirmDialogProps) {
  const [typed, setTyped] = useState('');
  const ready = !busy && (typedConfirmation === undefined || typed === typedConfirmation);
  return (
    <Dialog
      open
      title={title}
      onClose={onCancel}
      size="sm"
      dismissible={!busy}
      footer={
        <>
          <Button onClick={onCancel} disabled={busy}>{cancelLabel ?? componentCopy.cancel}</Button>
          <Button variant={tone === 'danger' ? 'danger' : 'primary'} onClick={onConfirm} disabled={!ready}>{confirmLabel}</Button>
        </>
      }
    >
      <div className="confirm__message">{message}</div>
      {typedConfirmation !== undefined && (
        <TextField
          label={componentCopy.typeToConfirm(typedConfirmation)}
          value={typed}
          onChange={(e) => setTyped(e.target.value)}
          autoComplete="off"
          spellCheck={false}
        />
      )}
      {error && <p className="field__error" role="alert">{error}</p>}
    </Dialog>
  );
}
```

Create `frontend/src/components/MenuButton.tsx`:

```tsx
import { useEffect, useId, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { Icon, type IconName } from './Icon';
import { Tooltip } from './Tooltip';
import './components.css';

export type MenuItem =
  | { kind: 'item'; id: string; label: string; icon?: IconName; onSelect: () => void }
  | { kind: 'checkbox'; id: string; label: string; checked: boolean; onChange: (v: boolean) => void }
  | { kind: 'separator'; id: string }
  | { kind: 'static'; id: string; content: ReactNode };

export interface MenuButtonProps { label: string; trigger: ReactNode; items: MenuItem[]; align?: 'start' | 'end' }

const ACTIONABLE = '[role="menuitem"],[role="menuitemcheckbox"]';

export function MenuButton({ label, trigger, items, align = 'end' }: MenuButtonProps) {
  const [open, setOpen] = useState(false);
  const button = useRef<HTMLButtonElement>(null);
  const menu = useRef<HTMLUListElement>(null);
  const menuId = useId();
  const actionable = () => [...(menu.current?.querySelectorAll<HTMLElement>(ACTIONABLE) ?? [])];

  useEffect(() => {
    if (!open) return;
    actionable()[0]?.focus();
    const onDown = (e: MouseEvent) => {
      const t = e.target as Node;
      if (!menu.current?.contains(t) && !button.current?.contains(t)) setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [open]);

  const close = (focusTrigger: boolean) => {
    setOpen(false);
    if (focusTrigger) button.current?.focus();
  };

  const onMenuKeyDown = (e: KeyboardEvent<HTMLUListElement>) => {
    const list = actionable();
    const i = list.indexOf(document.activeElement as HTMLElement);
    const move = (j: number) => {
      e.preventDefault();
      list[(j + list.length) % list.length]?.focus();
    };
    switch (e.key) {
      case 'ArrowDown': move(i + 1); break;
      case 'ArrowUp': move(i - 1); break;
      case 'Home': move(0); break;
      case 'End': move(list.length - 1); break;
      case 'Escape':
        e.preventDefault();
        e.stopPropagation();
        close(true);
        break;
      case 'Tab': close(false); break;
      default: break;
    }
  };

  return (
    <div className="menu-anchor">
      <Tooltip label={label}>
        <button
          ref={button}
          type="button"
          className="menu-trigger"
          aria-label={label}
          aria-haspopup="menu"
          aria-expanded={open}
          aria-controls={open ? menuId : undefined}
          onClick={() => setOpen((o) => !o)}
          onKeyDown={(e) => {
            if (e.key === 'ArrowDown') {
              e.preventDefault();
              setOpen(true);
            }
          }}
        >
          {trigger}
        </button>
      </Tooltip>
      {open && (
        <ul id={menuId} ref={menu} role="menu" aria-label={label} className={`menu menu--${align}`} onKeyDown={onMenuKeyDown}>
          {items.map((it) => {
            switch (it.kind) {
              case 'separator':
                return <li key={it.id} role="separator" className="menu__separator" />;
              case 'static':
                return <li key={it.id} role="none" className="menu__static">{it.content}</li>;
              case 'item':
                return (
                  <li key={it.id} role="none">
                    <button type="button" role="menuitem" tabIndex={-1} className="menu__item" onClick={() => { close(true); it.onSelect(); }}>
                      {it.icon && <Icon name={it.icon} size={16} />}
                      <span>{it.label}</span>
                    </button>
                  </li>
                );
              case 'checkbox':
                return (
                  <li key={it.id} role="none">
                    <button type="button" role="menuitemcheckbox" aria-checked={it.checked} tabIndex={-1} className="menu__item" onClick={() => { it.onChange(!it.checked); close(true); }}>
                      <span>{it.label}</span>
                      <span className="switch" data-on={it.checked} aria-hidden="true"><span className="switch__thumb" /></span>
                    </button>
                  </li>
                );
            }
          })}
        </ul>
      )}
    </div>
  );
}
```

Create `frontend/src/components/Toggle.tsx`:

```tsx
import { useId } from 'react';
import './components.css';

export interface ToggleProps { label: string; checked: boolean; onChange: (v: boolean) => void; description?: string; disabled?: boolean }

export function Toggle({ label, checked, onChange, description, disabled }: ToggleProps) {
  const id = useId();
  return (
    <div className="toggle-row">
      <span className="toggle-row__text">
        <span id={id} className="toggle-row__label">{label}</span>
        {description && <span className="toggle-row__desc muted">{description}</span>}
      </span>
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        aria-labelledby={id}
        disabled={disabled}
        className="switch"
        data-on={checked}
        onClick={() => onChange(!checked)}
      >
        <span className="switch__thumb" />
      </button>
    </div>
  );
}
```

Create `frontend/src/components/TextField.tsx`:

```tsx
import { useId, type ComponentPropsWithRef } from 'react';
import './components.css';

export interface TextFieldProps extends Omit<ComponentPropsWithRef<'input'>, 'id'> { label: string; error?: string | null; hint?: string }

export function TextField({ label, error, hint, className, ...input }: TextFieldProps) {
  const id = useId();
  const hintId = `${id}-hint`;
  const errorId = `${id}-error`;
  const describedBy = [hint ? hintId : '', error ? errorId : ''].filter(Boolean).join(' ') || undefined;
  return (
    <div className={['field', error ? 'field--error' : '', className].filter(Boolean).join(' ')}>
      <label htmlFor={id} className="field__label">{label}</label>
      <input id={id} className="field__input" aria-invalid={error ? true : undefined} aria-describedby={describedBy} {...input} />
      {hint && <span id={hintId} className="field__hint muted">{hint}</span>}
      {error && <span id={errorId} className="field__error">{error}</span>}
    </div>
  );
}
```

Create `frontend/src/components/SelectField.tsx`:

```tsx
import { useId, type ComponentPropsWithRef } from 'react';
import './components.css';

export interface SelectFieldProps extends Omit<ComponentPropsWithRef<'select'>, 'id' | 'children'> {
  label: string;
  options: { value: string; label: string }[];
  error?: string | null;
}

export function SelectField({ label, options, error, className, ...select }: SelectFieldProps) {
  const id = useId();
  const errorId = `${id}-error`;
  return (
    <div className={['field', error ? 'field--error' : '', className].filter(Boolean).join(' ')}>
      <label htmlFor={id} className="field__label">{label}</label>
      <select id={id} className="field__input" aria-invalid={error ? true : undefined} aria-describedby={error ? errorId : undefined} {...select}>
        {options.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
      </select>
      {error && <span id={errorId} className="field__error">{error}</span>}
    </div>
  );
}
```

Create `frontend/src/components/Skeleton.tsx`:

```tsx
import './components.css';

export function Skeleton({ rows = 3, label }: { rows?: number; label: string }) {
  return (
    <div role="status" aria-label={label} className="skeleton">
      {Array.from({ length: rows }, (_, i) => <div key={i} className="skeleton__row" />)}
    </div>
  );
}
```

Create `frontend/src/components/components.css`:

```css
.btn {
  display: inline-flex; align-items: center; gap: var(--space-2); padding: var(--space-2) var(--space-4);
  border-radius: var(--radius-1); border: 1px solid var(--border); background: var(--surface); cursor: pointer; white-space: nowrap;
}
.btn:disabled { opacity: 0.55; cursor: not-allowed; }
.btn--primary { background: var(--accent); border-color: var(--accent); color: var(--on-accent); }
.btn--danger { background: var(--surface); border-color: var(--down); color: var(--down); }
.btn--ghost { background: transparent; border-color: transparent; }
.btn:not(:disabled):hover { background: var(--surface-2); }
.btn--primary:not(:disabled):hover { background: var(--accent); filter: brightness(1.08); }

.icon-btn {
  display: inline-flex; align-items: center; justify-content: center; width: 32px; height: 32px; padding: 0;
  border: 1px solid transparent; border-radius: var(--radius-1); background: transparent; cursor: pointer; color: var(--text);
}
.icon-btn:not(:disabled):hover { background: var(--surface-2); }
.icon-btn:disabled { opacity: 0.55; cursor: not-allowed; }

.tooltip-anchor { position: relative; display: inline-flex; }
.tooltip {
  position: absolute; z-index: var(--z-tooltip); left: 50%; top: calc(100% + 6px); transform: translateX(-50%);
  max-width: 320px; width: max-content; padding: var(--space-1) var(--space-2); border-radius: var(--radius-1);
  background: var(--text); color: var(--bg); font-size: var(--fs-xs); white-space: pre-line; pointer-events: none;
}
.tooltip-anchor--top .tooltip { top: auto; bottom: calc(100% + 6px); }
.tooltip-anchor--right .tooltip { left: calc(100% + 6px); top: 50%; transform: translateY(-50%); }

.dialog-overlay {
  position: fixed; inset: 0; z-index: var(--z-dialog); display: flex; align-items: center; justify-content: center;
  background: var(--overlay);
}
.dialog {
  display: flex; flex-direction: column; max-height: calc(100vh - 64px); background: var(--bg); color: var(--text);
  border: 1px solid var(--border); border-radius: var(--radius-2); box-shadow: var(--shadow);
}
.dialog--sm { width: 420px; } .dialog--md { width: 560px; } .dialog--lg { width: 760px; }
.dialog__header { display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); padding: var(--space-4) var(--space-4) var(--space-2); }
.dialog__title { font-size: var(--fs-lg); }
.dialog__description { padding: 0 var(--space-4); color: var(--text-muted); }
.dialog__body { padding: var(--space-2) var(--space-4) var(--space-4); overflow: auto; display: flex; flex-direction: column; gap: var(--space-3); }
.dialog__footer { display: flex; justify-content: flex-end; gap: var(--space-2); padding: var(--space-3) var(--space-4); border-top: 1px solid var(--border); }

.menu-anchor { position: relative; display: inline-flex; }
.menu-trigger { display: inline-flex; align-items: center; border: none; background: transparent; padding: 0; cursor: pointer; border-radius: var(--radius-pill); }
.menu {
  position: absolute; z-index: var(--z-menu); top: calc(100% + 6px); min-width: 220px; margin: 0; padding: var(--space-1) 0; list-style: none;
  background: var(--bg); border: 1px solid var(--border); border-radius: var(--radius-2); box-shadow: var(--shadow);
}
.menu--end { right: 0; } .menu--start { left: 0; }
.menu__item {
  display: flex; align-items: center; gap: var(--space-2); width: 100%; padding: var(--space-2) var(--space-3);
  border: none; background: transparent; text-align: left; cursor: pointer; justify-content: flex-start;
}
.menu__item[role="menuitemcheckbox"] { justify-content: space-between; }
.menu__item:focus, .menu__item:hover { background: var(--surface-2); outline: none; }
.menu__separator { height: 1px; margin: var(--space-1) 0; background: var(--border); }
.menu__static { padding: var(--space-2) var(--space-3); }

.switch {
  position: relative; flex: none; width: 36px; height: 20px; padding: 0; border-radius: var(--radius-pill);
  border: 1px solid var(--border); background: var(--surface-2); cursor: pointer;
}
.switch[data-on="true"] { background: var(--accent); border-color: var(--accent); }
.switch__thumb {
  position: absolute; top: 2px; left: 2px; width: 14px; height: 14px; border-radius: 50%; background: var(--bg);
  transition: transform var(--motion-fast);
}
.switch[data-on="true"] .switch__thumb { transform: translateX(16px); }
.toggle-row { display: flex; align-items: center; justify-content: space-between; gap: var(--space-4); padding: var(--space-2) 0; }
.toggle-row__text { display: flex; flex-direction: column; }

.field { display: flex; flex-direction: column; gap: var(--space-1); }
.field__label { font-weight: 600; font-size: var(--fs-sm); }
.field__input {
  padding: var(--space-2) var(--space-3); border: 1px solid var(--border); border-radius: var(--radius-1);
  background: var(--bg); min-width: 0;
}
.field--error .field__input { border-color: var(--down); }
.field__hint { font-size: var(--fs-xs); }
.field__error { color: var(--down); font-size: var(--fs-sm); }

.skeleton { display: flex; flex-direction: column; gap: var(--space-2); }
.skeleton__row { height: 20px; border-radius: var(--radius-1); background: var(--surface-2); }
```

Overwrite `frontend/src/app/Sidebar.tsx` (collapsed entries get tooltips; the toggle becomes an `IconButton`):

```tsx
import { useRef, type KeyboardEvent } from 'react';
import { NavLink } from 'react-router-dom';
import { Icon } from '../components/Icon';
import { IconButton } from '../components/IconButton';
import { Tooltip } from '../components/Tooltip';
import { shellCopy } from '../i18n/copy';
import type { NavEntry } from './features';

export interface SidebarProps { entries: readonly NavEntry[]; collapsed: boolean; onToggleCollapsed: () => void }

export function Sidebar({ entries, collapsed, onToggleCollapsed }: SidebarProps) {
  const links = useRef<(HTMLAnchorElement | null)[]>([]);

  const onKeyDown = (e: KeyboardEvent<HTMLAnchorElement>, index: number) => {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
    e.preventDefault();
    const n = entries.length;
    links.current[(index + (e.key === 'ArrowDown' ? 1 : -1) + n) % n]?.focus();
  };

  return (
    <nav className={collapsed ? 'sidebar sidebar--collapsed' : 'sidebar'} aria-label={shellCopy.sidebarLabel}>
      <ul>
        {entries.map((entry, i) => {
          const link = (
            <NavLink
              ref={(el) => { links.current[i] = el; }}
              to={entry.path}
              className="sidebar__link"
              aria-label={collapsed ? entry.label : undefined}
              onKeyDown={(e) => onKeyDown(e, i)}
            >
              <Icon name={entry.icon} />
              <span className="sidebar__label">{entry.label}</span>
            </NavLink>
          );
          return <li key={entry.id}>{collapsed ? <Tooltip label={entry.label} placement="right">{link}</Tooltip> : link}</li>;
        })}
      </ul>
      <div className="sidebar__collapse">
        <IconButton
          label={collapsed ? shellCopy.expandSidebar : shellCopy.collapseSidebar}
          icon={collapsed ? 'chevronRight' : 'chevronLeft'}
          onClick={onToggleCollapsed}
          aria-expanded={!collapsed}
        />
      </div>
    </nav>
  );
}
```

- [ ] **Step 4: Run the suite to verify it passes**

Run: `cd frontend && npx vitest run && npm run typecheck`
Expected: PASS (18 test files), `tsc` exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): accessible primitives — tooltip, dialog, menu, fields (P0-4 Task 3)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
