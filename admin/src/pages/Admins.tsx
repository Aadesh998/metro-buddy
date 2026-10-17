import { useEffect, useState, type FormEvent } from 'react'

import { api, ApiError, type Admin, type Role } from '../api'
import { Badge, Button, Card, ErrorText, Input, PageHeader, Select } from '../components/ui'

export function AdminsPage({ me }: { me: Admin }) {
  const [rows, setRows] = useState<Admin[]>([])
  const [email, setEmail] = useState('')
  const [role, setRole] = useState<Role>('moderator')
  const [created, setCreated] = useState<{ temp_password: string; totp_url: string; admin: Admin } | null>(null)
  const [error, setError] = useState<string>()

  const load = () => api<{ items: Admin[] }>('GET', '/admins').then((r) => setRows(r.items))
  useEffect(() => {
    load()
  }, [])

  async function create(e: FormEvent) {
    e.preventDefault()
    setError(undefined)
    try {
      setCreated(await api('POST', '/admins', { email, role }))
      setEmail('')
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed')
    }
  }

  async function update(a: Admin, patch: Partial<Admin>) {
    const r = await api<{ items: Admin[] }>('PATCH', `/admins/${a.id}`, patch)
    setRows(r.items)
  }

  return (
    <>
      <PageHeader title="Team" sub="Moderators handle the queue. Trust & safety leads also handle grievances and the PII vault." />
      <Card className="mb-6 p-5">
        <form onSubmit={create} className="flex flex-wrap items-end gap-3">
          <label className="min-w-64 flex-1 space-y-1 text-sm">
            <span>Email</span>
            <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          </label>
          <label className="space-y-1 text-sm">
            <span className="block">Role</span>
            <Select value={role} onChange={(e) => setRole(e.target.value as Role)}>
              <option value="moderator">Moderator</option>
              <option value="trust_safety_lead">Trust &amp; safety lead</option>
              <option value="admin">Admin</option>
            </Select>
          </label>
          <Button type="submit">Add member</Button>
        </form>
        <ErrorText error={error} />
        {created ? (
          <div className="mt-4 rounded-lg border border-accent/40 bg-brand/5 p-4 text-sm">
            <p className="mb-2 font-semibold">Share these with {created.admin.email} once, over a secure channel. They won’t be shown again.</p>
            <p>
              Temporary password: <code className="rounded bg-raised px-1.5 py-0.5">{created.temp_password}</code>
            </p>
            <p className="mt-1 break-all">
              Authenticator setup URL: <code className="rounded bg-raised px-1.5 py-0.5">{created.totp_url}</code>
            </p>
          </div>
        ) : null}
      </Card>
      <Card>
        <table className="w-full text-sm">
          <thead className="text-left text-muted">
            <tr>
              <th className="px-5 py-3 font-medium">Email</th>
              <th className="px-5 py-3 font-medium">Role</th>
              <th className="px-5 py-3 font-medium">Status</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {rows.map((a) => (
              <tr key={a.id} className="border-t border-line">
                <td className="px-5 py-3">{a.email}</td>
                <td className="px-5 py-3">
                  <Select value={a.role} disabled={a.id === me.id} onChange={(e) => update(a, { role: e.target.value as Role })}>
                    <option value="moderator">moderator</option>
                    <option value="trust_safety_lead">trust_safety_lead</option>
                    <option value="admin">admin</option>
                  </Select>
                </td>
                <td className="px-5 py-3">{a.active ? <Badge tone="actioned">active</Badge> : <Badge>disabled</Badge>}</td>
                <td className="px-5 py-3 text-right">
                  {a.id !== me.id ? (
                    <Button
                      kind={a.active ? 'secondary' : 'ghost'}
                      onClick={() => (!a.active || window.confirm(`Disable ${a.email}? They will be signed out immediately.`)) && update(a, { active: !a.active })}
                    >
                      {a.active ? 'Disable' : 'Enable'}
                    </Button>
                  ) : null}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </>
  )
}
