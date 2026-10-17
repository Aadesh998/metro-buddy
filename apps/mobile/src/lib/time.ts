export function ago(iso: string) {
  const s = Math.max(0, (Date.now() - Date.parse(iso)) / 1000);
  if (s < 60) return 'now';
  if (s < 3600) return `${Math.floor(s / 60)}m`;
  if (s < 86400) return `${Math.floor(s / 3600)}h`;
  if (s < 86400 * 30) return `${Math.floor(s / 86400)}d`;
  return new Date(iso).toLocaleDateString();
}

export function until(iso: string) {
  const s = (Date.parse(iso) - Date.now()) / 1000;
  if (s <= 0) return 'closed';
  if (s < 3600) return `${Math.ceil(s / 60)}m left`;
  if (s < 86400) return `${Math.floor(s / 3600)}h left`;
  return `${Math.floor(s / 86400)}d left`;
}
