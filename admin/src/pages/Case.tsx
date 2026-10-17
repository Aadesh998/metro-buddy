import { ArrowLeft } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'

import { api, ApiError, roleRank, type Admin } from '../api'
import { Badge, Button, Card, ErrorText, Input, PageHeader, Select, Textarea } from '../components/ui'
import { navigate } from '../router'
import { ago } from '../time'
import type { CaseRow } from './Queue'

type Detail = {
  case: CaseRow & { ai_rationale: string | null; target_id: string }
  reports: { reason: string; details: string; created_at: string }[]
  target: { kind: string; title?: string; body: string; context?: string; status?: string; media?: string[] }
  persona: null | {
    id: string
    alias: string
    bio: string
    user_status: string
    account_age_days: number
    trust_score: number
    trust_tier: number
    verified: boolean
    prior_actions: number
    open_cases: number
    is_host: boolean
  }
  actions: { id: string; action: string; admin_email: string; until: string | null; note: string; created_at: string }[]
}

const ACTIONS: { key: string; label: string; min: Admin['role']; danger?: boolean }[] = [
  { key: 'remove_content', label: 'Remove content', min: 'moderator' },
  { key: 'warn', label: 'Warn', min: 'moderator' },
  { key: 'mute_24h', label: 'Mute 24h', min: 'moderator' },
  { key: 'suspend', label: 'Suspend', min: 'moderator', danger: true },
  { key: 'ban', label: 'Ban', min: 'trust_safety_lead', danger: true },
  { key: 'revoke_host', label: 'Revoke host', min: 'moderator' },
  { key: 'restore', label: 'Restore', min: 'moderator' },
]

