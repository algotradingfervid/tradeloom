import { instrumentCopy as copy } from '../../i18n/copy';
import { displayName, exchangeTag, isFuture, isInScope, type LabelInstrument } from '../../instruments/naming';
import '../list.css';
import './instrument.css';

const flagTitle = (flag: string, hint: string) => `${flag} — ${hint}`;

/**
 * FR-WATCH-013/014: the only component that renders an instrument's name and exchange tag. Only the symbol truncates;
 * the F badge, the tag chip and the flag chips keep their width, and the title carries the full text in the
 * InstrumentName form "name · flag — hint" (Ruling 96 I2). An out-of-scope instrument is never silently dropped
 * (P0-3 Ruling 38): it shows its name, raw tag and the shared "View only" flag.
 */
export function InstrumentLabel({ instrument, showName = false }: {
  instrument: LabelInstrument & { alsoOnNse?: boolean };
  showName?: boolean;
}) {
  const name = displayName(instrument);
  if (!isInScope(instrument)) {
    return (
      <span
        className="instrument-label"
        data-instrument-token={instrument.token}
        data-out-of-scope="true"
        title={[name, instrument.tag, flagTitle(copy.outOfScope, copy.outOfScopeHint)].join(' · ')}
      >
        <span className="instrument-label__symbol" data-instrument-name>{name}</span>
        <span className="chip">{instrument.tag}</span>
        <span className="chip chip--flag" aria-description={copy.outOfScopeHint}>{copy.outOfScope}</span>
      </span>
    );
  }
  const tag = exchangeTag(instrument);
  const future = isFuture(instrument);
  const alsoOnNse = instrument.alsoOnNse === true;
  const title = [name, tag, future && copy.future, alsoOnNse && flagTitle(copy.alsoOnNse, copy.alsoOnNseHint), showName && instrument.name]
    .filter(Boolean)
    .join(' · ');
  return (
    <span className="instrument-label" data-instrument-token={instrument.token} data-exchange-tag={tag} title={title}>
      <span className="instrument-label__symbol" data-instrument-name>{name}</span>
      {future && <span className="instrument-label__badge" role="img" aria-label={copy.future}>F</span>}
      <span className="chip">{tag}</span>
      {alsoOnNse && <span className="chip chip--flag" aria-description={copy.alsoOnNseHint}>{copy.alsoOnNse}</span>}
      {showName && <span className="instrument-label__company">{instrument.name}</span>}
    </span>
  );
}
