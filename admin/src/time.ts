export function ago(iso: string) {
  const s = (Date.now() - Date.parse(iso)) / 1000
  if (s < 0) return `in ${dur(-s)}`
  return `${dur(s)} ago`
}

export function dur(s: number) {
  if (s < 60) return `${Math.round(s)}s`
  if (s < 3600) return `${Math.round(s / 60)}m`
  if (s < 86400) return `${Math.round(s / 3600)}h`
  return `${Math.round(s / 86400)}d`
}
