import { useEffect, useState } from 'react'

import { api } from '../api'
import { Card, PageHeader, Select } from '../components/ui'

type Point = { day: string; count: number }
type Metrics = { days: number; series: Record<string, Point[]>; now: Record<string, number>; ai_cost: { today_usd: number; by_purpose_usd: Record<string, number> } }

const SERIES: [string, string][] = [
  ['signups', 'Sign-ups'],
  ['active_sessions', 'Active sessions (consented)'],
  ['posts', 'Posts'],
  ['comments', 'Comments'],
  ['messages', 'Messages'],
  ['matches', 'Travel matches'],
  ['trips_met', 'Trips that met'],
  ['episodes', 'Studio episodes'],
  ['reports', 'Reports'],
  ['sos', 'SOS alerts'],
]

const NOW: [string, string][] = [
  ['users', 'Active accounts'],
  ['open_cases', 'Open cases'],
  ['overdue_cases', 'Cases past SLA'],
  ['open_appeals', 'Open appeals'],
  ['open_grievances', 'Open grievances'],
  ['host_applications', 'Host applications'],
  ['live_episodes', 'Live episodes'],
  ['active_intents', 'Travel Now active'],
]

function days(n: number) {
  const out: string[] = []
  const ist = new Date(Date.now() + 330 * 60_000)
  for (let i = n - 1; i >= 0; i--) out.push(new Date(ist.getTime() - i * 86_400_000).toISOString().slice(0, 10))
  return out
}

function Spark({ label, points, range }: { label: string; points: Point[]; range: string[] }) {
  const byDay = Object.fromEntries(points.map((p) => [p.day, p.count]))
  const vals = range.map((d) => byDay[d] ?? 0)
  const max = Math.max(1, ...vals)
  const total = vals.reduce((a, b) => a + b, 0)
  return (
    <Card className="p-4">
      <div className="mb-2 flex items-baseline justify-between">
        <p className="text-sm text-muted">{label}</p>
        <p className="text-lg font-semibold">{total.toLocaleString()}</p>
      </div>
      <div className="flex h-16 items-end gap-0.5">
        {vals.map((v, i) => (
          <div key={range[i]} title={`${range[i]}: ${v}`} className="flex-1 rounded-sm bg-brand/70" style={{ height: `${Math.max(2, (v / max) * 100)}%` }} />
        ))}
      </div>
    </Card>
  )
}

export function MetricsPage() {
  const [n, setN] = useState(14)
  const [m, setM] = useState<Metrics>()
  useEffect(() => {
    api<Metrics>('GET', `/metrics?days=${n}`).then(setM)
  }, [n])
  const range = days(n)
  return (
    <>
      <PageHeader
        title="Metrics"
        sub="Aggregate counts only. Days are in IST."
        right={
          <Select value={n} onChange={(e) => setN(Number(e.target.value))}>
            <option value={7}>Last 7 days</option>
            <option value={14}>Last 14 days</option>
            <option value={30}>Last 30 days</option>
            <option value={90}>Last 90 days</option>
          </Select>
        }
      />
      {m ? (
        <div className="space-y-8">
          <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
            {NOW.map(([k, label]) => (
              <Card key={k} className="p-4">
                <p className="text-sm text-muted">{label}</p>
                <p className={`text-2xl font-semibold ${k === 'overdue_cases' && m.now[k] > 0 ? 'text-danger' : ''}`}>{(m.now[k] ?? 0).toLocaleString()}</p>
              </Card>
            ))}
          </div>
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {SERIES.map(([k, label]) => (
              <Spark key={k} label={label} points={m.series[k] ?? []} range={range} />
            ))}
          </div>
          <Card className="p-4">
            <p className="mb-2 text-sm text-muted">AI spend today (UTC)</p>
            <p className="mb-3 text-2xl font-semibold">${m.ai_cost.today_usd.toFixed(4)}</p>
            <ul className="space-y-1 text-sm">
              {Object.entries(m.ai_cost.by_purpose_usd).map(([k, v]) => (
                <li key={k} className="flex justify-between">
                  <span className="text-muted">{k.replaceAll('_', ' ')}</span>
                  <span>${v.toFixed(4)}</span>
                </li>
              ))}
            </ul>
          </Card>
        </div>
      ) : null}
    </>
  )
}
