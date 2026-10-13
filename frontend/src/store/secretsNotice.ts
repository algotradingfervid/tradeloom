import { create } from 'zustand';
import { registerReset } from './reset';

/**
 * Run-lifetime secrets notice (Ruling 97 I4): set when `Session.MigrateToKeyring` refused with `file_not_removed`
 * (secrets moved, the old secrets.enc is still on disk). Go keeps the matching in-memory flag for the whole run
 * (Ruling 72 (b)), so the warning and its retry survive leaving Settings and a logout. Not account data.
 */
interface SecretsNoticeStore {
  leftoverFile: boolean;
  setLeftoverFile(v: boolean): void;
}

export const useSecretsNoticeStore = create<SecretsNoticeStore>()((set) => ({
  leftoverFile: false,
  setLeftoverFile: (leftoverFile) => set({ leftoverFile }),
}));

registerReset(() => useSecretsNoticeStore.setState({ leftoverFile: false }), { onLogout: false, name: 'secretsNotice', inspect: () => useSecretsNoticeStore.getState() });
