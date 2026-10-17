import * as SecureStore from 'expo-secure-store';
import { Platform } from 'react-native';

const REFRESH_KEY = 'mb.refresh_token';

// Only the refresh token is persisted (Keychain/Keystore). The access token lives in memory.
export type Session = {
  accessToken: string;
  accessExpiresAt: number;
  refreshToken: string;
};

let current: Session | null = null;
const listeners = new Set<(s: Session | null) => void>();

export function getSession() {
  return current;
}

// Web is only a developer preview; real clients keep the token in Keychain/Keystore.
const store =
  Platform.OS === 'web'
    ? {
        set: async (v: string) => localStorage.setItem(REFRESH_KEY, v),
        get: async () => localStorage.getItem(REFRESH_KEY),
        del: async () => localStorage.removeItem(REFRESH_KEY),
      }
    : {
        set: (v: string) => SecureStore.setItemAsync(REFRESH_KEY, v, { keychainAccessible: SecureStore.AFTER_FIRST_UNLOCK_THIS_DEVICE_ONLY }),
        get: () => SecureStore.getItemAsync(REFRESH_KEY),
        del: () => SecureStore.deleteItemAsync(REFRESH_KEY),
      };

export async function setSession(s: Session | null) {
  current = s;
  if (s) await store.set(s.refreshToken);
  else await store.del();
  listeners.forEach((l) => l(s));
}

export function loadStoredRefreshToken() {
  return store.get();
}

export function onSessionChange(fn: (s: Session | null) => void) {
  listeners.add(fn);
  return () => {
    listeners.delete(fn);
  };
}

// Restores an in-memory session from a stored refresh token without rewriting storage;
// the first API call will rotate it.
export function primeSession(refreshToken: string) {
  current = { accessToken: '', accessExpiresAt: 0, refreshToken };
}
