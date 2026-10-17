import { useEffect, useState, type FormEvent } from 'react'

import { api, ApiError, roleRank, type Admin } from '../api'
import { Badge, Button, Card, ErrorText, Input, PageHeader, Select } from '../components/ui'

type Bowl = { ID: string; Kind: string; Slug: string; Name: string; Description: string; Visibility: string; MemberCount: number }

export function BowlsPage({ me }: { me: Admin }) {
  const [rows, setRows] = useState<Bowl[]>([])
  const [kind, setKind] = useState('')
  const [form, setForm] = useState({ kind: 'topic', slug: '', name: '', description: '', visibility: 'public' })
  const [error, setError] = useState<string>()
  const canEdit = roleRank[me.role] >= roleRank.admin

  const load = (k = kind) => api<{ items: Bowl[] }>('GET', `/bowls${k ? `?kind=${k}` : ''}`).then((r) => setRows(r.items))
  useEffect(() => {
    api<{ items: Bowl[] }>('GET', '/bowls').then((r) => setRows(r.items))
  }, [])

  async function create(e: FormEvent) {
    e.preventDefault()
    setError(undefined)
    try {
      await api('POST', '/bowls', form)
      setForm({ ...form, slug: '', name: '', description: '' })
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed')
    }
  }

  return (
    <>
      <PageHeader
        title="Bowls"
        sub={`${rows.length} communities`}
        right={
          <Select
            value={kind}
            onChange={(e) => {
              setKind(e.target.value)
              load(e.target.value)
            }}
          >
            <option value="">All kinds</option>
            {['city', 'line', 'station', 'office_hub', 'college', 'topic'].map((k) => (
              <option key={k}>{k}</option>
            ))}
          </Select>
        }
      />
      {canEdit ? (
        <Card className="mb-6 p-5">
          <form onSubmit={create} className="grid grid-cols-2 gap-3 lg:grid-cols-6">
            <Select value={form.kind} onChange={(e) => setForm({ ...form, kind: e.target.value })}>
              {['topic', 'office_hub', 'college'].map((k) => (
                <option key={k}>{k}</option>
              ))}
            </Select>
            <Input placeholder="slug" value={form.slug} onChange={(e) => setForm({ ...form, slug: e.target.value })} required />
            <Input placeholder="Name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
            <Input className="lg:col-span-2" placeholder="Description" value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
            <Button type="submit">Create bowl</Button>
          </form>
          <ErrorText error={error} />
        </Card>
      ) : null}
      <Card>
        <table className="w-full text-sm">
          <thead className="text-left text-muted">
            <tr>
              <th className="px-5 py-3 font-medium">Name</th>
              <th className="px-5 py-3 font-medium">Kind</th>
              <th className="px-5 py-3 font-medium">Members</th>
              <th className="px-5 py-3 font-medium">Visibility</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((b) => (
              <tr key={b.ID} className="border-t border-line">
                <td className="px-5 py-2.5">
                  {b.Name} <span className="text-faint">/{b.Slug}</span>
                </td>
                <td className="px-5 py-2.5">
                  <Badge>{b.Kind}</Badge>
                </td>
                <td className="px-5 py-2.5">{b.MemberCount}</td>
                <td className="px-5 py-2.5 text-muted">{b.Visibility}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </>
  )
}
