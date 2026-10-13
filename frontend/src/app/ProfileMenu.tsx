import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import type { Profile } from '../api/backend';
import { Icon } from '../components/Icon';
import { MenuButton, type MenuItem } from '../components/MenuButton';
import type { ThemeName } from '../design/tokens';
import { profileMenuCopy as copy } from '../i18n/copy';
import { useAppStore } from '../store/app';
import { useSessionStore } from '../store/session';
import { useSetting } from '../store/settings';
import { pushToast } from '../store/toasts';
import { useTheme } from './theme';
import { useProfile } from './useProfile';

export function Avatar({ profile, size = 32 }: { profile: Profile | null; size?: number }) {
  const [failedUrl, setFailedUrl] = useState<string | null>(null);
  if (!profile) {
    return <span className="avatar" style={{ width: size, height: size }} aria-hidden="true"><Icon name="user" size={size * 0.6} /></span>;
  }
  // Kite's picture is remote: offline, or blocked by a future img-src CSP, it would show a broken image, so a failed
  // load falls back to the initials (Ruling 90 M3). A different URL is tried afresh.
  if (profile.avatarUrl && failedUrl !== profile.avatarUrl) {
    return <img className="avatar" src={profile.avatarUrl} alt="" width={size} height={size} onError={() => setFailedUrl(profile.avatarUrl)} />;
  }
  return <span className="avatar" style={{ width: size, height: size }} aria-hidden="true">{profile.initials}</span>;
}

export function ProfileMenu() {
  const navigate = useNavigate();
  const loggedIn = useSessionStore((s) => s.session?.state === 'ready');
  const { profile, failed, reload } = useProfile();
  const info = useAppStore((s) => s.info);
  const { theme } = useTheme();
  const [, setTheme] = useSetting<ThemeName>('appearance.theme', 'light');
  const [, setFollowSystem] = useSetting<boolean>('appearance.follow_system', false);
  const go = (path: string) => () => void navigate(path);

  // Ruling 90 M1: logged in without a profile (loading or failed) is not "Not logged in".
  const who: MenuItem =
    loggedIn && profile
      ? { kind: 'static', id: 'who', content: (<div className="profile-head"><strong>{profile.name}</strong><span className="muted">{profile.userId}</span></div>) }
      : loggedIn
        ? { kind: 'static', id: 'who', content: <span className="muted">{failed ? copy.profileUnavailable : copy.profilePending}</span> }
        : { kind: 'static', id: 'who', content: <span>{copy.notLoggedIn}</span> };

  // Ruling 90 I2: theme first, then follow_system; each refusal is reported (the write already rolled back).
  const toggleTheme = async (on: boolean) => {
    const themeErr = await setTheme(on ? 'dark' : 'light');
    if (themeErr) {
      pushToast({ level: 'error', message: copy.themeFailed(themeErr.reason) });
      return;
    }
    const followErr = await setFollowSystem(false);
    if (followErr) pushToast({ level: 'error', message: copy.themeFailed(followErr.reason) });
  };

  const items: MenuItem[] = [
    who,
    { kind: 'separator', id: 's1' },
    ...(loggedIn ? [] : [{ kind: 'item', id: 'login', label: copy.logIn, icon: 'user', onSelect: go('/login') } satisfies MenuItem]),
    ...(failed ? [{ kind: 'item', id: 'retry', label: copy.retry, icon: 'refresh', onSelect: () => void reload() } satisfies MenuItem] : []),
    { kind: 'item', id: 'profile', label: copy.profile, icon: 'user', onSelect: go('/profile') },
    { kind: 'item', id: 'holdings', label: copy.holdings, icon: 'holdings', onSelect: go('/holdings') },
    { kind: 'item', id: 'settings', label: copy.settings, icon: 'settings', onSelect: go('/settings/kite') },
    { kind: 'item', id: 'help', label: copy.help, icon: 'help', onSelect: go('/help') },
    { kind: 'separator', id: 's2' },
    {
      kind: 'checkbox', id: 'dark', label: copy.darkTheme, checked: theme === 'dark',
      onChange: (on) => void toggleTheme(on),
    },
    { kind: 'separator', id: 's3' },
    { kind: 'static', id: 'version', content: <span className="menu__version muted">{info ? copy.version(info.version) : ''}</span> },
  ];

  return <MenuButton label={copy.menu} trigger={<Avatar profile={loggedIn ? profile : null} />} items={items} align="end" />;
}
