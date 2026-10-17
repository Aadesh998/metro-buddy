import type { Admin } from '../api'
import { PageHeader } from '../components/ui'

export function CasePage({ id }: { id: string; me: Admin }) {
  return <PageHeader title={`Case ${id}`} />
}
