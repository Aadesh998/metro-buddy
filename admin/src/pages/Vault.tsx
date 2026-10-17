import { useState, type FormEvent } from 'react'

import { api, ApiError } from '../api'
import { Button, Card, ErrorText, Input, PageHeader, Select, Textarea } from '../components/ui'

type Entry = { content_type: string; content_id: string; raw: string; expires_at: string }

export function VaultPage() {
  const [ct, setCt] = useState('post')
  const [id, setId] = useState('')
  const [reason, setReason] = useState('')
  const [rows, setRows] = useState<Entry[] | null>(null)
  const [error, setError] = useState<string>()

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError(undefined)
    try {
      const r = await api<{ items: Entry[] }>('POST', '/vault/read', { content_type: ct, content_id: id.trim(), reason })
      setRows(r.items)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed')
    }
  }

  return (
    <>
      <PageHeader title="PII vault" sub="Original, unmasked text for legal requests. Kept 90 days. Every read is logged with your reason." />
      <Card className="mb-6 p-5">
        <form onSubmit={submit} className="space-y-3">
          <div className="flex gap-3">
            <Select value={ct} onChange={(e) => setCt(e.target.value)}>
              {['post', 'comment', 'message', 'persona'].map((x) => (
                <option key={x}>{x}</option>
              ))}
            </Select>
            <Input placeholder="Content ID" value={id} onChange={(e) => setId(e.target.value)} required />
          </div>
          <Textarea rows={2} placeholder="Reason, e.g. police request reference (min 10 characters)" value={reason} onChange={(e) => setReason(e.target.value)} required minLength={10} />
          <Button type="submit" kind="danger">
            Read original text
          </Button>
          <ErrorText error={error} />
        </form>
      </Card>
      {rows ? (
        rows.length === 0 ? (
          <p className="text-muted">Nothing in the vault for that ID (it may have expired).</p>
        ) : (
          rows.map((r, i) => (
            <Card key={i} className="mb-3 p-5">
              <p className="mb-2 text-xs text-muted">Expires {new Date(r.expires_at).toLocaleDateString()}</p>
              <pre className="whitespace-pre-wrap text-sm">{r.raw}</pre>
            </Card>
          ))
        )
      ) : null}
    </>
  )
}
