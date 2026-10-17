import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { router } from "expo-router";

import { ApiError, del, get, post } from "@/api/client";
import type { Conversation, PersonaRef } from "@/api/types";

export type ConnState = "none" | "outgoing" | "incoming" | "connected";

export type Relationship = {
  persona_id: string;
  following: boolean;
  follows_you: boolean;
  connection: ConnState;
  connection_id: string | null;
  followers: number;
  following_count: number;
  connections: number;
};

export type Connection = {
  id: string;
  persona: PersonaRef;
  state: ConnState;
  note: string;
  created_at: string;
  accepted_at: string | null;
};

export type ConnBox = "incoming" | "accepted" | "outgoing";

export const socialKeys = {
  all: ["social"] as const,
  rel: (id: string) => ["social", "rel", id] as const,
  connections: (box: ConnBox) => ["social", "connections", box] as const,
  follows: (kind: "followers" | "following") => ["social", kind] as const,
};

export function useRelationship(id: string | undefined) {
  return useQuery({
    queryKey: socialKeys.rel(id ?? ""),
    queryFn: () => get<Relationship>(`/v1/personas/${id}/relationship`),
    enabled: !!id,
  });
}

export function useConnections(box: ConnBox) {
  return useQuery({
    queryKey: socialKeys.connections(box),
    queryFn: () =>
      get<{ items: Connection[] }>(`/v1/connections?box=${box}`).then(
        (r) => r.items,
      ),
  });
}

export function useFollows(kind: "followers" | "following") {
  return useQuery({
    queryKey: socialKeys.follows(kind),
    queryFn: () =>
      get<{ items: PersonaRef[] }>(`/v1/me/${kind}`).then((r) => r.items),
  });
}

// Every social action returns or changes a relationship; refresh all social queries after it.
function useSocialMutation<V>(fn: (v: V) => Promise<unknown>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: (data, _v) => {
      const r = data as Relationship | undefined;
      if (r && typeof r === "object" && "persona_id" in r)
        qc.setQueryData(socialKeys.rel(r.persona_id), r);
      qc.invalidateQueries({ queryKey: socialKeys.all });
    },
  });
}

export function useFollow() {
  return useSocialMutation(({ id, on }: { id: string; on: boolean }) =>
    on
      ? post<Relationship>(`/v1/personas/${id}/follow`)
      : del(`/v1/personas/${id}/follow`),
  );
}

export function useConnect() {
  return useSocialMutation(({ id, note }: { id: string; note?: string }) =>
    post<Relationship>("/v1/connections", {
      to_persona_id: id,
      note: note ?? "",
    }),
  );
}

export function useRespond() {
  return useSocialMutation(
    ({ connectionId, accept }: { connectionId: string; accept: boolean }) =>
      post<Relationship>(
        `/v1/connections/${connectionId}/${accept ? "accept" : "decline"}`,
      ),
  );
}

export function useRemoveConnection() {
  return useSocialMutation((connectionId: string) =>
    del(`/v1/connections/${connectionId}`),
  );
}

export type ChatTarget = {
  id: string;
  alias: string;
  avatar_seed: string;
  context?: string;
  context_id?: string;
};

// Opens the chat with a connection, or the message request screen for anyone else.
export function useMessage() {
  return useMutation({
    mutationFn: (p: ChatTarget) =>
      post<Conversation>("/v1/dm/open", { persona_id: p.id }),
    onSuccess: (conv) => router.push(`/chat/${conv.id}`),
    onError: (e, p) => {
      if (e instanceof ApiError && e.code === "not_connected") {
        router.push({
          pathname: "/dm/new",
          params: {
            persona: p.id,
            alias: p.alias,
            seed: p.avatar_seed,
            context: p.context ?? "profile",
            context_id: p.context_id,
          },
        });
      }
    },
  });
}
