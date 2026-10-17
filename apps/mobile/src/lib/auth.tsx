import Constants from 'expo-constants';
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { Platform } from 'react-native';

import { api, ApiError, refreshSession, toSession } from '@/api/client';
import { googleIdToken, googleSignOut } from '@/features/auth/google';
import { clearOutbox } from '@/features/chat/outbox';
import { unregisterPush } from '@/lib/push';
import { queryClient } from '@/lib/query';
import { getSession, loadStoredRefreshToken, onSessionChange, primeSession, setSession } from '@/lib/session';

type Status = 'loading' | 'signedOut' | 'signedIn';

type AuthCtx = {
  status: Status;
  signUpWithEmail: (email: string, password: string) => Promise<{ isNewUser: boolean }>;
  logInWithEmail: (email: string, password: string) => Promise<{ isNewUser: boolean }>;
  // Resolves null when the user closes the Google account picker.
  signInWithGoogle: () => Promise<{ isNewUser: boolean } | null>;
  logout: () => Promise<void>;
};

const Ctx = createContext<AuthCtx | null>(null);

export function useAuth() {
  const c = useContext(Ctx);
  if (!c) throw new Error('useAuth outside AuthProvider');
  return c;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<Status>('loading');

  useEffect(() => {
    let alive = true;
    (async () => {
      const stored = await loadStoredRefreshToken();
      if (!stored) {
        if (alive) setStatus('signedOut');
        return;
      }
      try {
        await refreshSession(stored);
        if (alive) setStatus(getSession() ? 'signedIn' : 'signedOut');
      } catch (e) {
        if (!alive) return;
        // Offline at launch: keep the user signed in and rotate once the network is back.
        if (e instanceof ApiError && e.status === 0) {
          primeSession(stored);
          setStatus('signedIn');
        } else {
          setStatus('signedOut');
        }
      }
    })();
    const off = onSessionChange((s) => {
      if (!s) {
        queryClient.clear();
        setStatus('signedOut');
      }
    });
    return () => {
      alive = false;
      off();
    };
  }, []);

  const startSession = useCallback(async (path: string, body: Record<string, unknown>) => {
    const res = await api<{ session: Parameters<typeof toSession>[0]; is_new_user: boolean }>(
      'POST',
      path,
      { ...body, device: { platform: Platform.OS === 'ios' ? 'ios' : 'android', app_version: Constants.expoConfig?.version ?? '' } },
      { auth: false },
    );
    await setSession(toSession(res.session));
    setStatus('signedIn');
    return { isNewUser: res.is_new_user };
  }, []);

  const signUpWithEmail = useCallback(
    (email: string, password: string) => startSession('/v1/auth/email/signup', { email, password }),
    [startSession],
  );

  const logInWithEmail = useCallback(
    (email: string, password: string) => startSession('/v1/auth/email/login', { email, password }),
    [startSession],
  );

  const signInWithGoogle = useCallback(async () => {
    const idToken = await googleIdToken();
    if (!idToken) return null;
    return startSession('/v1/auth/google', { id_token: idToken });
  }, [startSession]);

  const logout = useCallback(async () => {
    await unregisterPush();
    await clearOutbox();
    await googleSignOut();
    const s = getSession();
    if (s?.refreshToken) {
      await api('POST', '/v1/auth/logout', { refresh_token: s.refreshToken }, { auth: false }).catch(() => {});
    }
    await setSession(null);
  }, []);

  const value = useMemo(
    () => ({ status, signUpWithEmail, logInWithEmail, signInWithGoogle, logout }),
    [status, signUpWithEmail, logInWithEmail, signInWithGoogle, logout],
  );
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}
