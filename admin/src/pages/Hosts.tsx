import { useEffect, useState } from 'react'

import { api, ApiError, roleRank, type Admin } from '../api'
import { Badge, Button, Card, ErrorText, PageHeader, Select, Textarea } from '../components/ui'
import { ago } from '../time'

type HostApp = { id: string; alias: string; status: string; tagline: string; pitch: string; trust_score: number; reports_90d: number; created_at: string }
type LiveEp = { id: string; title: string; show_title: string; host_alias: string; started_at: string; listeners: number; reports: number; auto_muted: boolean }

function Application({ h, canDecide, onDone }: { h: HostApp; canDecide: boolean; onDone: () => void }) {
  const [note, setNote] = useState('')
  const [error, setError] = useState<string>()
  async function decide(decision: string) {
    setError(undefined)
    try {
      await api('POST', `/hosts/${h.id}/decision`, { decision, note })
      onDone()
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed')
    }
  }
  return (
    <Card className="p-5">
      <div className="mb-2 flex items-center gap-2 text-sm">
        <span className="font-medium">{h.alias}</span>
        <Badge tone={h.trust_score >= 60 ? 'actioned' : h.trust_score < 40 ? 'high' : 'medium'}>trust {h.trust_score}</Badge>
        {h.reports_90d > 0 ? <Badge tone="high">{h.reports_90d} reports in 90d</Badge> : null}
        <span className="ml-auto text-muted">applied {ago(h.created_at)}</span>
      </div>
      <p className="mb-1 font-medium">{h.tagline}</p>
      <p className="mb-3 whitespace-pre-wrap text-sm text-muted">{h.pitch}</p>
      {canDecide ? (
        <>
          <Textarea rows={2} placeholder="Note (kept in the audit log)" value={note} onChange={(e) => setNote(e.target.value)} />
          <ErrorText error={error} />
          <div className="mt-2 flex gap-2">
            {h.status !== 'approved' ? <Button onClick={() => decide('approve')}>Approve</Button> : null}
            {h.status === 'applied' ? (
              <Button kind="secondary" onClick={() => decide('reject')}>
                Reject
              </Button>
            ) : null}
            {h.status === 'approved' ? (
              <Button kind="danger" onClick={() => decide('suspend')}>
                Suspend host
              </Button>
            ) : null}
          </div>
        </>
      ) : (
        <p className="text-xs text-muted">A trust &amp; safety lead decides host applications.</p>
      )}
    </Card>
  )
}

function LiveNow({ canEnd }: { canEnd: boolean }) {
  const [eps, setEps] = useState<LiveEp[]>([])
  const load = () => api<{ items: LiveEp[] }>('GET', '/episodes/live').then((r) => setEps(r.items))
  useEffect(() => {
    load()
    const t = setInterval(load, 15000)
    return () => clearInterval(t)
  }, [])
  if (eps.length === 0) return null
  return (
    <div className="mb-8">
      <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Live now</h2>
      <div className="space-y-2">
        {eps.map((e) => (
          <Card key={e.id} className="flex items-center gap-3 p-4 text-sm">
            <span className="h-2 w-2 rounded-full bg-danger" />
            <div className="min-w-0 flex-1">
              <p className="font-medium">
                {e.show_title} · {e.title}
              </p>
              <p className="text-muted">
                {e.host_alias} · {e.listeners} listening · started {ago(e.started_at)}
              </p>
            </div>
            {e.auto_muted ? <Badge tone="high">auto-muted</Badge> : null}
            {e.reports > 0 ? <Badge tone="medium">{e.reports} reports</Badge> : null}
            {canEnd ? (
              <Button
                kind="danger"
                onClick={async () => {
                  if (!confirm('End this episode for everyone?')) return
                  await api('POST', `/episodes/${e.id}/end`)
                  load()
                }}
              >
                End episode
              </Button>
            ) : null}
          </Card>
        ))}
      </div>
    </div>
  )
}

export function HostsPage({ me }: { me: Admin }) {
  const [status, setStatus] = useState('applied')
  const [rows, setRows] = useState<HostApp[]>([])
  const [tick, setTick] = useState(0)
  const lead = roleRank[me.role] >= roleRank.trust_safety_lead
  useEffect(() => {
    api<{ items: HostApp[] }>('GET', `/hosts?status=${status}`).then((r) => setRows(r.items))
  }, [status, tick])
  return (
    <>
      <PageHeader
        title="Studio hosts"
        sub="Approve hosts who can hold a room safely. Check trust and recent reports first."
        right={
          <Select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="applied">Applications</option>
            <option value="approved">Approved</option>
            <option value="suspended">Suspended</option>
            <option value="rejected">Rejected</option>
          </Select>
        }
      />
      <LiveNow canEnd={lead} />
      <div className="space-y-4">
        {rows.length === 0 ? <p className="text-muted">Nothing here.</p> : null}
        {rows.map((h) => (
          <Application key={h.id} h={h} canDecide={lead} onDone={() => setTick((n) => n + 1)} />
        ))}
      </div>
    </>
  )
}
