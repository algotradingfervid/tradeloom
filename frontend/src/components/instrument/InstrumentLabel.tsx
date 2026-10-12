import { instrumentCopy as copy } from '../../i18n/copy';
import { displayName, exchangeTag, isFuture, isInScope, type LabelInstrument } from '../../instruments/naming';
import './instrument.css';

/**
 * FR-WATCH-013/014: the only component that renders an instrument's name and exchange tag. Only the symbol truncates;
 * the F badge, the tag and the "Also on NSE" flag keep their width and are repeated in the title (Ruling 96 I2).
 * NFR-15: an out-of-scope instrument renders nothing.
 */
export function InstrumentLabel({ instrument, showName = false }: {
  instrument: LabelInstrument & { alsoOnNse?: boolean };
  showName?: boolean;
}) {
  if (!isInScope(instrument)) return null;
  const tag = exchangeTag(instrument);
  const name = displayName(instrument);
  const future = isFuture(instrument);
  const alsoOnNse = instrument.alsoOnNse === true;
  const title = [name, tag, future && copy.future, alsoOnNse && copy.alsoOnNse, showName && instrument.name]
    .filter(Boolean)
    .join(' · ');
  return (
    <span className="instrument-label" data-instrument-token={instrument.token} data-exchange-tag={tag} title={title}>
      <span className="instrument-label__symbol" data-instrument-name>{name}</span>
      {future && <span className="instrument-label__badge" role="img" aria-label={copy.future}>F</span>}
      <span className="instrument-label__tag">{tag}</span>
      {alsoOnNse && <span className="instrument-label__flag" aria-description={copy.alsoOnNseHint}>{copy.alsoOnNse}</span>}
      {showName && <span className="instrument-label__company">{instrument.name}</span>}
    </span>
  );
}
