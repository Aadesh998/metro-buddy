import { useEffect, useState } from 'react'

import { api, ApiError } from '../api'
import { Badge, Button, Card, ErrorText, PageHeader, Select, Textarea } from '../components/ui'
import { navigate } from '../router'
import { ago } from '../time'

type Appeal = { id: string; text: string; status: string; action: string; action_note: string; case_id: string; sla_due_at: string; overdue: boolean; created_at: string }

function Row({ a, onDone }: { a: Appeal; onDone: (rows: Appeal[]) => void }) {
  const [note, setNote] = useState('')
  const [error, setError] = useState<string>()
  async function decide(decision: string) {
    setError(undefined)
    try {
      onDone((await api<{ items: Appeal[] }>('POST', `/appeals/${a.id}/decide`, { decision, note })).items)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed')
    }
  }
  return (
    <Card className="p-5">
      <div className="mb-2 flex items-center gap-2 text-sm">
        <Badge tone="medium">{a.action.replaceAll('_', ' ')}</Badge>
        <span className={a.overdue ? 'text-danger' : 'text-muted'}>decide {ago(a.sla_due_at)}</span>
        <button className="ml-auto text-xs text-brand" onClick={() => navigate(`/cases/${a.case_id}`)}>
          Open case
        </button>
      </div>
      <p className="mb-1 text-xs text-muted">Original note: {a.action_note}</p>
      <p className="mb-3 whitespace-pre-wrap text-sm">“{a.text}”</p>
      {a.status === 'open' ? (
        <>
          <Textarea rows={2} placeholder="Reason for your decision" value={note} onChange={(e) => setNote(e.target.value)} />
          <ErrorText error={error} />
          <div className="mt-2 flex gap-2">
            <Button kind="secondary" disabled={note.length < 3} onClick={() => decide('upheld')}>
              Uphold action
            </Button>
            <Button disabled={note.length < 3} onClick={() => decide('overturned')}>
              Overturn and restore
            </Button>
          </div>
        </>
      ) : (
        <Badge tone={a.status === 'overturned' ? 'actioned' : 'dismissed'}>{a.status}</Badge>
      )}
    </Card>
  )
}

export function AppealsPage() {
  const [status, setStatus] = useState('open')
  const [rows, setRows] = useState<Appeal[]>([])
  useEffect(() => {
    api<{ items: Appeal[] }>('GET', `/appeals?status=${status}`).then((r) => setRows(r.items))
  }, [status])
  return (
    <>
      <PageHeader
        title="Appeals"
        sub="A different team member from the one who acted must decide. Target: 15 days."
        right={
          <Select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="open">Open</option>
            <option value="closed">Decided</option>
          </Select>
        }
      />
      <div className="space-y-4">
        {rows.length === 0 ? <p className="text-muted">No appeals here.</p> : null}
        {rows.map((a) => (
          <Row key={a.id} a={a} onDone={(r) => setRows(status === 'open' ? r : rows)} />
        ))}
      </div>
    </>
  )
}
