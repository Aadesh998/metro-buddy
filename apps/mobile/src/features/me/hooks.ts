import { useQuery } from '@tanstack/react-query';

import { get } from '@/api/client';
import type { Me, OwnPersona, Vibe } from '@/api/types';

export const meKeys = {
  me: ['me'] as const,
  persona: ['me', 'persona'] as const,
  vibes: ['me', 'vibes'] as const,
  consents: ['me', 'consents'] as const,
};

export function useMe(enabled = true) {
  return useQuery({ queryKey: meKeys.me, queryFn: () => get<Me>('/v1/me'), enabled });
}

export function useMyPersona(enabled = true) {
  return useQuery({ queryKey: meKeys.persona, queryFn: () => get<OwnPersona>('/v1/me/persona'), enabled });
}

export function useMyVibes(enabled = true) {
  return useQuery({
    queryKey: meKeys.vibes,
    queryFn: () => get<{ items: Vibe[] }>('/v1/me/vibes').then((r) => r.items),
    enabled,
  });
}

export type OnboardingStep = 'age' | 'consents' | 'persona' | 'home-metro' | 'vibes' | null;

export function useOnboardingStep(enabled: boolean): { step: OnboardingStep; loading: boolean } {
  const me = useMe(enabled);
  const hasPersona = !!me.data?.has_persona;
  const persona = useMyPersona(enabled && hasPersona);
  const vibes = useMyVibes(enabled && hasPersona);

  if (!enabled) return { step: null, loading: false };
  if (me.isPending || (hasPersona && (persona.isPending || vibes.isPending))) return { step: null, loading: true };
  const m = me.data;
  if (!m) return { step: null, loading: true };
  if (!m.age_verified) return { step: 'age', loading: false };
  if (!m.core_consent) return { step: 'consents', loading: false };
  if (!m.has_persona) return { step: 'persona', loading: false };
  if (!persona.data?.home_station_id) return { step: 'home-metro', loading: false };
  if ((vibes.data?.length ?? 0) < 3) return { step: 'vibes', loading: false };
  return { step: null, loading: false };
}
