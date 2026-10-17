import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { ApiError, get, post } from '@/api/client';
import type { CommutePlan, Match, TravelIntent, Trip } from '@/api/types';

export const travelKeys = {
  plans: ['plans'] as const,
  intent: ['intent'] as const,
  matches: ['matches'] as const,
  trip: ['trip', 'active'] as const,
  tripById: (id: string) => ['trip', id] as const,
  contacts: ['contacts'] as const,
};

export function needsConsent(e: unknown) {
  return e instanceof ApiError && e.code === 'consent_required';
}

const quiet = { retry: false, meta: { silent: true } };

export function usePlans() {
  return useQuery({ queryKey: travelKeys.plans, queryFn: () => get<{ items: CommutePlan[] }>('/v1/commute-plans').then((r) => r.items), ...quiet });
}

export function useActiveIntent() {
  return useQuery({ queryKey: travelKeys.intent, queryFn: () => get<{ intent: TravelIntent | null }>('/v1/travel-intents/active').then((r) => r.intent), ...quiet });
}

export function useMatches() {
  return useQuery({
    queryKey: travelKeys.matches,
    queryFn: () => get<{ items: Match[] }>('/v1/matches').then((r) => r.items),
    refetchInterval: 30_000,
    ...quiet,
  });
}

export function useActiveTrip() {
  return useQuery({ queryKey: travelKeys.trip, queryFn: () => get<{ trip: Trip | null }>('/v1/trips/active').then((r) => r.trip), refetchInterval: 30_000 });
}

export function useTrip(id: string) {
  return useQuery({ queryKey: travelKeys.tripById(id), queryFn: () => get<Trip>(`/v1/trips/${id}`), refetchInterval: 20_000 });
}

export function useRespondMatch() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, accept }: { id: string; accept: boolean }) => post<Match | undefined>(`/v1/matches/${id}/${accept ? 'accept' : 'decline'}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: travelKeys.matches });
      qc.invalidateQueries({ queryKey: travelKeys.trip });
      qc.invalidateQueries({ queryKey: ['conversations'] });
    },
  });
}
