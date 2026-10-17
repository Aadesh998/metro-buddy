import { useSyncExternalStore } from 'react';

import { useNow } from '@/lib/clock';

// Tiny external stores for ephemeral realtime state that shouldn't live in the query cache.
function createStore<T>(initial: T) {
  let state = initial;
  const listeners = new Set<() => void>();
  return {
    get: () => state,
    set: (fn: (s: T) => T) => {
      state = fn(state);
      listeners.forEach((l) => l());
    },
    subscribe: (l: () => void) => {
      listeners.add(l);
      return () => {
        listeners.delete(l);
      };
    },
  };
}

export const typingStore = createStore<Record<string, Record<string, number>>>({});
export const presenceStore = createStore<Record<string, boolean>>({});
export const receiptStore = createStore<Record<string, Record<string, { read?: string; delivered?: string }>>>({});

export function useTyping(conversationId: string) {
  const all = useSyncExternalStore(typingStore.subscribe, typingStore.get);
  const now = useNow(1000);
  return Object.entries(all[conversationId] ?? {})
    .filter(([, exp]) => exp > now)
    .map(([persona]) => persona);
}

export function useOnline(personaId?: string) {
  const all = useSyncExternalStore(presenceStore.subscribe, presenceStore.get);
  return personaId ? !!all[personaId] : false;
}

export function useReceipts(conversationId: string) {
  const all = useSyncExternalStore(receiptStore.subscribe, receiptStore.get);
  return all[conversationId] ?? {};
}

let active: string | null = null;
export const activeConversation = {
  get: () => active,
  set: (id: string | null) => {
    active = id;
  },
};

// Raised when something the user just sent suggests distress; the chat screen shows helplines.
export const supportStore = createStore<number>(0);

export function useSupportSignal() {
  return useSyncExternalStore(supportStore.subscribe, supportStore.get);
}
