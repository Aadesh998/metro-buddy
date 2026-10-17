import type { RemoteTrack } from 'livekit-client';

const elements = new Set<HTMLMediaElement>();

export async function startAudio() {}
export async function stopAudio() {}

export function attachAudio(track: RemoteTrack) {
  if (track.kind !== 'audio') return;
  const el = track.attach();
  el.style.display = 'none';
  document.body.appendChild(el);
  elements.add(el);
}

export function detachAll() {
  elements.forEach((el) => el.remove());
  elements.clear();
}
