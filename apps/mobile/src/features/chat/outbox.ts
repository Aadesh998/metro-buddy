import AsyncStorage from '@react-native-async-storage/async-storage';

import { ApiError, post } from '@/api/client';
import type { Message } from '@/api/types';

const KEY = 'mb.outbox';

export type Pending = { conversation_id: string; id: string; body: string; created_at: string };

let queue: Pending[] = [];
let loaded = false;
let flushing = false;
const listeners = new Set<() => void>();

async function load() {
  if (loaded) return;
  try {
    queue = JSON.parse((await AsyncStorage.getItem(KEY)) ?? '[]');
  } catch {
    queue = [];
  }
  loaded = true;
}

async function save() {
  await AsyncStorage.setItem(KEY, JSON.stringify(queue)).catch(() => {});
  listeners.forEach((l) => l());
}

export function onOutboxChange(fn: () => void) {
  listeners.add(fn);
  return () => {
    listeners.delete(fn);
  };
}

export async function pendingFor(conversationId: string) {
  await load();
  return queue.filter((p) => p.conversation_id === conversationId);
}

export async function enqueue(p: Pending) {
  await load();
  queue.push(p);
  await save();
}

// Messages persist across restarts and are retried with the same id, which the server de-duplicates.
export async function flush(onSent: (p: Pending, m: Message) => void, onFailed: (p: Pending, e: ApiError) => void) {
  await load();
  if (flushing || queue.length === 0) return;
  flushing = true;
  try {
    for (const p of [...queue]) {
      try {
        const m = await post<Message>(`/v1/conversations/${p.conversation_id}/messages`, { id: p.id, body: p.body });
        queue = queue.filter((q) => q.id !== p.id);
        await save();
        onSent(p, m);
      } catch (e) {
        if (e instanceof ApiError && e.status !== 0 && e.status !== 429 && e.status < 500) {
          queue = queue.filter((q) => q.id !== p.id);
          await save();
          onFailed(p, e);
          continue;
        }
        break;
      }
    }
  } finally {
    flushing = false;
  }
}

export async function clearOutbox() {
  queue = [];
  loaded = true;
  await AsyncStorage.removeItem(KEY).catch(() => {});
}
