import { BarChart3, Coffee, FileWarning, Gavel, KeyRound, LogOut, Mic, ShieldAlert, Users } from 'lucide-react'
import type { ReactNode } from 'react'

import { roleRank, type Admin, type Role } from './api'
import { AdminsPage } from './pages/Admins'
import { AppealsPage } from './pages/Appeals'
import { BowlsPage } from './pages/Bowls'
import { CasePage } from './pages/Case'
import { GrievancesPage } from './pages/Grievances'
import { HostsPage } from './pages/Hosts'
import { MetricsPage } from './pages/Metrics'
import { QueuePage } from './pages/Queue'
import { VaultPage } from './pages/Vault'
import { navigate, usePath } from './router'

type Item = { path: string; label: string; icon: ReactNode; min: Role }

const nav: Item[] = [
  { path: '/', label: 'Moderation queue', icon: <ShieldAlert size={18} />, min: 'moderator' },
  { path: '/appeals', label: 'Appeals', icon: <Gavel size={18} />, min: 'moderator' },
  { path: '/grievances', label: 'Grievances', icon: <FileWarning size={18} />, min: 'trust_safety_lead' },
  { path: '/hosts', label: 'Studio hosts', icon: <Mic size={18} />, min: 'moderator' },
  { path: '/bowls', label: 'Bowls', icon: <Coffee size={18} />, min: 'moderator' },
  { path: '/vault', label: 'PII vault', icon: <KeyRound size={18} />, min: 'trust_safety_lead' },
  { path: '/metrics', label: 'Metrics', icon: <BarChart3 size={18} />, min: 'moderator' },
  { path: '/admins', label: 'Team', icon: <Users size={18} />, min: 'admin' },
]

export function Shell({ me, onLogout }: { me: Admin; onLogout: () => void }) {
  const path = usePath()
  const allowed = nav.filter((n) => roleRank[me.role] >= roleRank[n.min])

  let page: ReactNode
  if (path.startsWith('/cases/')) page = <CasePage id={path.slice(7)} me={me} />
  else if (path === '/appeals') page = <AppealsPage />
  else if (path === '/grievances') page = <GrievancesPage />
  else if (path === '/hosts') page = <HostsPage />
  else if (path === '/bowls') page = <BowlsPage me={me} />
  else if (path === '/vault') page = <VaultPage />
  else if (path === '/metrics') page = <MetricsPage />
  else if (path === '/admins') page = <AdminsPage me={me} />
  else page = <QueuePage />

  return (
    <div className="flex min-h-full">
      <aside className="flex w-60 shrink-0 flex-col border-r border-line bg-surface">
        <div className="flex items-center gap-2.5 px-5 py-5">
          <span className="grid h-8 w-8 place-items-center rounded-lg bg-accent text-sm font-black text-accent-ink">M</span>
          <span className="font-bold">Trust &amp; Safety</span>
        </div>
        <nav className="flex-1 space-y-0.5 px-3">
          {allowed.map((n) => {
            const active = n.path === '/' ? path === '/' || path.startsWith('/cases') : path.startsWith(n.path)
            return (
              <a
                key={n.path}
                href={`/admin${n.path}`}
                onClick={(e) => {
                  e.preventDefault()
                  navigate(n.path)
                }}
                className={`flex items-center gap-3 rounded-lg px-3 py-2 text-sm ${active ? 'bg-raised text-ink' : 'text-muted hover:bg-raised hover:text-ink'}`}
              >
                {n.icon}
                {n.label}
              </a>
            )
          })}
        </nav>
        <div className="border-t border-line p-4 text-xs">
          <p className="truncate text-ink">{me.email}</p>
          <p className="mb-3 text-muted">{me.role.replaceAll('_', ' ')}</p>
          <button onClick={onLogout} className="flex items-center gap-2 text-muted hover:text-danger">
            <LogOut size={14} /> Sign out
          </button>
        </div>
      </aside>
      <main className="min-w-0 flex-1 p-8">{page}</main>
    </div>
  )
}
