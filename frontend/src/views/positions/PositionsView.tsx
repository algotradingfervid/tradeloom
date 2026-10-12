import { useMemo, useState } from 'react';
import { DataTable, type Column } from '../../components/DataTable';
import { filterRows } from '../../components/filterRows';
import { ListBody } from '../../components/ListBody';
import { ListPage } from '../../components/ListPage';
import { Tabs } from '../../components/Tabs';
import { InstrumentName, Signed } from '../../components/Values';
import { positionsCopy as copy } from '../../i18n/copy';
import { formatNumber, formatPrice } from '../../i18n/format';
import { livePosition, liveLtp, positionNetChangePct, positionsTotalPnl, rupees, type LivePosition } from '../../portfolio/calc';
import { listStatusOf, usePortfolioResource } from '../../store/portfolio';
import { useTicksStore } from '../../store/ticks';

type PositionTab = 'net' | 'day';
interface PositionView extends LivePosition { netPct: number | null }

const columns: Column<PositionView>[] = [
  { id: 'product', header: copy.cols.product, width: '9%', cell: (r) => r.row.product },
  {
    // P0-3 Ruling 38 / Ruling 96 I2: out-of-scope rows are kept, read-only, flagged and counted in the total; no
    // Column.title here, so only the name truncates and the flag is never clipped.
    id: 'symbol', header: copy.cols.symbol, width: '30%',
    cell: (r) => (
      <InstrumentName
        name={r.row.symbol}
        exchange={r.row.exchange}
        flag={r.row.outOfScope ? copy.outOfScope : undefined}
        flagHint={copy.outOfScopeHint}
      />
    ),
  },
  { id: 'qty', header: copy.cols.qty, align: 'end', cell: (r) => formatNumber(r.row.qty, 0) },
  { id: 'avg', header: copy.cols.avg, align: 'end', cell: (r) => formatPrice(rupees(r.row.avgPaise)) },
  { id: 'ltp', header: copy.cols.ltp, align: 'end', cell: (r) => formatPrice(r.ltp) },
  { id: 'pnl', header: copy.cols.pnl, align: 'end', cell: (r) => <Signed value={r.pnl} kind="money" /> },
  { id: 'net', header: copy.cols.net, align: 'end', cell: (r) => <Signed value={r.netPct} kind="percent" /> },
];

export function PositionsView() {
  const { resource, reload, refresh } = usePortfolioResource('positions');
  const latest = useTicksStore((s) => s.latest);
  const [tab, setTab] = useState<PositionTab>('net');
  const [query, setQuery] = useState('');

  const rows = useMemo<PositionView[]>(
    () =>
      resource.data[tab]
        .map((p) => {
          const live = livePosition(p, liveLtp(latest[p.token]));
          return { ...live, netPct: positionNetChangePct(p, live.ltp) };
        })
        // Open rows first; the sort is stable, so Go's order is kept within each group.
        .sort((a, b) => Number(b.open) - Number(a.open)),
    [resource.data, tab, latest],
  );
  const visible = filterRows(rows, query, (r) => [r.row.symbol, r.row.exchange, r.row.product]);
  const total = positionsTotalPnl(tab === 'net' ? resource.data.netTotalPnlPaise : resource.data.dayTotalPnlPaise, rows);

  return (
    <ListPage
      title={copy.title}
      description={copy.description}
      onRefresh={() => void refresh()}
      refreshing={resource.loading}
      search={{ value: query, onChange: setQuery, matches: visible.length }}
      tabs={<Tabs label={copy.tabs} tabs={[{ id: 'net', label: copy.net }, { id: 'day', label: copy.day }]} value={tab} onChange={(id) => setTab(id as PositionTab)} />}
    >
      <div role="tabpanel" aria-labelledby={`tab-${tab}`}>
        <ListBody status={listStatusOf(resource, rows.length === 0)} error={resource.error} onRetry={() => void reload()} empty={{ message: copy.empty }} loadingLabel={copy.loading}>
          <DataTable
            caption={`${copy.title} — ${tab === 'net' ? copy.net : copy.day}`}
            columns={columns}
            rows={visible}
            rowKey={(r) => r.row.key}
            rowLabel={(r) => r.row.symbol}
            rowClassName={(r) => (r.open ? undefined : 'row--muted')}
            footer={
              <tr>
                <td colSpan={5}>{copy.totalPnl}</td>
                <td className="num"><Signed value={total} kind="money" /></td>
                <td />
              </tr>
            }
          />
        </ListBody>
      </div>
    </ListPage>
  );
}
