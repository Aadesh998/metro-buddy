import { useState, type FormEvent } from 'react'

import { api, ApiError, type Admin } from '../api'
import { Button, ErrorText, Input } from '../components/ui'

export function Login({ onDone }: { onDone: (a: Admin) => void }) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')
  const [error, setError] = useState<string>()
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(undefined)
    try {
      onDone(await api<Admin>('POST', '/login', { email, password, code }))
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Login failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="flex min-h-full items-center justify-center p-6">
      <form onSubmit={submit} className="w-full max-w-sm space-y-4 rounded-2xl border border-line bg-surface p-7">
        <div className="mb-2 flex items-center gap-3">
          <span className="grid h-9 w-9 place-items-center rounded-xl bg-accent font-black text-accent-ink">M</span>
          <div>
            <h1 className="text-lg font-bold">Metro Buddy</h1>
            <p className="text-xs text-muted">Trust &amp; Safety console</p>
          </div>
        </div>
        <label className="block space-y-1.5 text-sm">
          <span>Email</span>
          <Input type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} required />
        </label>
        <label className="block space-y-1.5 text-sm">
          <span>Password</span>
          <Input type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </label>
        <label className="block space-y-1.5 text-sm">
          <span>Authenticator code</span>
          <Input inputMode="numeric" autoComplete="one-time-code" maxLength={6} value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))} required />
        </label>
        <ErrorText error={error} />
        <Button type="submit" className="w-full py-2.5" disabled={busy || code.length !== 6}>
          {busy ? 'Signing in…' : 'Sign in'}
        </Button>
        <p className="text-center text-xs text-faint">Every action you take here is audit-logged.</p>
      </form>
    </main>
  )
}
