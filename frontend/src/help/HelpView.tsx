import { useMemo, useState } from 'react';
import { Link, NavLink, useParams } from 'react-router-dom';
import { useBackend } from '../api/BackendContext';
import { parseBackendError } from '../api/errors';
import { Button } from '../components/Button';
import { ListPage } from '../components/ListPage';
import { diagnosticsCopy, helpCopy as copy } from '../i18n/copy';
import { useShortcutStore } from '../shortcuts/store';
import { pushToast } from '../store/toasts';
import { NotFoundView } from '../views/NotFoundView';
import { ARTICLES, HELP_CATEGORIES, shortcutsArticle, type HelpArticle } from './articles';
import { renderArticle } from './renderArticle';
import { searchArticles } from './search';
import './help.css';

/**
 * One article. On the help page its title is an h2 under the page h1; in the side panel (`inPanel`) the dialog's own
 * h2 already carries the title, so it is not repeated and the article is named by aria-label instead.
 */
export function ArticleBody({ article, inPanel = false }: { article: HelpArticle; inPanel?: boolean }) {
  const titleId = `help-${article.id}`;
  return (
    <article className="help__article" aria-labelledby={inPanel ? undefined : titleId} aria-label={inPanel ? article.title : undefined}>
      <p className="muted">{article.category}</p>
      {!inPanel && <h2 id={titleId}>{article.title}</h2>}
      <p className="muted">{copy.updatedFor(article.updatedFor)}</p>
      {renderArticle(article.body)}
    </article>
  );
}

function SupportLinks() {
  const backend = useBackend();
  const fail = (e: unknown) => pushToast({ level: 'error', message: parseBackendError(e).reason });
  const bundle = () =>
    backend.diagnostics.createSupportBundle().then((path) => pushToast({ level: 'success', message: diagnosticsCopy.bundleSaved(path) }), fail);
  return (
    <section className="help__support" aria-labelledby="help-support">
      <h2 id="help-support">{copy.support}</h2>
      <p className="muted">{copy.supportHint}</p>
      <div className="settings__actions">
        <Button onClick={() => void backend.diagnostics.openLogsFolder().catch(fail)}>{diagnosticsCopy.openLogs}</Button>
        <Button onClick={() => void bundle()}>{diagnosticsCopy.bundle}</Button>
      </div>
    </section>
  );
}

/** The help centre (FR-ACCT-010). Ruling 108 M6: routed in rootChildren, outside the shell, so it has its own main. */
export function HelpView() {
  return (
    <main className="standalone-page">
      <HelpContent />
    </main>
  );
}

function HelpContent() {
  const { article: articleId } = useParams();
  const actions = useShortcutStore((s) => s.actions);
  const all = useMemo(() => [...ARTICLES, shortcutsArticle(actions)], [actions]);
  const [query, setQuery] = useState('');
  const categories = HELP_CATEGORIES.filter((c) => all.some((a) => a.category === c));
  const current = articleId ? all.find((a) => a.id === articleId) : undefined;
  if (articleId && !current) return <NotFoundView />;
  const searching = query.trim() !== '';
  const results = searchArticles(all, query);

  return (
    <ListPage
      title={copy.title}
      description={copy.description}
      // ListPage announces "no matches" (polite live status) when a search hides every article.
      search={{ value: query, onChange: setQuery, placeholder: copy.searchPlaceholder, matches: results.length }}
    >
      <div className="help">
        <nav className="help__tree" aria-label={copy.tree}>
          {categories.map((c) => (
            <div key={c}>
              <h2>{c}</h2>
              <ul>
                {all.filter((a) => a.category === c).map((a) => (
                  <li key={a.id}><NavLink to={`/help/${a.id}`}>{a.title}</NavLink></li>
                ))}
              </ul>
            </div>
          ))}
        </nav>
        <div>
          {searching ? (
            <section aria-label={copy.results}>
              {results.length > 0 && (
                <ul className="help__results">
                  {results.map((a) => (
                    <li key={a.id}><Link to={`/help/${a.id}`} onClick={() => setQuery('')}>{a.title}</Link> <span className="muted">— {a.category}</span></li>
                  ))}
                </ul>
              )}
            </section>
          ) : current ? (
            <ArticleBody article={current} />
          ) : (
            <>
              <ul className="help__cards" aria-label={copy.topics}>
                {categories.map((c) => {
                  const inCategory = all.filter((a) => a.category === c);
                  return (
                    <li key={c}>
                      <Link to={`/help/${inCategory[0].id}`} className="card help__card">
                        <strong>{c}</strong>
                        <span className="muted">{copy.articles(inCategory.length)}</span>
                      </Link>
                    </li>
                  );
                })}
              </ul>
              <SupportLinks />
            </>
          )}
        </div>
      </div>
    </ListPage>
  );
}
