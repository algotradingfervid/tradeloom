import { useState } from 'react';
import { SelectField } from '../../components/SelectField';
import { chartSettingsCopy as copy } from '../../i18n/copy';
import { useSetting } from '../../store/settings';
import { SettingSection } from './SettingRow';

/** Kite interval ids (FR-DATA-002 native intervals). P0-5 reads chart.default_timeframe with these ids. */
export const TIMEFRAME_OPTIONS = [
  { value: 'minute', label: '1 minute' },
  { value: '3minute', label: '3 minutes' },
  { value: '5minute', label: '5 minutes' },
  { value: '10minute', label: '10 minutes' },
  { value: '15minute', label: '15 minutes' },
  { value: '30minute', label: '30 minutes' },
  { value: '60minute', label: '1 hour' },
  { value: 'day', label: '1 day' },
];

export function ChartGroup() {
  const [timeframe, setTimeframe] = useSetting<string>('chart.default_timeframe', '60minute');
  const [error, setError] = useState<string | null>(null);
  return (
    <SettingSection title={copy.title}>
      <SelectField
        label={copy.timeframe}
        options={TIMEFRAME_OPTIONS}
        value={timeframe}
        error={error}
        onChange={async (e) => setError((await setTimeframe(e.target.value))?.reason ?? null)}
      />
      <p className="muted">{copy.hint}</p>
    </SettingSection>
  );
}
