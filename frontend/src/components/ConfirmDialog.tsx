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
