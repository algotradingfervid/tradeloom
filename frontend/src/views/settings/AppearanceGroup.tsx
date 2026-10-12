import { useTheme } from '../../app/theme';
import { Toggle } from '../../components/Toggle';
import type { ThemeName } from '../../design/tokens';
import { appearanceCopy as copy } from '../../i18n/copy';
import { useSetting } from '../../store/settings';
import { SettingSection } from './SettingRow';

export function AppearanceGroup() {
  const { theme } = useTheme();
  const [, setTheme] = useSetting<ThemeName>('appearance.theme', 'light');
  const [follow, setFollow] = useSetting<boolean>('appearance.follow_system', false);
  const [collapsed, setCollapsed] = useSetting<boolean>('appearance.sidebar_collapsed', false);
  return (
    <SettingSection title={copy.title}>
      <Toggle label={copy.darkTheme} checked={theme === 'dark'} disabled={follow} description={follow ? copy.followingHint : undefined} onChange={(on) => void setTheme(on ? 'dark' : 'light')} />
      <Toggle label={copy.followSystem} checked={follow} description={copy.followSystemHint} onChange={(on) => void setFollow(on)} />
      <Toggle label={copy.sidebarCollapsed} checked={collapsed} onChange={(on) => void setCollapsed(on)} />
    </SettingSection>
  );
}
