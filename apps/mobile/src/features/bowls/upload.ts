import { ApiError, post } from '@/api/client';
import { t } from '@/i18n';

type Ticket = { key: string; upload_url: string; method: string; headers: Record<string, string> };

// Uploads go straight to object storage with a pre-signed URL; the API only sees the key.
export async function uploadImage(asset: { uri: string; mimeType?: string; fileSize?: number }) {
  const blob = await (await fetch(asset.uri)).blob();
  const mime = asset.mimeType ?? blob.type ?? 'image/jpeg';
  const ticket = await post<Ticket>('/v1/uploads', { mime, size: asset.fileSize ?? blob.size });
  const res = await fetch(ticket.upload_url, { method: ticket.method, headers: ticket.headers, body: blob });
  if (!res.ok) throw new ApiError(res.status, 'upload_failed', t('bowls.uploadFailed'));
  return ticket.key;
}
