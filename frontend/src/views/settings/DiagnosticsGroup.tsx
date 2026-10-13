import { useState } from 'react';
import { SupportActions } from '../../app/SupportActions';
import { SelectField } from '../../components/SelectField';
import { diagnosticsCopy as copy } from '../../i18n/copy';
import { useSetting } from '../../store/settings';
import { SettingSection } from './SettingRow';

export function DiagnosticsGroup() {
  const [level, setLevel] = useSetting<string>('log.level', 'info');
  const [levelError, setLevelError] = useState<string | null>(null);
  return (
    <SettingSection title={copy.title}>
      <SelectField
        label={copy.level}
        options={[...copy.levels]}
        value={level}
        error={levelError}
        onChange={async (e) => setLevelError((await setLevel(e.target.value))?.reason ?? null)}
      />
      <p className="muted">{copy.levelHint}</p>
      <SupportActions />
      <p className="muted">{copy.bundleHint}</p>
    </SettingSection>
  );
}
