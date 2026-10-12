import { create } from 'zustand';
import type { AppInfo } from '../api/backend';
import { registerReset } from './reset';

interface AppStore { info: AppInfo | null; setInfo(i: AppInfo): void }

export const useAppStore = create<AppStore>()((set) => ({ info: null, setInfo: (info) => set({ info }) }));

registerReset(() => useAppStore.setState({ info: null }));
