import { getRandomBytes } from 'expo-crypto';

// UUIDv7: 48-bit ms timestamp + random. The server only accepts fresh v7 ids for messages,
// which keeps client-generated ids time-ordered and unforgeable into the past.
export function uuidv7(now = Date.now()): string {
  const b = getRandomBytes(16);
  let ts = now;
  for (let i = 5; i >= 0; i--) {
    b[i] = ts % 256;
    ts = Math.floor(ts / 256);
  }
  b[6] = (b[6] & 0x0f) | 0x70;
  b[8] = (b[8] & 0x3f) | 0x80;
  const h = Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('');
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}
