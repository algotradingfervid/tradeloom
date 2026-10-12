import { useId } from 'react';
import './components.css';

export interface ToggleProps { label: string; checked: boolean; onChange: (v: boolean) => void; description?: string; disabled?: boolean }

export function Toggle({ label, checked, onChange, description, disabled }: ToggleProps) {
  const id = useId();
  const descId = `${id}-desc`;
  return (
    <div className="toggle-row">
      <span className="toggle-row__text">
        <span id={id} className="toggle-row__label">{label}</span>
        {description && <span id={descId} className="toggle-row__desc muted">{description}</span>}
      </span>
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        aria-labelledby={id}
        aria-describedby={description ? descId : undefined}
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
