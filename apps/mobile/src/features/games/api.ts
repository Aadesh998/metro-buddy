import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef } from 'react';

import { ApiError, get, post } from '@/api/client';
import type { PersonaRef } from '@/api/types';
import { toast } from '@/components/ui/Toast';
import { on } from '@/lib/realtime';

export type GameName = 'sudoku' | 'scribble' | 'wordchain' | 'chess' | 'imposter' | 'breakwall';
export type RoomStatus = 'lobby' | 'playing' | 'finished' | 'closed';
export type SizeFilter = 'small' | 'group' | 'party';

export type GameInfo = { name: GameName; min_players: number; max_players: number };

export type RoomPlayer = { persona: PersonaRef; spectator: boolean; ready: boolean; seat: number; score: number };

export type Room<V = unknown> = {
  id: string;
  game: GameName;
  visibility: 'public' | 'private';
  code?: string;
  host_id: string;
  min_players: number;
  max_players: number;
  status: RoomStatus;
  version: number;
  deadline_at: string | null;
  players: RoomPlayer[];
  player_count: number;
  me?: RoomPlayer;
  view?: V;
};

export const gameKeys = {
  catalog: ['games', 'catalog'] as const,
  rooms: (game?: string, size?: string) => ['games', 'rooms', game ?? '', size ?? ''] as const,
  room: (id: string) => ['games', 'room', id] as const,
};

export function useCatalog() {
  return useQuery({ queryKey: gameKeys.catalog, queryFn: () => get<{ items: GameInfo[] }>('/v1/games').then((r) => r.items), staleTime: Infinity });
}

export function useRooms(game?: GameName, size?: SizeFilter) {
  const qs = new URLSearchParams();
  if (game) qs.set('game', game);
  if (size) qs.set('size', size);
  return useQuery({
    queryKey: gameKeys.rooms(game, size),
    queryFn: () => get<{ items: Room[] }>(`/v1/games/rooms?${qs}`).then((r) => r.items),
    refetchInterval: 10_000,
  });
}

// The room, kept current by "game.updated" pings; polling covers a dropped socket.
export function useRoom<V>(id: string) {
  const qc = useQueryClient();
  const q = useQuery({
    queryKey: gameKeys.room(id),
    queryFn: () => get<Room<V>>(`/v1/games/rooms/${id}`),
    refetchInterval: (query) => (query.state.data?.status === 'playing' ? 4000 : 8000),
  });
  // Pings come several times a second while someone draws; refetch at most every 400ms.
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | null = null;
    const off = on('game.updated', (d: { room_id: string; version: number }) => {
      if (d.room_id !== id) return;
      const cur = qc.getQueryData<Room>(gameKeys.room(id));
      if ((cur && cur.version >= d.version) || timer) return;
      timer = setTimeout(() => {
        timer = null;
        qc.invalidateQueries({ queryKey: gameKeys.room(id) });
      }, 400);
    });
    return () => {
      off();
      if (timer) clearTimeout(timer);
    };
  }, [id, qc]);
  return q;
}

// Live events (a stroke, a placed cell) arrive before the next refetch.
export function useLive<T>(id: string, type: string, fn: (data: T) => void) {
  const ref = useRef(fn);
  useEffect(() => {
    ref.current = fn;
  }, [fn]);
  useEffect(
    () =>
      on('game.live', (d: { room_id: string; type: string; data: T }) => {
        if (d.room_id === id && d.type === type) ref.current(d.data);
      }),
    [id, type],
  );
}

const fail = (e: unknown) => toast(e instanceof ApiError ? e.message : String(e), 'error');

export function useRoomAction(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ action, body }: { action: 'join' | 'leave' | 'ready' | 'start' | 'rematch'; body?: unknown }) =>
      post<Room | undefined>(`/v1/games/rooms/${id}/${action}`, body ?? {}),
    onSuccess: (r) => {
      if (r && typeof r === 'object' && 'id' in r) qc.setQueryData(gameKeys.room(id), r);
      else qc.invalidateQueries({ queryKey: gameKeys.room(id) });
      qc.invalidateQueries({ queryKey: ['games', 'rooms'] });
    },
    onError: fail,
  });
}

// Sends a move; the server answers 204 and the "game.updated" ping refreshes the view.
export function useMove<M>(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (m: M) => post(`/v1/games/rooms/${id}/move`, m),
    onSuccess: () => qc.invalidateQueries({ queryKey: gameKeys.room(id) }),
    onError: fail,
  });
}

// Seconds left until an ISO deadline, ticking once a second.
export function secondsLeft(deadline: string | null | undefined, now: number) {
  if (!deadline) return null;
  return Math.max(0, Math.ceil((new Date(deadline).getTime() - now) / 1000));
}
