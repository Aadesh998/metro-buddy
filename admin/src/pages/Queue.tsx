import { AlertTriangle, Clock } from 'lucide-react'
import { useEffect, useState } from 'react'

import { api } from '../api'
import { Badge, Card, PageHeader, Select } from '../components/ui'
import { navigate } from '../router'
import { ago } from '../time'

export type CaseRow = {
  id: string
  target_type: string
  severity: string
  ai_severity: string | null
  status: string
  source: string
  report_count: number
  assignee_email: string | null
  sla_due_at: string
  overdue: boolean
  created_at: string
  top_reason: string
}

export function QueuePage() {
  const [status, setStatus] = useState('active')
  const [mine, setMine] = useState(false)
  const [data, setData] = useState<{ items: CaseRow[]; open: number; overdue: number }>()

  useEffect(() => {
    const load = () => api<typeof data>('GET', `/cases?status=${status}${mine ? '&mine=1' : ''}`).then(setData)
    load()
    const t = setInterval(load, 20000)
    return () => clearInterval(t)
  }, [status, mine])

  return (
    <>
      <PageHeader
        title="Moderation queue"
        sub={data ? `${data.open} open · ${data.overdue} past SLA` : 'Loading…'}
        right={
          <div className="flex items-center gap-3 text-sm">
            <label className="flex items-center gap-2 text-muted">
              <input type="checkbox" checked={mine} onChange={(e) => setMine(e.target.checked)} /> Assigned to me
            </label>
            <Select value={status} onChange={(e) => setStatus(e.target.value)}>
              <option value="active">Active</option>
              <option value="escalated">Escalated</option>
              <option value="closed">Closed</option>
            </Select>
          </div>
        }
      />
      <Card>
        {data && data.items.length === 0 ? <p className="p-10 text-center text-muted">Queue is clear.</p> : null}
        <table className="w-full text-sm">
          <tbody>
            {data?.items.map((c) => (
              <tr key={c.id} onClick={() => navigate(`/cases/${c.id}`)} className="cursor-pointer border-t border-line first:border-t-0 hover:bg-raised">
                <td className="w-28 py-3 pl-5">
                  <Badge tone={c.severity}>{c.severity}</Badge>
                </td>
                <td className="py-3">
                  <p className="font-medium">
                    {c.top_reason ? c.top_reason.replaceAll('_', ' ') : c.source.replaceAll('_', ' ')} · {c.target_type}
                  </p>
                  <p className="text-xs text-muted">
                    {c.report_count} report{c.report_count === 1 ? '' : 's'} · opened {ago(c.created_at)}
                    {c.ai_severity ? ` · AI suggests ${c.ai_severity}` : ''}
                  </p>
                </td>
                <td className="py-3">
                  <Badge tone={c.status}>{c.status.replace('_', ' ')}</Badge>
                </td>
                <td className="py-3 text-muted">{c.assignee_email ?? 'unassigned'}</td>
                <td className="py-3 pr-5 text-right">
                  <span className={`inline-flex items-center gap-1 ${c.overdue ? 'text-danger' : 'text-muted'}`}>
                    {c.overdue ? <AlertTriangle size={14} /> : <Clock size={14} />}
                    SLA {ago(c.sla_due_at)}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </>
  )
}
