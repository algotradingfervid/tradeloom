import { Button } from '../components/Button';
import { bootCopy } from '../i18n/copy';
import { useAppStore } from '../store/app';

/** Start-up load failure with Retry (Ruling 83-F I2), shown in the AppShell banner slot so it stays inside the 100vh frame. */
export function BootNotice() {
  const bootError = useAppStore((s) => s.bootError);
  const retryBoot = useAppStore((s) => s.retryBoot);
  if (!bootError) return null;
  return (
    <div role="alert" className="boot-notice">
      <span>{bootCopy.failed}</span>
      <span className="boot-notice__reason">{bootError.reason}</span>
      <Button onClick={retryBoot}>{bootCopy.retry}</Button>
    </div>
  );
}
