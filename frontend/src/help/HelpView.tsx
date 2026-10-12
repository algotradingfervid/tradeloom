import { forwardRef, useEffect, useMemo, useRef, useState } from 'react';
import { Link, NavLink, useParams } from 'react-router-dom';
import { SupportActions } from '../app/SupportActions';
import { ListPage } from '../components/ListPage';
import { helpCopy as copy } from '../i18n/copy';
import { useShortcutStore } from '../shortcuts/store';
import { NotFoundView } from '../views/NotFoundView';
import { ARTICLES, HELP_CATEGORIES, shortcutsArticle, type HelpArticle } from './articles';
import { renderArticle } from './renderArticle';
import { searchArticles } from './search';
import './help.css';

/**
 * One article. On the help page its title is an h2 under the page h1; `focusTitle` moves focus to it (after the user
 * picked another article, so focus is not lost). In the side panel (`inPanel`) the dialog's own h2 already carries the
 * title, so it is not repeated: the article is named by aria-label and is itself focusable (tabIndex -1, the
 * dialog's initial focus) so its body scrolls with the keyboard.
 */
export const ArticleBody = forwardRef<HTMLElement, { article: HelpArticle; inPanel?: boolean; focusTitle?: boolean }>(
  function ArticleBody({ article, inPanel = false, focusTitle = false }, ref) {
    const titleId = `help-${article.id}`;
    const title = useRef<HTMLHeadingElement>(null);
    useEffect(() => {
      if (focusTitle) title.current?.focus();
    }, [focusTitle]);
    return (
      <article
        ref={ref}
        className="help__article"
        aria-labelledby={inPanel ? undefined : titleId}
        aria-label={inPanel ? article.title : undefined}
        tabIndex={inPanel ? -1 : undefined}
      >
        <p className="muted">{article.category}</p>
        {!inPanel && <h2 ref={title} id={titleId} tabIndex={-1}>{article.title}</h2>}
        <p className="muted">{copy.updatedFor(article.updatedFor)}</p>
        {renderArticle(article.body)}
      </article>
    );
  },
);

function SupportLinks() {
  return (
    <section className="help__support" aria-labelledby="help-support">
      <h2 id="help-support">{copy.support}</h2>
      <p className="muted">{copy.supportHint}</p>
      <SupportActions />
    </section>
  );
}

/**
 * The help centre (FR-ACCT-010), framed by DocsFrame (Ruling 117 I1). Each article gets a fresh HelpContent (Minor 1:
 * the search clears); after the first render a new article takes focus on its heading (Minor 2), while opening Help
 * directly leaves focus where the shell puts it.
 */
export function HelpView() {
  const { article: articleId } = useParams();
  const [first] = useState(articleId);
  const [changed, setChanged] = useState(false);
  // Derived during render (React's "adjust state on prop change" pattern): once the article changes, it stays changed.
  if (!changed && articleId !== first) setChanged(true);
  return <HelpContent key={articleId ?? ''} articleId={articleId} focusArticle={changed || articleId !== first} />;
}

function HelpContent({ articleId, focusArticle }: { articleId: string | undefined; focusArticle: boolean }) {
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
            <ArticleBody article={current} focusTitle={focusArticle} />
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
