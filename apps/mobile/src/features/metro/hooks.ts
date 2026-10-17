import { useQuery } from '@tanstack/react-query';

import { get } from '@/api/client';
import type { City, Station } from '@/api/types';

export function useCities() {
  return useQuery({
    queryKey: ['cities'],
    queryFn: () => get<{ items: City[] }>('/v1/cities').then((r) => r.items),
    staleTime: Infinity,
  });
}

// The whole city station list is small (~300) and searched on-device for instant results.
export function useStations(cityId?: string) {
  return useQuery({
    queryKey: ['stations', cityId],
    queryFn: () => get<{ items: Station[] }>(`/v1/cities/${cityId}/stations`).then((r) => r.items),
    enabled: !!cityId,
    staleTime: Infinity,
    gcTime: Infinity,
  });
}

export function normalize(s: string) {
  return s.toLowerCase().replace(/[^a-z0-9]+/g, ' ').trim();
}

export function searchStations(all: Station[], q: string): Station[] {
  const nq = normalize(q);
  if (!nq) return all;
  const terms = nq.split(' ');
  const scored: { s: Station; score: number }[] = [];
  for (const s of all) {
    const hay = normalize(s.search_text);
    if (!terms.every((t) => hay.includes(t))) continue;
    const name = normalize(s.name);
    const score = name.startsWith(nq) ? 0 : hay.split(' ').some((w) => w.startsWith(terms[0])) ? 1 : 2;
    scored.push({ s, score });
  }
  scored.sort((a, b) => a.score - b.score || a.s.name.localeCompare(b.s.name));
  return scored.map((x) => x.s);
}
