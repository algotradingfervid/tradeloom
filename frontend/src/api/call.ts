import { parseBackendError } from './errors';

export class BindingError extends Error {
  constructor(readonly code: string, message: string, readonly stage: string = '') {
    super(message);
    this.name = 'BindingError';
  }
}

/** Awaits a Backend promise; a rejection becomes a BindingError carrying P0-3's {code, stage, reason}. */
export async function call<T>(p: Promise<T>): Promise<T> {
  try {
    return await p;
  } catch (e) {
    const { code, stage, reason } = parseBackendError(e);
    throw new BindingError(code, reason, stage);
  }
}
