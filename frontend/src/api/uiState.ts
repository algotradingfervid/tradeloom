import { currentBackend } from './current';

/** UI state lives in Go's window_state table (Settings.GetUiState/SetUiState, "" when absent); values are JSON. */
export async function readUiState<T>(key: string, fallback: T): Promise<T> {
  const raw = await currentBackend().settings.getUiState(key);
  if (!raw) return fallback;
  try {
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}

export async function writeUiState<T>(key: string, value: T): Promise<void> {
  await currentBackend().settings.setUiState(key, JSON.stringify(value));
}
