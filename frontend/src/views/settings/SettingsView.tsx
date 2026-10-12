import type { ComponentType } from 'react';
import { NavLink, useParams } from 'react-router-dom';
import { BUILD_PHASE } from '../../app/features';
import { settingsViewCopy as copy } from '../../i18n/copy';
import { NotFoundView } from '../NotFoundView';
import { AboutGroup } from './AboutGroup';
import { AppearanceGroup } from './AppearanceGroup';
import { CalendarGroup } from './CalendarGroup';
import { ChartGroup } from './ChartGroup';
import { DataGroup } from './DataGroup';
import { DiagnosticsGroup } from './DiagnosticsGroup';
import { SETTINGS_GROUPS, type SettingsGroupDef, type SettingsGroupId } from './groups';
import { KiteGroup } from './KiteGroup';
import { ShortcutsGroup } from './ShortcutsGroup';
import './settings.css';

/** Later tasks add `calendar`, `data`, `shortcuts`, `diagnostics`; later phases add theirs. */
export const GROUP_COMPONENTS: Partial<Record<SettingsGroupId, ComponentType>> = {
  kite: KiteGroup,
  appearance: AppearanceGroup,
  chart: ChartGroup,
  calendar: CalendarGroup,
  data: DataGroup,
  shortcuts: ShortcutsGroup,
  diagnostics: DiagnosticsGroup,
  about: AboutGroup,
};

export function visibleSettingsGroups(): SettingsGroupDef[] {
  return SETTINGS_GROUPS.filter((g) => BUILD_PHASE >= g.phase && GROUP_COMPONENTS[g.id] !== undefined);
}

export function SettingsView() {
  const { group } = useParams();
  const groups = visibleSettingsGroups();
  const current = groups.find((g) => g.id === group);
  const Group = current ? GROUP_COMPONENTS[current.id] : undefined;
  if (!current || !Group) return <NotFoundView />;
  return (
    <div className="settings">
      <nav className="settings__nav" aria-label={copy.nav}>
        <ul>
          {groups.map((g) => (
            <li key={g.id}><NavLink to={`/settings/${g.id}`}>{g.label}</NavLink></li>
          ))}
        </ul>
      </nav>
      <section className="settings__panel" aria-labelledby="settings-title">
        <h1 id="settings-title">{current.label}</h1>
        <Group />
      </section>
    </div>
  );
}
