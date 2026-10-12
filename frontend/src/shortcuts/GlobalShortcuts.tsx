import { useNavigate } from 'react-router-dom';
import { visibleNavEntries } from '../app/features';
import { focusNewestToast } from '../store/toasts';
import { useShortcut } from './ShortcutProvider';

function NavShortcut({ actionId, path }: { actionId: string; path: string }) {
  const navigate = useNavigate();
  useShortcut(actionId, () => void navigate(path));
  return null;
}

/** Hidden (later-phase) sidebar entries get no handler, so their Alt+N does nothing. */
export function GlobalShortcuts() {
  const navigate = useNavigate();
  useShortcut('toasts.focus', () => {
    focusNewestToast();
  });
  useShortcut('help.open', () => void navigate('/help'));
  return (
    <>
      {visibleNavEntries().map((e) => <NavShortcut key={e.id} actionId={e.shortcutId} path={e.path} />)}
    </>
  );
}
