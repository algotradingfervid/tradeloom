import { useParams } from 'react-router-dom';
import { ListPage } from '../../components/ListPage';
import { renderArticle } from '../../help/renderArticle';
import { legalCopy } from '../../i18n/copy';
import { LEGAL_PAGES } from '../../legal/content';
import { NotFoundView } from '../NotFoundView';
import '../../help/help.css';

/**
 * Terms, privacy and risk pages (FR-ACCT-015). Ruling 108 M6: routed in rootChildren, outside the shell, so the risk
 * screen's links work before acknowledgement without the shell chrome; the page therefore has its own main.
 */
export function LegalView() {
  const { page } = useParams();
  // Own keys only: "/legal/constructor" must not resolve to Object.prototype members.
  const content = page && Object.hasOwn(LEGAL_PAGES, page) ? LEGAL_PAGES[page] : undefined;
  return (
    <main className="standalone-page">
      {content ? (
        <ListPage title={content.title} description={legalCopy.updatedFor(content.updatedFor)}>
          {/* Section headings sit directly under the page h1. */}
          <article className="legal" aria-label={content.title}>{renderArticle(content.paragraphs.join('\n\n'), { headingLevel: 2 })}</article>
        </ListPage>
      ) : (
        <NotFoundView />
      )}
    </main>
  );
}
