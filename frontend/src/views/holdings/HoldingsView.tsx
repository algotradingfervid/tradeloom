import { useMemo, useState } from 'react';
import { DataTable, type Column } from '../../components/DataTable';
import { filterRows } from '../../components/filterRows';
import { ListBody } from '../../components/ListBody';
import { ListPage } from '../../components/ListPage';
import { InstrumentName, Signed } from '../../components/Values';
import { holdingsCopy as copy } from '../../i18n/copy';
import { formatMoney, formatNumber, formatPrice } from '../../i18n/format';
import { holdingRow, liveLtp, rupees, summarizeHoldings, type LiveHolding } from '../../portfolio/calc';
import { listStatusOf, usePortfolioResource } from '../../store/portfolio';
import { useSessionStore } from '../../store/session';
import { useTicksStore } from '../../store/ticks';

const columns: Column<LiveHolding>[] = [
  {
    id: 'symbol', header: copy.cols.symbol, width: '24%', title: (r) => r.row.symbol,
    cell: (r) => (
      <>
        <InstrumentName name={r.row.symbol} exchange={r.row.exchange} />
        {/* P0-3 Ruling 38: kept, read-only, flagged in text and counted in the totals. */}
        {r.row.outOfScope && <span className="chip" title={copy.outOfScopeHint}>{copy.outOfScope}</span>}
      </>
    ),
  },
  { id: 'qty', header: copy.cols.qty, align: 'end', cell: (r) => formatNumber(r.row.qty, 0) },
  { id: 'avg', header: copy.cols.avg, align: 'end', cell: (r) => formatPrice(rupees(r.row.avgPaise)) },
  { id: 'ltp', header: copy.cols.ltp, align: 'end', cell: (r) => formatPrice(r.ltp) },
  { id: 'invested', header: copy.cols.invested, align: 'end', cell: (r) => formatMoney(r.invested) },
  { id: 'current', header: copy.cols.current, align: 'end', cell: (r) => formatMoney(r.current) },
  { id: 'pnl', header: copy.cols.pnl, align: 'end', cell: (r) => <Signed value={r.pnl} kind="money" /> },
  { id: 'net', header: copy.cols.net, align: 'end', cell: (r) => <Signed value={r.netChangePct} kind="percent" /> },
  { id: 'day', header: copy.cols.day, align: 'end', cell: (r) => <Signed value={r.dayChange} kind="money" /> },
];

export function HoldingsView() {
  const { resource, reload, refresh } = usePortfolioResource('holdings');
  const latest = useTicksStore((s) => s.latest);
  const sessionStartMs = useSessionStore((s) => s.session?.loggedInAtMs ?? 0);
  const [query, setQuery] = useState('');
  const rows = useMemo(
    () => resource.data.rows.map((h) => holdingRow(h, liveLtp(latest[h.token], sessionStartMs))),
    [resource.data, latest, sessionStartMs],
  );
  const visible = filterRows(rows, query, (r) => [r.row.symbol, r.row.exchange]);
  const totals = summarizeHoldings(resource.data.summary, rows);

  return (
    <ListPage
      title={copy.title}
      description={copy.description}
      onRefresh={() => void refresh()}
      refreshing={resource.loading}
      search={{ value: query, onChange: setQuery, matches: visible.length }}
    >
      <ListBody
        status={listStatusOf(resource, rows.length === 0)}
        error={resource.error}
        onRetry={() => void reload()}
        empty={{ message: copy.empty, action: { label: copy.refresh, onSelect: () => void refresh() } }}
        loadingLabel={copy.loading}
      >
        <DataTable
          caption={copy.title}
          columns={columns}
          rows={visible}
          rowKey={(r) => String(r.row.token)}
          rowLabel={(r) => r.row.symbol}
          footer={
            <tr>
              <td>{copy.total}</td>
              <td /><td /><td />
              <td className="num">{formatMoney(totals.investment)}</td>
              <td className="num">{formatMoney(totals.currentValue)}</td>
              <td className="num"><Signed value={totals.pnl} kind="money" /></td>
              <td className="num"><Signed value={totals.pnlPct} kind="percent" /></td>
              <td className="num"><Signed value={totals.dayChange} kind="money" /></td>
            </tr>
          }
        />
      </ListBody>
    </ListPage>
  );
}
