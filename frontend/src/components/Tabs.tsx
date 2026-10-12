import { useId, useRef, type KeyboardEvent } from 'react';
import './list.css';

export interface TabDef { id: string; label: string }

/**
 * Ids of a tab and of the panel it controls, unique per Tabs instance (`useId` prefix). The owner renders one panel
 * whose content follows the selected tab, so every tab controls that same panel (no reference to a missing id).
 */
export const tabIds = (prefix: string, id: string) => ({ tab: `${prefix}-tab-${id}`, panel: `${prefix}-panel` });

/**
 * WAI-ARIA tabs with automatic activation: ArrowLeft/ArrowRight wrap, Home/End jump to the first/last tab. With
 * `idPrefix`, each tab's `aria-controls` names `tabIds(idPrefix, tab.id).panel`, which the owner must render; without
 * it no aria-controls is set (it would point at nothing).
 */
export function Tabs({ label, tabs, value, onChange, idPrefix }: {
  label: string;
  tabs: TabDef[];
  value: string;
  onChange: (id: string) => void;
  /** The owner's `useId()` value, so it can name the panel; tab ids default to an id of this component's own. */
  idPrefix?: string;
}) {
  const ownId = useId();
  const prefix = idPrefix ?? ownId;
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  const onKeyDown = (e: KeyboardEvent<HTMLButtonElement>, i: number) => {
    let j: number;
    if (e.key === 'ArrowRight') j = (i + 1) % tabs.length;
    else if (e.key === 'ArrowLeft') j = (i - 1 + tabs.length) % tabs.length;
    else if (e.key === 'Home') j = 0;
    else if (e.key === 'End') j = tabs.length - 1;
    else return;
    e.preventDefault();
    onChange(tabs[j].id);
    refs.current[j]?.focus();
  };
  return (
    <div role="tablist" aria-label={label} className="tabs">
      {tabs.map((t, i) => {
        const ids = tabIds(prefix, t.id);
        return (
          <button
            key={t.id}
            ref={(el) => { refs.current[i] = el; }}
            type="button"
            role="tab"
            id={ids.tab}
            aria-controls={idPrefix ? ids.panel : undefined}
            aria-selected={t.id === value}
            tabIndex={t.id === value ? 0 : -1}
            onClick={() => onChange(t.id)}
            onKeyDown={(e) => onKeyDown(e, i)}
          >
            {t.label}
          </button>
        );
      })}
    </div>
  );
}
