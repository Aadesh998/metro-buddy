import { useInfiniteQuery, useMutation, useQuery, useQueryClient, type InfiniteData, type QueryClient } from '@tanstack/react-query';

import { ApiError, get, post } from '@/api/client';
import type { Conversation, DMRequest, Message, Room } from '@/api/types';
import { toast } from '@/components/ui/Toast';
import { uuidv7 } from '@/lib/uuid';
import { enqueue, flush, pendingFor } from './outbox';
import { supportStore } from './store';

export const chatKeys = {
  conversations: ['conversations'] as const,
  conversation: (id: string) => ['conversation', id] as const,
  messages: (id: string) => ['messages', id] as const,
  requests: (box: 'incoming' | 'outgoing') => ['requests', box] as const,
  rooms: (scope: string) => ['rooms', scope] as const,
  inbox: ['inbox'] as const,
};

type MsgPage = { items: Message[]; next_cursor: string };
type Msgs = InfiniteData<MsgPage>;

export function useConversations() {
  return useQuery({
    queryKey: chatKeys.conversations,
    queryFn: () => get<{ items: Conversation[] }>('/v1/conversations?limit=100').then((r) => r.items),
  });
}

export function useConversation(id: string) {
  return useQuery({ queryKey: chatKeys.conversation(id), queryFn: () => get<Conversation>(`/v1/conversations/${id}`) });
}

export function useMessages(id: string) {
  return useInfiniteQuery({
    queryKey: chatKeys.messages(id),
    initialPageParam: '',
    queryFn: async ({ pageParam }) => {
      const page = await get<MsgPage>(`/v1/conversations/${id}/messages?limit=40&cursor=${pageParam}`);
      if (pageParam) return page;
      // Messages still waiting in the offline outbox stay visible at the bottom of the chat.
      const pending = await pendingFor(id);
      const ids = new Set(page.items.map((m) => m.id));
      const local = pending
        .filter((p) => !ids.has(p.id))
        .map((p) => localMessage(p.conversation_id, p.id, p.body, p.created_at, 'sending'));
      return { ...page, items: [...local.reverse(), ...page.items] };
    },
    getNextPageParam: (last) => last.next_cursor || undefined,
  });
}

function localMessage(conv: string, id: string, body: string, created: string, local: Message['local']): Message {
  return {
    id, conversation_id: conv, sender: null, is_mine: true, kind: 'text', body, media: [], reply_to_id: null,
    deleted: false, pii_hidden: false, created_at: created, expires_at: null, local,
  };
}

// upsertMessage inserts or replaces a message in the conversation's cache (newest first).
export function upsertMessage(qc: QueryClient, m: Message) {
  qc.setQueryData<Msgs>(chatKeys.messages(m.conversation_id), (old) => {
    if (!old) return old;
    let found = false;
    const pages = old.pages.map((pg) => ({
      ...pg,
      items: pg.items.map((x) => {
        if (x.id !== m.id) return x;
        found = true;
        return { ...x, ...m, local: m.local };
      }),
    }));
    if (!found && pages.length) pages[0] = { ...pages[0], items: [m, ...pages[0].items] };
    return { ...old, pages };
  });
}

export function patchMessage(qc: QueryClient, convId: string, id: string, fn: (m: Message) => Message) {
  qc.setQueryData<Msgs>(chatKeys.messages(convId), (old) =>
    old ? { ...old, pages: old.pages.map((pg) => ({ ...pg, items: pg.items.map((x) => (x.id === id ? fn(x) : x)) })) } : old,
  );
}

export function bumpConversation(qc: QueryClient, m: Message, unreadDelta: number) {
  let present = false;
  qc.setQueryData<Conversation[]>(chatKeys.conversations, (old) => {
    if (!old) return old;
    const next = old.map((c) => {
      if (c.id !== m.conversation_id) return c;
      present = true;
      return { ...c, last_message: m, last_message_at: m.created_at, unread: Math.max(0, c.unread + unreadDelta) };
    });
    return next.sort((a, b) => Date.parse(b.last_message_at ?? '0') - Date.parse(a.last_message_at ?? '0'));
  });
  if (!present) qc.invalidateQueries({ queryKey: chatKeys.conversations });
}

export function flushOutbox(qc: QueryClient) {
  return flush(
    (_p, m) => {
      upsertMessage(qc, { ...m, local: undefined });
      bumpConversation(qc, m, 0);
      if (m.show_support) supportStore.set((n) => n + 1);
    },
    (p, e) => {
      patchMessage(qc, p.conversation_id, p.id, (x) => ({ ...x, local: 'failed' }));
      toast(e.message);
    },
  );
}

// Optimistic send: the message shows instantly and survives being offline or the app closing.
export function useSend(conversationId: string) {
  const qc = useQueryClient();
  return async (body: string) => {
    const id = uuidv7();
    const created = new Date().toISOString();
    upsertMessage(qc, localMessage(conversationId, id, body, created, 'sending'));
    await enqueue({ conversation_id: conversationId, id, body, created_at: created });
    await flushOutbox(qc);
  };
}

export function useRequests(box: 'incoming' | 'outgoing') {
  return useQuery({ queryKey: chatKeys.requests(box), queryFn: () => get<{ items: DMRequest[] }>(`/v1/dm-requests?box=${box}`).then((r) => r.items) });
}

export function useRespond() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, action }: { id: string; action: 'accept' | 'ignore' }) => post<Conversation | undefined>(`/v1/dm-requests/${id}/${action}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['requests'] });
      qc.invalidateQueries({ queryKey: chatKeys.conversations });
    },
  });
}

export function useRooms(scope: 'live' | 'upcoming', bowlId?: string) {
  const path = scope === 'live' ? '/v1/rooms/live' : '/v1/rooms';
  return useQuery({
    queryKey: chatKeys.rooms(`${scope}:${bowlId ?? ''}`),
    queryFn: () => get<{ items: Room[] }>(`${path}${bowlId ? `?bowl_id=${bowlId}` : ''}`).then((r) => r.items),
    refetchInterval: 30_000,
  });
}

export function isApiError(e: unknown, code: string) {
  return e instanceof ApiError && e.code === code;
}
