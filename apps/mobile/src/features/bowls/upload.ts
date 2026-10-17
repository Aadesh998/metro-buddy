import { ApiError, post } from '@/api/client';

type Ticket = { key: string; upload_url: string; method: string; headers: Record<string, string> };

// Uploads go straight to object storage with a pre-signed URL; the API only sees the key.
export async function uploadImage(asset: { uri: string; mimeType?: string; fileSize?: number }) {
  const blob = await (await fetch(asset.uri)).blob();
  const mime = asset.mimeType ?? blob.type ?? 'image/jpeg';
  const t = await post<Ticket>('/v1/uploads', { mime, size: asset.fileSize ?? blob.size });
  const res = await fetch(t.upload_url, { method: t.method, headers: t.headers, body: blob });
  if (!res.ok) throw new ApiError(res.status, 'upload_failed', 'Upload failed. Try again.');
  return t.key;
}
