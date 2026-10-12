import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { Button } from '../../components/Button';
import { riskCopy } from '../../i18n/copy';
import { riskDisclosure } from '../../legal/content';
import { pushToast } from '../../store/toasts';
import { adoptReturnedStatus } from './adoptStatus';
import { OnboardingFrame } from './OnboardingFrame';

export function RiskDisclosureView() {
  const backend = useBackend();
  const [busy, setBusy] = useState(false);
  const ack = async () => {
    setBusy(true);
    try {
      adoptReturnedStatus(await backend.session.ackRiskDisclosure());
    } catch (e) {
      pushToast({ level: 'error', message: parseBackendError(e).reason });
      setBusy(false);
    }
  };
  return (
    <OnboardingFrame title={riskCopy.heading}>
      {riskDisclosure.paragraphs.map((p) => <p key={p}>{p}</p>)}
      <p>
        <Link to="/legal/terms">{riskCopy.terms}</Link>
        {' · '}
        <Link to="/legal/privacy">{riskCopy.privacy}</Link>
      </p>
      <Button variant="primary" onClick={ack} disabled={busy}>{riskCopy.accept}</Button>
    </OnboardingFrame>
  );
}
