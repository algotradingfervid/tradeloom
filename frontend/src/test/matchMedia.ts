type Listener = (e: MediaQueryListEvent) => void;

let systemDark = false;
const listeners = new Set<Listener>();

export function setSystemDark(value: boolean): void {
  systemDark = value;
  for (const l of [...listeners]) l({ matches: value, media: '(prefers-color-scheme: dark)' } as MediaQueryListEvent);
}

export function resetMatchMedia(): void {
  systemDark = false;
  listeners.clear();
}

export function installMatchMedia(): void {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: (query: string) => ({
      get matches() {
        return query.includes('dark') ? systemDark : false;
      },
      media: query,
      onchange: null,
      addEventListener: (_type: string, l: Listener) => listeners.add(l),
      removeEventListener: (_type: string, l: Listener) => listeners.delete(l),
      addListener: (l: Listener) => listeners.add(l),
      removeListener: (l: Listener) => listeners.delete(l),
      dispatchEvent: () => true,
    }),
  });
}
