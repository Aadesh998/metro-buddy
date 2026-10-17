export class ApiError extends Error {
  status: number
  code: string
  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

export async function api<T = unknown>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`/admin/api${path}`, {
    method,
    credentials: 'same-origin',
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (res.status === 204) return undefined as T
  const json = await res.json().catch(() => ({}))
  if (!res.ok) {
    if (res.status === 401) window.dispatchEvent(new Event('admin:logout'))
    throw new ApiError(res.status, json?.error?.code ?? 'unknown', json?.error?.message ?? 'Request failed')
  }
  return json as T
}

export type Role = 'moderator' | 'trust_safety_lead' | 'admin'
export type Admin = { id: string; email: string; role: Role; active: boolean; created_at: string }

export const roleRank: Record<Role, number> = { moderator: 0, trust_safety_lead: 1, admin: 2 }
