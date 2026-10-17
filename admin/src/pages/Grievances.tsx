import { useEffect, useState } from 'react'

import { api, ApiError } from '../api'
import { Badge, Button, Card, ErrorText, PageHeader, Select, Textarea } from '../components/ui'
import { ago } from '../time'

type G = {
  id: string
  ref: string
  category: string
  body: string
  contact_email?: string
  from_user: boolean
  acknowledged_at: string | null
  resolved_at: string | null
  ack_due_at: string
  resolve_due_at: string
  ack_overdue: boolean
  resolve_overdue: boolean
  resolution: string | null
  created_at: string
}

function Ticket({ g, onChange }: { g: G; onChange: (g: G) => void }) {
  const [resolution, setResolution] = useState('')
  const [error, setError] = useState<string>()
  async function call(path: string, body: unknown) {
    setError(undefined)
    try {
      onChange(await api<G>('POST', `/grievances/${g.id}/${path}`, body))
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed')
    }
  }
  return (
    <Card className="p-5">
      <div className="mb-2 flex flex-wrap items-center gap-2 text-sm">
        <span className="font-mono font-semibold">{g.ref}</span>
        <Badge tone={g.category === 'ncii' ? 'critical' : 'medium'}>{g.category}</Badge>
        <span className="text-muted">{g.from_user ? 'in-app' : g.contact_email}</span>
        <span className="ml-auto text-xs text-muted">received {ago(g.created_at)}</span>
      </div>
      <p className="mb-3 whitespace-pre-wrap text-sm">{g.body}</p>
      <div className="mb-3 flex gap-4 text-xs">
        <span className={g.ack_overdue ? 'text-danger' : 'text-muted'}>{g.acknowledged_at ? `acknowledged ${ago(g.acknowledged_at)}` : `acknowledge ${ago(g.ack_due_at)}`}</span>
        <span className={g.resolve_overdue ? 'text-danger' : 'text-muted'}>{g.resolved_at ? `resolved ${ago(g.resolved_at)}` : `resolve ${ago(g.resolve_due_at)}`}</span>
      </div>
      {g.resolved_at ? (
        <p className="text-sm text-mint">{g.resolution}</p>
      ) : (
        <div className="space-y-2">
          {!g.acknowledged_at ? (
            <Button kind="secondary" onClick={() => call('ack', {})}>
              Acknowledge
            </Button>
          ) : null}
          <Textarea rows={2} placeholder="Resolution (what we did)" value={resolution} onChange={(e) => setResolution(e.target.value)} />
          <Button disabled={resolution.length < 10} onClick={() => call('resolve', { resolution })}>
            Resolve
          </Button>
          <ErrorText error={error} />
        </div>
      )}
    </Card>
  )
}

export function GrievancesPage() {
  const [status, setStatus] = useState('open')
  const [rows, setRows] = useState<G[]>([])
  useEffect(() => {
    api<{ items: G[] }>('GET', `/grievances?status=${status}`).then((r) => setRows(r.items))
  }, [status])
  return (
    <>
      <PageHeader
        title="Grievances"
        sub="IT Rules: acknowledge within 24 hours, resolve within 15 days (intimate images within 24 hours)."
        right={
          <Select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="open">Open</option>
            <option value="closed">Resolved</option>
          </Select>
        }
      />
      <div className="space-y-4">
        {rows.length === 0 ? <p className="text-muted">Nothing here.</p> : null}
        {rows.map((g) => (
          <Ticket key={g.id} g={g} onChange={(n) => setRows(rows.map((x) => (x.id === n.id ? n : x)))} />
        ))}
      </div>
    </>
  )
}
