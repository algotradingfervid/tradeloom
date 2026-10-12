import { useNavigate } from 'react-router-dom';
import { Button } from '../components/Button';
import { notFoundCopy } from '../i18n/copy';
import '../components/list.css';

export type MissingKind = 'page' | 'strategy' | 'scanner' | 'backtest';

export function NotFoundView({ kind = 'page' }: { kind?: MissingKind }) {
  const navigate = useNavigate();
  return (
    <section className="not-found">
      <h1>{notFoundCopy[kind]}</h1>
      <p className="muted">{notFoundCopy.explanation}</p>
      <Button variant="primary" onClick={() => void navigate('/home')}>{notFoundCopy.goHome}</Button>
    </section>
  );
}
