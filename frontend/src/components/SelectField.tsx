import { useId, type ComponentPropsWithRef } from 'react';
import './components.css';

export interface SelectFieldProps extends Omit<ComponentPropsWithRef<'select'>, 'id' | 'children'> {
  label: string;
  options: { value: string; label: string }[];
  error?: string | null;
}

export function SelectField({ label, options, error, className, 'aria-describedby': extraDescribedBy, ...select }: SelectFieldProps) {
  const id = useId();
  const errorId = `${id}-error`;
  return (
    <div className={['field', error ? 'field--error' : '', className].filter(Boolean).join(' ')}>
      <label htmlFor={id} className="field__label">{label}</label>
      <select id={id} className="field__input" aria-invalid={error ? true : undefined} aria-describedby={[extraDescribedBy ?? '', error ? errorId : ''].filter(Boolean).join(' ') || undefined} {...select}>
        {options.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
      </select>
      {error && <span id={errorId} className="field__error" role="alert">{error}</span>}
    </div>
  );
}
