import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from 'react'

export function Button({ kind = 'primary', className = '', ...rest }: ButtonHTMLAttributes<HTMLButtonElement> & { kind?: 'primary' | 'secondary' | 'danger' | 'ghost' }) {
  const k = {
    primary: 'bg-accent text-accent-ink hover:brightness-95',
    secondary: 'bg-raised text-ink border border-line hover:bg-line',
    danger: 'bg-danger text-white hover:brightness-95',
    ghost: 'text-accent hover:bg-raised',
  }[kind]
  return <button className={`rounded-lg px-3.5 py-2 text-sm font-semibold disabled:opacity-50 ${k} ${className}`} {...rest} />
}

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={`w-full rounded-lg border border-line bg-surface px-3 py-2 text-sm text-ink outline-none focus:border-accent ${props.className ?? ''}`} />
}

export function Select(props: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select {...props} className={`rounded-lg border border-line bg-surface px-3 py-2 text-sm text-ink ${props.className ?? ''}`} />
}

export function Textarea(props: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea {...props} className={`w-full rounded-lg border border-line bg-surface px-3 py-2 text-sm text-ink outline-none focus:border-accent ${props.className ?? ''}`} />
}

const tones: Record<string, string> = {
  critical: 'bg-danger/20 text-danger',
  high: 'bg-orange-500/20 text-orange-300',
  medium: 'bg-accent/15 text-accent',
  low: 'bg-raised text-muted',
  open: 'bg-sky/15 text-sky',
  in_review: 'bg-accent/15 text-accent',
  actioned: 'bg-mint/15 text-mint',
  dismissed: 'bg-raised text-muted',
  escalated: 'bg-danger/20 text-danger',
}

export function Badge({ children, tone }: { children: ReactNode; tone?: string }) {
  return <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${tones[tone ?? ''] ?? 'bg-raised text-muted'}`}>{children}</span>
}

export function Card({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <div className={`rounded-xl border border-line bg-surface ${className}`}>{children}</div>
}

export function PageHeader({ title, sub, right }: { title: string; sub?: string; right?: ReactNode }) {
  return (
    <div className="mb-5 flex items-end justify-between gap-4">
      <div>
        <h1 className="text-2xl font-bold">{title}</h1>
        {sub ? <p className="text-sm text-muted">{sub}</p> : null}
      </div>
      {right}
    </div>
  )
}

export function ErrorText({ error }: { error?: string }) {
  return error ? <p className="text-sm text-danger">{error}</p> : null
}
