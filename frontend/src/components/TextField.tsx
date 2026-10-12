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
