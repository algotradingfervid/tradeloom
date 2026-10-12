import './components.css';

export function Skeleton({ rows = 3, label }: { rows?: number; label: string }) {
  return (
    <div role="status" aria-label={label} className="skeleton">
      {Array.from({ length: rows }, (_, i) => <div key={i} className="skeleton__row" />)}
    </div>
  );
}
