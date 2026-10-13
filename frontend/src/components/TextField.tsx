import { useId, type ComponentPropsWithRef } from 'react';
import './components.css';

export interface TextFieldProps extends Omit<ComponentPropsWithRef<'input'>, 'id'> { label: string; error?: string | null; hint?: string }

export function TextField({
  label, error, hint, className, 'aria-describedby': extraDescribedBy, 'aria-invalid': extraInvalid, ...input
}: TextFieldProps) {
  const id = useId();
  const hintId = `${id}-hint`;
  const errorId = `${id}-error`;
  const describedBy = [extraDescribedBy ?? '', hint ? hintId : '', error ? errorId : ''].filter(Boolean).join(' ') || undefined;
  return (
    <div className={['field', error ? 'field--error' : '', className].filter(Boolean).join(' ')}>
      <label htmlFor={id} className="field__label">{label}</label>
      {/* aria-invalid ORs the inline error with the caller's flag (e.g. a form-level refusal); the spread cannot drop it. */}
      <input {...input} id={id} className="field__input" aria-invalid={error || (extraInvalid && extraInvalid !== 'false') ? true : undefined} aria-describedby={describedBy} />
      {hint && <span id={hintId} className="field__hint muted">{hint}</span>}
      {error && <span id={errorId} className="field__error" role="alert">{error}</span>}
    </div>
  );
}
