import { API_URL } from '@/lib/config';
import { getSession, setSession, type Session } from '@/lib/session';

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public fields?: Record<string, string>,
  ) {
    super(message);
  }
}

type SessionDTO = {
  access_token: string;
  access_expires_at: string;
  refresh_token: string;
};

export function toSession(s: SessionDTO): Session {
  return {
    accessToken: s.access_token,
    accessExpiresAt: Date.parse(s.access_expires_at),
    refreshToken: s.refresh_token,
  };
}

async function raw(method: string, path: string, body?: unknown, token?: string) {
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (token) headers.Authorization = `Bearer ${token}`;
  let res: Response;
  try {
    res = await fetch(`${API_URL}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, 'network', "Can't reach Metro Buddy. Check your connection.");
  }
  if (res.status === 204) return undefined;
  const text = await res.text();
  const json = text ? JSON.parse(text) : undefined;
  if (!res.ok) {
    const e = json?.error ?? {};
    throw new ApiError(res.status, e.code ?? 'unknown', e.message ?? 'Something went wrong', e.fields);
  }
  return json;
}

let refreshing: Promise<Session | null> | null = null;

// Single-flight: concurrent 401s share one rotation, since a second rotation of the
// same refresh token would look like token theft and revoke the whole session.
export function refreshSession(refreshToken?: string): Promise<Session | null> {
  if (refreshing) return refreshing;
  const token = refreshToken ?? getSession()?.refreshToken;
  if (!token) return Promise.resolve(null);
  refreshing = (async () => {
    try {
      const s = toSession(await raw('POST', '/v1/auth/refresh', { refresh_token: token }));
      await setSession(s);
      return s;
    } catch (e) {
      if (e instanceof ApiError && e.status !== 0) await setSession(null);
      throw e;
    } finally {
      refreshing = null;
    }
  })();
  return refreshing;
}

export async function api<T = unknown>(
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE',
  path: string,
  body?: unknown,
  opts: { auth?: boolean } = {},
): Promise<T> {
  const auth = opts.auth ?? true;
  if (!auth) return raw(method, path, body) as Promise<T>;

  let s = getSession();
  if (s && s.accessExpiresAt - Date.now() < 30_000) s = await refreshSession();
  if (!s) throw new ApiError(401, 'unauthorized', 'Please log in again');
  try {
    return (await raw(method, path, body, s.accessToken)) as T;
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      const next = await refreshSession();
      if (!next) throw e;
      return (await raw(method, path, body, next.accessToken)) as T;
    }
    throw e;
  }
}

export const get = <T>(path: string) => api<T>('GET', path);
export const post = <T>(path: string, body?: unknown) => api<T>('POST', path, body ?? {});
export const put = <T>(path: string, body?: unknown) => api<T>('PUT', path, body ?? {});
export const patch = <T>(path: string, body?: unknown) => api<T>('PATCH', path, body ?? {});
export const del = <T>(path: string) => api<T>('DELETE', path);
