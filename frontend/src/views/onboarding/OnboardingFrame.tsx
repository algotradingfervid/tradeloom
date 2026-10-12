import type { ReactNode } from 'react';
import { BootNotice } from '../../app/BootNotice';
import { setupCopy } from '../../i18n/copy';
import './onboarding.css';

export function OnboardingFrame({ title, children }: { title: string; children: ReactNode }) {
  return (
    <main className="onboarding">
      <div className="onboarding__card">
        {/* Task 7 → Task 9: the boot-failure notice also shows before the shell exists. */}
        <BootNotice />
        <span className="onboarding__brand">{setupCopy.brand}</span>
        <h1>{title}</h1>
        {children}
      </div>
    </main>
  );
}