export function CasePage({ id, me }: { id: string; me: Admin }) {
  const [d, setD] = useState<Detail>()
  const [action, setAction] = useState('remove_content')
  const [note, setNote] = useState('')
  const [days, setDays] = useState(7)
  const [error, setError] = useState<string>()
  const [busy, setBusy] = useState(false)

  const load = useCallback(() => api<Detail>('GET', `/cases/${id}`).then(setD), [id])
  useEffect(() => {
    load()
  }, [load])

  async function run(path: string, body: unknown, confirmText?: string) {
    if (confirmText && !window.confirm(confirmText)) return
    setBusy(true)
    setError(undefined)
    try {
      setD(await api<Detail>('POST', `/cases/${id}/${path}`, body))
      setNote('')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed')
    } finally {
      setBusy(false)
    }
  }

  if (!d) return null
  const c = d.case
  const closed = c.status === 'actioned' || c.status === 'dismissed'
  const chosen = ACTIONS.find((a) => a.key === action)!

  return (
    <>
      <button onClick={() => navigate('/')} className="mb-4 flex items-center gap-1 text-sm text-muted hover:text-ink">
        <ArrowLeft size={14} /> Queue
      </button>
      <PageHeader
        title={`${c.top_reason ? c.top_reason.replaceAll('_', ' ') : c.source} · ${c.target_type}`}
        sub={`Case ${c.id.slice(0, 8)} · opened ${ago(c.created_at)} · SLA ${ago(c.sla_due_at)}`}
        right={
          <div className="flex gap-2">
            <Badge tone={c.severity}>{c.severity}</Badge>
            <Badge tone={c.status}>{c.status.replace('_', ' ')}</Badge>
          </div>
        }
      />
      <div className="grid gap-5 lg:grid-cols-3">
        <div className="space-y-5 lg:col-span-2">
          <Card className="p-5">
            <p className="mb-2 text-xs uppercase tracking-wide text-muted">
              Reported {d.target.kind} {d.target.context ? `· ${d.target.context}` : ''} {d.target.status ? `· ${d.target.status}` : ''}
            </p>
            {d.target.title ? <h2 className="mb-2 text-lg font-semibold">{d.target.title}</h2> : null}
            <p className="whitespace-pre-wrap text-sm leading-6">{d.target.body || <span className="text-muted">(no text)</span>}</p>
            {d.target.media?.length ? <p className="mt-3 text-xs text-muted">{d.target.media.length} image(s) attached</p> : null}
            <p className="mt-4 text-xs text-faint">Shown masked. Use the PII vault (leads only) if a legal request needs the original.</p>
          </Card>
          <Card className="border-sky/40 p-5">
            <div className="mb-1 flex items-center justify-between">
              <p className="text-xs uppercase tracking-wide text-sky">AI suggestion (not a decision){c.ai_severity ? ` · ${c.ai_severity}` : ''}</p>
              <button
                className="text-xs text-brand"
                onClick={async () => {
                  await api('POST', `/cases/${id}/triage`).catch(() => {})
                  setTimeout(load, 4000)
                }}
              >
                {c.ai_severity ? 'Ask again' : 'Ask AI'}
              </button>
            </div>
            <p className="text-sm text-muted">{c.ai_rationale ?? 'No suggestion yet. The author may have turned off AI processing.'}</p>
          </Card>
          <Card className="p-5">
            <p className="mb-3 text-sm font-semibold">{d.reports.length} report(s)</p>
            <ul className="space-y-2 text-sm">
              {d.reports.map((r, i) => (
                <li key={i} className="flex gap-3">
                  <Badge>{r.reason.replaceAll('_', ' ')}</Badge>
                  <span className="text-muted">{r.details || '—'}</span>
                  <span className="ml-auto text-xs text-faint">{ago(r.created_at)}</span>
                </li>
              ))}
            </ul>
          </Card>
          <Card className="p-5">
            <p className="mb-3 text-sm font-semibold">Decision</p>
            {c.status === 'open' ? (
              <Button kind="secondary" className="mb-4" onClick={() => run('claim', {})}>
                Claim case
              </Button>
            ) : null}
            <div className="space-y-3">
              <div className="flex flex-wrap gap-3">
                <Select value={action} onChange={(e) => setAction(e.target.value)}>
                  {ACTIONS.filter((a) => roleRank[me.role] >= roleRank[a.min]).map((a) => (
                    <option key={a.key} value={a.key}>
                      {a.label}
                    </option>
                  ))}
                </Select>
                {action === 'suspend' ? (
                  <label className="flex items-center gap-2 text-sm">
                    for <Input type="number" min={1} max={365} value={days} onChange={(e) => setDays(Number(e.target.value))} className="w-20" /> days
                  </label>
                ) : null}
              </div>
              <Textarea rows={3} placeholder="Note for the record (shown to reviewers of any appeal)" value={note} onChange={(e) => setNote(e.target.value)} />
              <ErrorText error={error} />
              <div className="flex flex-wrap gap-2">
                <Button
                  kind={chosen.danger ? 'danger' : 'primary'}
                  disabled={busy || note.length < 3 || (closed && action !== 'restore')}
                  onClick={() =>
                    run('actions', { action, note, until_days: action === 'suspend' ? days : undefined }, chosen.danger ? `${chosen.label} ${d.persona?.alias ?? 'this account'}?` : undefined)
                  }
                >
                  {chosen.label}
                </Button>
                <Button kind="secondary" disabled={busy || closed || note.length < 3} onClick={() => run('dismiss', { note })}>
                  Dismiss
                </Button>
                <Button kind="ghost" disabled={busy || closed || note.length < 3} onClick={() => run('escalate', { note })}>
                  Escalate to lead
                </Button>
              </div>
            </div>
          </Card>
        </div>
        <div className="space-y-5">
          {d.persona ? (
            <Card className="p-5 text-sm">
              <p className="mb-1 text-xs uppercase tracking-wide text-muted">Author</p>
              <p className="text-base font-semibold">
                {d.persona.alias} {d.persona.verified ? <Badge tone="open">verified</Badge> : null} {d.persona.is_host ? <Badge tone="medium">host</Badge> : null}
              </p>
              <p className="mb-3 text-muted">{d.persona.bio}</p>
              <dl className="grid grid-cols-2 gap-y-1.5">
                <dt className="text-muted">Account</dt>
                <dd>{d.persona.user_status}</dd>
                <dt className="text-muted">Age</dt>
                <dd>{d.persona.account_age_days} days</dd>
                <dt className="text-muted">Trust</dt>
                <dd>
                  {d.persona.trust_score} (tier {d.persona.trust_tier})
                </dd>
                <dt className="text-muted">Prior actions</dt>
                <dd className={d.persona.prior_actions ? 'text-danger' : ''}>{d.persona.prior_actions}</dd>
                <dt className="text-muted">Open cases</dt>
                <dd>{d.persona.open_cases}</dd>
              </dl>
            </Card>
          ) : null}
          <Card className="p-5 text-sm">
            <p className="mb-3 text-xs uppercase tracking-wide text-muted">History</p>
            {d.actions.length === 0 ? <p className="text-muted">No actions yet.</p> : null}
            <ol className="space-y-3">
              {d.actions.map((a) => (
                <li key={a.id}>
                  <p className="font-medium">{a.action.replaceAll('_', ' ')}</p>
                  <p className="text-xs text-muted">
                    {a.admin_email} · {ago(a.created_at)}
                    {a.until ? ` · until ${new Date(a.until).toLocaleString()}` : ''}
                  </p>
                  <p className="text-xs text-muted">{a.note}</p>
                </li>
              ))}
            </ol>
          </Card>
        </div>
      </div>
    </>
  )
}
