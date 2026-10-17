import Constants from 'expo-constants';
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { Platform } from 'react-native';

import { api, ApiError, refreshSession, toSession } from '@/api/client';
import { queryClient } from '@/lib/query';
import { getSession, loadStoredRefreshToken, onSessionChange, primeSession, setSession } from '@/lib/session';

type Status = 'loading' | 'signedOut' | 'signedIn';

type AuthCtx = {
  status: Status;
  requestOtp: (phone: string) => Promise<void>;
  verifyOtp: (phone: string, code: string) => Promise<{ isNewUser: boolean }>;
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

  const requestOtp = useCallback(async (phone: string) => {
    await api('POST', '/v1/auth/otp/request', { phone }, { auth: false });
  }, []);

  const verifyOtp = useCallback(async (phone: string, code: string) => {
    const res = await api<{ session: Parameters<typeof toSession>[0]; is_new_user: boolean }>(
      'POST',
      '/v1/auth/otp/verify',
      {
        phone,
        code,
        device: { platform: Platform.OS === 'ios' ? 'ios' : 'android', app_version: Constants.expoConfig?.version ?? '' },
      },
      { auth: false },
    );
    await setSession(toSession(res.session));
    setStatus('signedIn');
    return { isNewUser: res.is_new_user };
  }, []);

  const logout = useCallback(async () => {
    const s = getSession();
    if (s?.refreshToken) {
      await api('POST', '/v1/auth/logout', { refresh_token: s.refreshToken }, { auth: false }).catch(() => {});
    }
    await setSession(null);
  }, []);

  const value = useMemo(() => ({ status, requestOtp, verifyOtp, logout }), [status, requestOtp, verifyOtp, logout]);
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}
