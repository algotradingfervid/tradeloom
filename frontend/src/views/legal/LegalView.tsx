import { useParams } from 'react-router-dom';
import { ListPage } from '../../components/ListPage';
import { renderArticle } from '../../help/renderArticle';
import { helpCopy } from '../../i18n/copy';
import { LEGAL_PAGES } from '../../legal/content';
import { NotFoundView } from '../NotFoundView';
import '../../help/help.css';

/** Terms, privacy and risk pages (FR-ACCT-015), framed by DocsFrame (Ruling 108 M6 / 117 I1). */
export function LegalView() {
  const { page } = useParams();
  // Own keys only: "/legal/constructor" must not resolve to Object.prototype members.
  const content = page && Object.hasOwn(LEGAL_PAGES, page) ? LEGAL_PAGES[page] : undefined;
  if (!content) return <NotFoundView />;
  return (
    <ListPage title={content.title} description={helpCopy.updatedFor(content.updatedFor)}>
      {/* Section headings sit directly under the page h1. */}
      <article className="legal" aria-label={content.title}>{renderArticle(content.paragraphs.join('\n\n'), { headingLevel: 2 })}</article>
    </ListPage>
  );
}
