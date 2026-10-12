import { useEffect, type ReactNode } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { gateCopy } from '../i18n/copy';
import { useAppStore } from '../store/app';
import { useSessionStore } from '../store/session';
import { BootNotice } from './BootNotice';
import { gateTarget } from './gate';

export function SessionGate({ children }: { children: ReactNode }) {
  const session = useSessionStore((s) => s.session);
  const bootFailed = useAppStore((s) => s.bootError !== null);
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const target = session ? gateTarget(session, pathname) : null;
  useEffect(() => {
    if (target) void navigate(target, { replace: true });
  }, [target, navigate]);
  if (!session || target) {
    // A failed session load must not leave an endless "Starting…": the boot notice offers Retry (Ruling 83-F I2).
    return (
      <div className="boot">
        {!session && bootFailed ? <BootNotice /> : <span role="status">{gateCopy.starting}</span>}
      </div>
    );
  }
  return <>{children}</>;
}
