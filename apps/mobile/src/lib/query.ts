import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query';

import { ApiError } from '@/api/client';
import { toast } from '@/components/ui/Toast';

function report(e: unknown) {
  if (e instanceof ApiError && e.status === 401) return;
  toast(e instanceof Error ? e.message : 'Something went wrong');
}

export const queryClient = new QueryClient({
  // Queries marked silent handle their own errors (e.g. a consent prompt instead of a toast).
  queryCache: new QueryCache({ onError: (e, q) => !q.meta?.silent && report(e) }),
  mutationCache: new MutationCache({
    onError: (e, _v, _c, m) => {
      if (!m.options.onError) report(e);
    },
  }),
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: (count, e) => !(e instanceof ApiError && e.status >= 400 && e.status < 500) && count < 2,
    },
  },
});
