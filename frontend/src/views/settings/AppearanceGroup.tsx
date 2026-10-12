import { useTheme } from '../../app/theme';
import type { BackendError } from '../../api/errors';
import { Toggle } from '../../components/Toggle';
import type { ThemeName } from '../../design/tokens';
import { appearanceCopy as copy, profileMenuCopy } from '../../i18n/copy';
import { useSetting } from '../../store/settings';
import { pushToast } from '../../store/toasts';
import { SettingSection } from './SettingRow';

/** writeSetting already rolled the value back on refusal; tell the user why (Ruling 97 I3). */
async function reportRefusal(write: Promise<BackendError | null>, message: (reason: string) => string): Promise<void> {
  const err = await write;
  if (err) pushToast({ level: 'error', message: message(err.reason) });
}

export function AppearanceGroup() {
  const { theme } = useTheme();
  const [, setTheme] = useSetting<ThemeName>('appearance.theme', 'light');
  const [follow, setFollow] = useSetting<boolean>('appearance.follow_system', false);
  const [collapsed, setCollapsed] = useSetting<boolean>('appearance.sidebar_collapsed', false);
  return (
    <SettingSection title={copy.title}>
      <Toggle
        label={copy.darkTheme}
        checked={theme === 'dark'}
        disabled={follow}
        description={follow ? copy.followingHint : undefined}
        onChange={(on) => void reportRefusal(setTheme(on ? 'dark' : 'light'), profileMenuCopy.themeFailed)}
      />
      <Toggle label={copy.followSystem} checked={follow} description={copy.followSystemHint} onChange={(on) => void reportRefusal(setFollow(on), copy.saveFailed)} />
      <Toggle label={copy.sidebarCollapsed} checked={collapsed} onChange={(on) => void reportRefusal(setCollapsed(on), copy.saveFailed)} />
    </SettingSection>
  );
}
