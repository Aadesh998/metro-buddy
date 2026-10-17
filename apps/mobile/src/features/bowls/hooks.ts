import { useInfiniteQuery, useMutation, useQuery, useQueryClient, type InfiniteData } from '@tanstack/react-query';

import { del, get, post } from '@/api/client';
import type { Bowl, Comment, FeedPage, Post } from '@/api/types';

export type Sort = 'hot' | 'new' | 'top';

export const bowlKeys = {
  all: ['bowls'] as const,
  suggested: ['bowls', 'suggested'] as const,
  joined: ['bowls', 'joined'] as const,
  search: (q: string) => ['bowls', 'search', q] as const,
  bowl: (id: string) => ['bowls', 'detail', id] as const,
  feed: (scope: string, sort: Sort) => ['feed', scope, sort] as const,
  post: (id: string) => ['post', id] as const,
  comments: (id: string) => ['comments', id] as const,
};

export function useSuggestedBowls() {
  return useQuery({ queryKey: bowlKeys.suggested, queryFn: () => get<{ items: Bowl[] }>('/v1/bowls?suggested=1').then((r) => r.items) });
}

export function useJoinedBowls() {
  return useQuery({ queryKey: bowlKeys.joined, queryFn: () => get<{ items: Bowl[] }>('/v1/bowls/joined').then((r) => r.items) });
}

export function useBowlSearch(q: string) {
  return useQuery({
    queryKey: bowlKeys.search(q),
    queryFn: () => get<{ items: Bowl[] }>(`/v1/bowls?q=${encodeURIComponent(q)}&limit=50`).then((r) => r.items),
    enabled: q.trim().length >= 2,
  });
}

export function useBowl(id: string) {
  return useQuery({ queryKey: bowlKeys.bowl(id), queryFn: () => get<Bowl>(`/v1/bowls/${id}`) });
}

function patchBowlEverywhere(qc: ReturnType<typeof useQueryClient>, id: string, joined: boolean) {
  const apply = (b: Bowl) => (b.id === id ? { ...b, joined, member_count: Math.max(0, b.member_count + (joined ? 1 : -1)) } : b);
  qc.setQueriesData<Bowl[]>({ queryKey: bowlKeys.all }, (old) => (Array.isArray(old) ? old.map(apply) : old));
  qc.setQueryData<Bowl>(bowlKeys.bowl(id), (old) => (old ? apply(old) : old));
}

// Join/leave flip the UI immediately and roll back if the server refuses.
export function useJoin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, join }: { id: string; join: boolean }) => (join ? post<Bowl>(`/v1/bowls/${id}/join`) : del<Bowl>(`/v1/bowls/${id}/join`)),
    onMutate: ({ id, join }) => patchBowlEverywhere(qc, id, join),
    onError: (_e, { id, join }) => patchBowlEverywhere(qc, id, !join),
    onSuccess: (b) => {
      qc.setQueryData(bowlKeys.bowl(b.id), b);
      qc.invalidateQueries({ queryKey: bowlKeys.joined });
      qc.invalidateQueries({ queryKey: ['feed', 'home'] });
    },
  });
}

export function useFeed(scope: 'home' | 'following' | string, sort: Sort) {
  const path = scope === 'home' ? '/v1/feed' : scope === 'following' ? '/v1/feed/following' : `/v1/bowls/${scope}/posts`;
  return useInfiniteQuery({
    queryKey: bowlKeys.feed(scope, sort),
    initialPageParam: '',
    queryFn: ({ pageParam }) => get<FeedPage>(`${path}?sort=${sort}&cursor=${pageParam}`),
    getNextPageParam: (last) => last.next_cursor || undefined,
  });
}

export function usePost(id: string) {
  return useQuery({ queryKey: bowlKeys.post(id), queryFn: () => get<Post>(`/v1/posts/${id}`) });
}

export function useComments(postId: string) {
  return useQuery({
    queryKey: bowlKeys.comments(postId),
    queryFn: () => get<{ items: Comment[] }>(`/v1/posts/${postId}/comments`).then((r) => r.items),
  });
}

type Feed = InfiniteData<FeedPage>;

export function patchPost(qc: ReturnType<typeof useQueryClient>, id: string, fn: (p: Post) => Post) {
  qc.setQueriesData<Feed>({ queryKey: ['feed'] }, (old) =>
    old
      ? {
          ...old,
          pages: old.pages.map((pg) => ({
            ...pg,
            pinned: pg.pinned && pg.pinned.id === id ? fn(pg.pinned) : pg.pinned,
            items: pg.items.map((p) => (p.id === id ? fn(p) : p)),
          })),
        }
      : old,
  );
  qc.setQueryData<Post>(bowlKeys.post(id), (old) => (old ? fn(old) : old));
}

export function useVote() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { target_type: 'post' | 'comment'; target_id: string; value: -1 | 0 | 1; postId: string }) =>
      post<{ score: number; my_vote: -1 | 0 | 1 }>('/v1/votes', { target_type: v.target_type, target_id: v.target_id, value: v.value }),
    onSuccess: (r, v) => {
      if (v.target_type === 'post') patchPost(qc, v.target_id, (p) => ({ ...p, score: r.score, my_vote: r.my_vote }));
      else
        qc.setQueryData<Comment[]>(bowlKeys.comments(v.postId), (old) =>
          old?.map((c) => (c.id === v.target_id ? { ...c, score: r.score, my_vote: r.my_vote } : c)),
        );
    },
  });
}
