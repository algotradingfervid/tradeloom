import type { MouseEvent } from 'react';
import { Link } from 'react-router-dom';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { CHART_ATTRIBUTION } from '../../chart/attribution';
import { aboutCopy as copy, profileMenuCopy } from '../../i18n/copy';
import { DASH } from '../../i18n/format';
import { useAppStore } from '../../store/app';
import { pushToast } from '../../store/toasts';
import { Fact, SettingSection } from './SettingRow';
import '../../components/list.css';

export function AboutGroup() {
  const info = useAppStore((s) => s.info);
  const backend = useBackend();
  // The href keeps the link a real, copyable link; the click opens the system browser instead of navigating the app's
  // only webview off-origin (the reason the chart's attributionLogo is off, NFR-04).
  const openChartSite = (e: MouseEvent<HTMLAnchorElement>) => {
    e.preventDefault();
    void backend.system.openExternal(CHART_ATTRIBUTION.url)
      .catch((err: unknown) => pushToast({ level: 'error', message: parseBackendError(err).reason }));
  };
  return (
    <>
      <SettingSection title={copy.app}>
        <dl className="facts">
          <Fact label={copy.version}>{info ? profileMenuCopy.version(info.version) : DASH}</Fact>
          <Fact label={copy.build}>{info?.build ?? DASH}</Fact>
          <Fact label={copy.schema}>{info ? String(info.schemaVersion) : DASH}</Fact>
        </dl>
      </SettingSection>
      <SettingSection title={copy.legal}>
        <ul className="settings__links">
          <li><Link to="/legal/terms">{copy.terms}</Link></li>
          <li><Link to="/legal/privacy">{copy.privacy}</Link></li>
          <li><Link to="/legal/risk">{copy.risk}</Link></li>
        </ul>
      </SettingSection>
      <SettingSection title={copy.chartLibrary}>
        <dl className="facts">
          <Fact label={copy.component}>{CHART_ATTRIBUTION.name}</Fact>
          <Fact label={copy.licence}>{CHART_ATTRIBUTION.licence}</Fact>
        </dl>
        <p>{CHART_ATTRIBUTION.notice}</p>
        <p>
          <a href={CHART_ATTRIBUTION.url} onClick={openChartSite}>{copy.chartLink}</a> {copy.chartLinkHint}
        </p>
      </SettingSection>
      <SettingSection title={copy.licences}>
        <table className="data-table">
          <caption className="visually-hidden">{copy.licences}</caption>
          <thead>
            <tr><th scope="col">{copy.component}</th><th scope="col">{copy.licenceVersion}</th><th scope="col">{copy.licence}</th></tr>
          </thead>
          <tbody>
            {(info?.licences ?? []).map((l) => (
              <tr key={l.name}><td>{l.name}</td><td>{l.version}</td><td>{l.licence}</td></tr>
            ))}
          </tbody>
        </table>
      </SettingSection>
    </>
  );
}
