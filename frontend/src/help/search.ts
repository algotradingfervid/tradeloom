import type { HelpArticle } from './articles';

export function searchArticles(articles: HelpArticle[], query: string): HelpArticle[] {
  const terms = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (terms.length === 0) return [];
  const matches = articles.filter((a) => {
    const text = `${a.title}\n${a.body}`.toLowerCase();
    return terms.every((t) => text.includes(t));
  });
  const inTitle = (a: HelpArticle) => terms.every((t) => a.title.toLowerCase().includes(t));
  return matches.sort((a, b) => Number(inTitle(b)) - Number(inTitle(a)));
}
