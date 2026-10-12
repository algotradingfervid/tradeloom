import { useMemo, useState } from 'react';
import type { OrderRow, OrderSection } from '../../api/backend';
import { BackgroundErrorNotice } from '../../components/BackgroundErrorNotice';
import { DataTable, type Column } from '../../components/DataTable';
import { filterRows } from '../../components/filterRows';
import { ListBody } from '../../components/ListBody';
import { ListPage } from '../../components/ListPage';
import { InstrumentName } from '../../components/Values';
import { ordersCopy as copy, portfolioCopy } from '../../i18n/copy';
import { DASH, formatNumber, formatPrice, formatTime } from '../../i18n/format';
import { orderStatusLabel, rupees } from '../../portfolio/calc';
import { listStatusOf, usePortfolioResource } from '../../store/portfolio';
import { useStatusStore } from '../../store/status';

export const ORDER_POLL_MS = 30_000;
const SECTIONS: OrderSection[] = ['open', 'executed', 'rejected', 'cancelled'];

const sourceLabel = (o: OrderRow) => copy.sources[o.source];

/** Go `orders.BuildBook` order: newest first, then the higher order ID. */
const newestFirst = (a: OrderRow, b: OrderRow) =>
  a.timeMs !== b.timeMs ? b.timeMs - a.timeMs : a.orderId < b.orderId ? 1 : a.orderId > b.orderId ? -1 : 0;

const columns: Column<OrderRow>[] = [
  { id: 'time', header: copy.cols.time, width: '9%', cell: (o) => (o.timeMs > 0 ? formatTime(o.timeMs, true) : DASH) },
  // The side is always spelled out; the tone only adds colour.
  { id: 'type', header: copy.cols.type, width: '7%', cell: (o) => <span className={o.side === 'Buy' ? 'tone-up' : 'tone-down'}>{o.side}</span> },
  {
    // P0-3 Ruling 38 / Ruling 96 I2: out-of-scope orders are kept, read-only and flagged; no Column.title here.
    id: 'instrument', header: copy.cols.instrument, width: '22%',
    cell: (o) => (
      <InstrumentName name={o.symbol} exchange={o.exchange} flag={o.outOfScope ? copy.outOfScope : undefined} flagHint={copy.outOfScopeHint} />
    ),
  },
  { id: 'product', header: copy.cols.product, width: '8%', cell: (o) => o.product },
  { id: 'qty', header: copy.cols.qty, align: 'end', width: '9%', cell: (o) => `${formatNumber(o.filledQty, 0)}/${formatNumber(o.qty, 0)}` },
  { id: 'avg', header: copy.cols.avg, align: 'end', width: '11%', cell: (o) => (o.filledQty > 0 ? formatPrice(rupees(o.avgPricePaise)) : DASH) },
  {
    id: 'status', header: copy.cols.status, width: '16%',
    cell: (o) => (
      <>
        {orderStatusLabel(o.status)}
        {o.statusMessage && <span className="order-status__reason">{o.statusMessage}</span>}
      </>
    ),
  },
  { id: 'source', header: copy.cols.source, cell: (o) => sourceLabel(o), title: (o) => (o.tag ? `${sourceLabel(o)} (${o.tag})` : sourceLabel(o)) },
];

export function OrderBookView() {
  // Go re-syncs the book every 30 s and pushes "orders.changed"; while the feed is down the view also re-reads the cache.
  const feedLive = useStatusStore((s) => s.feed.state === 'live');
  const { resource, reload, refresh, retryInBackground } = usePortfolioResource('orders', { pollMs: feedLive ? undefined : ORDER_POLL_MS });
  const [query, setQuery] = useState('');
  const all = useMemo(() => [...resource.data.orders].sort(newestFirst), [resource.data]);
  const visible = filterRows(all, query, (o) => [o.symbol, o.orderId, o.tag, sourceLabel(o)]);

  return (
    <ListPage
      title={copy.title}
      description={copy.description}
      onRefresh={() => void refresh()}
      refreshing={resource.loading}
      search={{ value: query, onChange: setQuery, placeholder: copy.searchPlaceholder, matches: visible.length }}
    >
      <BackgroundErrorNotice error={resource.backgroundError} message={portfolioCopy.staleNotice} onRetry={() => void retryInBackground()} />
      <ListBody status={listStatusOf(resource, all.length === 0)} error={resource.error} onRetry={() => void reload()} empty={{ message: copy.empty }} loadingLabel={copy.loading}>
        {SECTIONS.map((section) => {
          const rows = visible.filter((o) => o.section === section);
          const headingId = `orders-${section}`;
          return (
            <section key={section} className="list-section" aria-labelledby={headingId}>
              <h2 id={headingId}>{`${copy.sections[section]} (${rows.length})`}</h2>
              {rows.length === 0 ? (
                <p className="muted">{copy.sectionEmpty[section]}</p>
              ) : (
                <DataTable caption={copy.sections[section]} columns={columns} rows={rows} rowKey={(o) => o.orderId} rowLabel={(o) => `${o.symbol} ${o.orderId}`} />
              )}
            </section>
          );
        })}
      </ListBody>
    </ListPage>
  );
}
