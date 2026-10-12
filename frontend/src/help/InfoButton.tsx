import { useState } from 'react';
import { Dialog } from '../components/Dialog';
import { IconButton } from '../components/IconButton';
import { ARTICLES } from './articles';
import { ArticleBody } from './HelpView';

/**
 * The (i) control on section headers (FR-SHELL-013): opens the matching article in a side panel. Focus moves only when
 * the user opens it; the panel traps focus, closes on Escape or ×, and returns focus to this button (Dialog).
 */
export function InfoButton({ articleId, label }: { articleId: string; label: string }) {
  const [open, setOpen] = useState(false);
  const article = ARTICLES.find((a) => a.id === articleId);
  if (!article) return null;
  return (
    <>
      <IconButton label={label} icon="info" aria-haspopup="dialog" onClick={() => setOpen(true)} />
      <Dialog open={open} title={article.title} size="side" onClose={() => setOpen(false)}>
        <ArticleBody article={article} inPanel />
      </Dialog>
    </>
  );
}
