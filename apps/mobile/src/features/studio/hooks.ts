import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect } from 'react';

import { del, get, post } from '@/api/client';
import type { CallIn, Episode, EpisodeQuestion, HostApplication, Show } from '@/api/types';
import { on } from '@/lib/realtime';

export const studioKeys = {
  schedule: ['episodes'] as const,
  shows: (filter: string) => ['shows', filter] as const,
  show: (id: string) => ['show', id] as const,
  episode: (id: string) => ['episode', id] as const,
  queue: (id: string) => ['episode', id, 'queue'] as const,
  questions: (id: string) => ['episode', id, 'questions'] as const,
  application: ['host-application'] as const,
};

export function useSchedule() {
  return useQuery({ queryKey: studioKeys.schedule, queryFn: () => get<{ items: Episode[] }>('/v1/episodes').then((r) => r.items), refetchInterval: 30_000 });
}

export function useShows(filter: '' | 'following' | 'mine') {
  return useQuery({ queryKey: studioKeys.shows(filter), queryFn: () => get<{ items: Show[] }>(`/v1/shows${filter ? `?${filter}=1` : ''}`).then((r) => r.items) });
}

export function useShow(id: string) {
  return useQuery({ queryKey: studioKeys.show(id), queryFn: () => get<Show>(`/v1/shows/${id}`) });
}

export function useEpisode(id: string) {
  return useQuery({ queryKey: studioKeys.episode(id), queryFn: () => get<Episode>(`/v1/episodes/${id}`), refetchInterval: 10_000 });
}

export function useQueue(id: string, enabled: boolean) {
  return useQuery({ queryKey: studioKeys.queue(id), queryFn: () => get<{ items: CallIn[] }>(`/v1/episodes/${id}/queue`).then((r) => r.items), enabled, refetchInterval: 15_000 });
}

export function useQuestions(id: string, enabled: boolean) {
  return useQuery({
    queryKey: studioKeys.questions(id),
    queryFn: () => get<{ items: EpisodeQuestion[] }>(`/v1/episodes/${id}/questions`).then((r) => r.items),
    enabled,
  });
}

export function useHostApplication() {
  return useQuery({ queryKey: studioKeys.application, queryFn: () => get<{ application: HostApplication | null }>('/v1/studio/host-application').then((r) => r.application) });
}

export function useFollow(show: Show) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => (show.following ? del<Show>(`/v1/shows/${show.id}/follow`) : post<Show>(`/v1/shows/${show.id}/follow`)),
    onSuccess: (s) => {
      if (s) qc.setQueryData(studioKeys.show(show.id), s);
      qc.invalidateQueries({ queryKey: ['shows'] });
    },
  });
}

// Keeps an open episode screen in sync with what the server pushes over the socket.
export function useEpisodeEvents(id: string, onReaction: (r: string) => void) {
  const qc = useQueryClient();
  useEffect(() => {
    const mine = (d: { episode_id?: string }) => d?.episode_id === id;
    const refresh = () => qc.invalidateQueries({ queryKey: studioKeys.episode(id) });
    const offs = [
      on('episode.stage', (d) => mine(d) && refresh()),
      on('episode.ended', (d) => mine(d) && refresh()),
      on('episode.auto_muted', (d) => mine(d) && refresh()),
      on('episode.queue', (d) => {
        if (!mine(d)) return;
        qc.invalidateQueries({ queryKey: studioKeys.queue(id) });
        refresh();
      }),
      on('episode.question', (d) => mine(d) && qc.invalidateQueries({ queryKey: studioKeys.questions(id) })),
      on('episode.reaction', (d) => mine(d) && onReaction(d.reaction)),
    ];
    return () => offs.forEach((off) => off());
  }, [id, qc, onReaction]);
}

export type TopicIdeas = { ideas: { topic: string; hook: string }[]; themes: string[]; from_ai: boolean; updated_at: string };

export function useTopicIdeas(enabled: boolean) {
  return useQuery({ queryKey: ['topic-ideas'], queryFn: () => get<TopicIdeas>('/v1/studio/topic-ideas'), enabled, staleTime: 30 * 60_000 });
}
