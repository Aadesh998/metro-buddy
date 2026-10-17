import { useSyncExternalStore } from 'react';

// One shared ticking clock. Components read time through it instead of calling Date.now()
// during render, which keeps renders pure and re-renders only when their bucket changes.
let now = Date.now();
const listeners = new Set<() => void>();
setInterval(() => {
  now = Date.now();
  listeners.forEach((l) => l());
}, 1000);

function subscribe(l: () => void) {
  listeners.add(l);
  return () => {
    listeners.delete(l);
  };
}

export function useNow(granularityMs = 30_000) {
  return useSyncExternalStore(subscribe, () => Math.floor(now / granularityMs) * granularityMs);
}
