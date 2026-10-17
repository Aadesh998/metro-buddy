import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query';

import { ApiError } from '@/api/client';
import { toast } from '@/components/ui/Toast';

function report(e: unknown) {
  if (e instanceof ApiError && e.status === 401) return;
  toast(e instanceof Error ? e.message : 'Something went wrong');
}

// Keep at most this many screens' worth of data in memory. Inactive queries (no screen is
// showing them) are evicted least-recently-used first; active ones are never touched.
const MAX_QUERIES = 120;
const lastUsed = new Map<string, number>();

const queryCache = new QueryCache({ onError: (e, q) => !q.meta?.silent && report(e) });

queryCache.subscribe((event) => {
  const hash = event.query.queryHash;
  if (event.type === 'removed') {
    lastUsed.delete(hash);
    return;
  }
  if (event.type === 'added' || event.type === 'observerAdded' || event.type === 'updated') {
    lastUsed.delete(hash);
    lastUsed.set(hash, Date.now());
  }
  if (event.type !== 'added' || lastUsed.size <= MAX_QUERIES) return;
  // Map iteration order is insertion order, so the first inactive entries are the least recent.
  for (const h of lastUsed.keys()) {
    if (lastUsed.size <= MAX_QUERIES) break;
    const q = queryCache.get(h);
    if (q && q.getObserversCount() === 0 && h !== hash) queryCache.remove(q);
  }
});

export const queryClient = new QueryClient({
  // Queries marked silent handle their own errors (e.g. a consent prompt instead of a toast).
  queryCache,
  mutationCache: new MutationCache({
    onError: (e, _v, _c, m) => {
      if (!m.options.onError) report(e);
    },
  }),
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      gcTime: 10 * 60_000,
      retry: (count, e) => !(e instanceof ApiError && e.status >= 400 && e.status < 500) && count < 2,
    },
  },
});
