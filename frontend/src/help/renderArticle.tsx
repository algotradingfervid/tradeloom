import type { ReactNode } from 'react';

function inline(text: string): ReactNode[] {
  return text.split(/(\*\*[^*]+\*\*)/g).map((part, i) =>
    part.startsWith('**') && part.endsWith('**') && part.length > 4 ? <strong key={i}>{part.slice(2, -2)}</strong> : part,
  );
}

export interface RenderArticleOptions {
  /** Level of "## " headings: 3 under an article's h2 title (default), 2 directly under a page h1 (legal pages). */
  headingLevel?: 2 | 3;
}

/** Minimal, injection-free renderer: blank-line blocks, "## " headings, "- " lists, **bold**. */
export function renderArticle(body: string, { headingLevel = 3 }: RenderArticleOptions = {}): ReactNode[] {
  const Heading = headingLevel === 2 ? 'h2' : 'h3';
  return body
    .trim()
    .split(/\n{2,}/)
    .filter((block) => block.trim() !== '')
    .map((block, i) => {
      if (block.startsWith('## ')) return <Heading key={i}>{block.slice(3)}</Heading>;
      const lines = block.split('\n');
      if (lines.every((l) => l.startsWith('- '))) {
        return <ul key={i}>{lines.map((l, j) => <li key={j}>{inline(l.slice(2))}</li>)}</ul>;
      }
      return <p key={i}>{inline(block.replace(/\n/g, ' '))}</p>;
    });
}
