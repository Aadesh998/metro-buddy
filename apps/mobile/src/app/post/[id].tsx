import { FlashList } from "@shopify/flash-list";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { router, useLocalSearchParams } from "expo-router";
import { Lock, MoreHorizontal, Send, Trash2, X } from "lucide-react-native";
import { useState } from "react";
import {
  Alert,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  TextInput,
  View,
} from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";

import { del, post } from "@/api/client";
import type { Comment } from "@/api/types";
import { Avatar } from "@/components/Avatar";
import { EmptyState } from "@/components/EmptyState";
import { PersonaName } from "@/components/PersonaBadge";
import { ScreenHeader } from "@/components/ScreenHeader";
import { SupportSheet } from "@/components/SupportSheet";
import { Text } from "@/components/ui/Text";
import {
  bowlKeys,
  patchPost,
  useComments,
  usePost,
  useVote,
} from "@/features/bowls/hooks";
import { PostCard } from "@/features/bowls/PostCard";
import { VoteBar } from "@/features/bowls/VoteBar";
import { useSafetySheet } from "@/features/safety/SafetySheet";
import { useT } from "@/i18n";
import { colors } from "@/lib/theme";
import { ago } from "@/lib/time";

function CommentRow({
  c,
  postId,
  onReply,
}: {
  c: Comment;
  postId: string;
  onReply: (c: Comment) => void;
}) {
  const { t } = useT();
  const vote = useVote();
  const safety = useSafetySheet();
  const qc = useQueryClient();
  const remove = () =>
    Alert.alert(t("bowls.post.deleteCommentTitle"), undefined, [
      { text: t("common.cancel"), style: "cancel" },
      {
        text: t("common.delete"),
        style: "destructive",
        onPress: () =>
          del(`/v1/comments/${c.id}`).then(() =>
            qc.invalidateQueries({ queryKey: bowlKeys.comments(postId) }),
          ),
      },
    ]);
  return (
    <View style={{ paddingLeft: 16 + c.depth * 18 }} className="pr-4">
      <View
        className={`gap-1.5 py-3 ${c.depth > 0 ? "border-l-2 border-line pl-3" : ""}`}
      >
        {c.deleted ? (
          <Text variant="caption">{t("bowls.post.removed")}</Text>
        ) : (
          <>
            <View className="flex-row items-center gap-2">
              <Avatar seed={c.author.avatar_seed} size={22} />
              <PersonaName p={c.author} />
              {c.is_op ? (
                <View className="rounded bg-accent/20 px-1.5">
                  <Text className="text-[10px] font-bold text-accent">
                    {t("bowls.post.op")}
                  </Text>
                </View>
              ) : null}
              <Text variant="caption">{ago(c.created_at)}</Text>
              <View className="flex-1" />
              {c.is_mine ? (
                <Pressable
                  onPress={remove}
                  hitSlop={10}
                  accessibilityLabel={t("bowls.post.deleteComment")}
                >
                  <Trash2 size={15} color={colors.faint} />
                </Pressable>
              ) : (
                <Pressable
                  onPress={() =>
                    safety.open({
                      type: "comment",
                      id: c.id,
                      personaId: c.author.id,
                      alias: c.author.alias,
                    })
                  }
                  hitSlop={10}
                  accessibilityLabel={t("common.safetyOptions")}
                >
                  <MoreHorizontal size={16} color={colors.faint} />
                </Pressable>
              )}
            </View>
            <Text className="leading-6">{c.body}</Text>
            <View className="flex-row items-center gap-4">
              <VoteBar
                score={c.score}
                mine={c.my_vote}
                onVote={(v) =>
                  vote.mutate({
                    target_type: "comment",
                    target_id: c.id,
                    value: v,
                    postId,
                  })
                }
              />
              <Pressable onPress={() => onReply(c)} hitSlop={8}>
                <Text className="text-sm font-semibold text-muted">
                  {t("bowls.post.reply")}
                </Text>
              </Pressable>
            </View>
          </>
        )}
      </View>
    </View>
  );
}

export default function PostScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { t } = useT();
  const qc = useQueryClient();
  const p = usePost(id);
  const comments = useComments(id);
  const [text, setText] = useState("");
  const [replyTo, setReplyTo] = useState<Comment | null>(null);
  const [support, setSupport] = useState(false);

  const send = useMutation({
    mutationFn: () =>
      post<Comment>(`/v1/posts/${id}/comments`, {
        body: text.trim(),
        parent_id: replyTo?.id,
      }),
    onSuccess: (c) => {
      setText("");
      setReplyTo(null);
      if (c.show_support) setSupport(true);
      qc.invalidateQueries({ queryKey: bowlKeys.comments(id) });
      patchPost(qc, id, (x) => ({ ...x, comment_count: x.comment_count + 1 }));
    },
  });

  if (p.isError) {
    return (
      <SafeAreaView className="flex-1 bg-bg">
        <ScreenHeader />
        <EmptyState
          title={t("bowls.post.unavailableTitle")}
          body={t("bowls.post.unavailableBody")}
        />
      </SafeAreaView>
    );
  }

  return (
    <SafeAreaView edges={["top", "bottom"]} className="flex-1 bg-bg">
      <ScreenHeader
        title={p.data?.bowl.name}
        right={
          p.data?.is_mine ? (
            <Pressable
              hitSlop={10}
              accessibilityLabel={t("bowls.post.deletePost")}
              onPress={() =>
                Alert.alert(
                  t("bowls.post.deletePostTitle"),
                  t("bowls.post.deletePostBody"),
                  [
                    { text: t("common.cancel"), style: "cancel" },
                    {
                      text: t("common.delete"),
                      style: "destructive",
                      onPress: () =>
                        del(`/v1/posts/${id}`).then(() => {
                          qc.invalidateQueries({ queryKey: ["feed"] });
                          router.back();
                        }),
                    },
                  ],
                )
              }
            >
              <Trash2 size={18} color={colors.muted} />
            </Pressable>
          ) : null
        }
      />
      <KeyboardAvoidingView
        className="flex-1"
        behavior={Platform.OS === "ios" ? "padding" : undefined}
      >
        <FlashList
          data={comments.data ?? []}
          keyExtractor={(c) => c.id}
          renderItem={({ item }) => (
            <CommentRow c={item} postId={id} onReply={setReplyTo} />
          )}
          ListHeaderComponent={
            p.data ? (
              <View className="mb-2 border-b border-line">
                <PostCard p={p.data} full />
              </View>
            ) : null
          }
          ListEmptyComponent={
            comments.isPending ? null : (
              <Text className="px-4 py-8 text-center text-muted">
                {t("bowls.post.noComments")}
              </Text>
            )
          }
        />
        {p.data?.locked ? (
          <View className="flex-row items-center justify-center gap-2 border-t border-line bg-surface px-3 py-3">
            <Lock size={14} color={colors.muted} />
            <Text variant="caption">{t("bowls.post.lockedNote")}</Text>
          </View>
        ) : (
          <View className="border-t border-line bg-surface px-3 pb-2 pt-2">
            {replyTo ? (
              <View className="mb-2 flex-row items-center justify-between px-1">
                <Text variant="caption">
                  {t("bowls.post.replyingTo", { alias: replyTo.author.alias })}
                </Text>
                <Pressable onPress={() => setReplyTo(null)} hitSlop={8}>
                  <X size={14} color={colors.muted} />
                </Pressable>
              </View>
            ) : null}
            <View className="flex-row items-end gap-2">
              <TextInput
                value={text}
                onChangeText={(v) => setText(v.slice(0, 2000))}
                placeholder={t("bowls.post.commentPlaceholder")}
                placeholderTextColor={colors.faint}
                multiline
                className="max-h-32 flex-1 rounded-2xl bg-raised px-4 py-2.5 text-base text-ink"
              />
              <Pressable
                disabled={!text.trim() || send.isPending}
                onPress={() => send.mutate()}
                className={`h-10 w-10 items-center justify-center rounded-full ${text.trim() ? "bg-accent" : "bg-raised"}`}
                accessibilityLabel={t("bowls.post.sendComment")}
              >
                <Send
                  size={17}
                  color={text.trim() ? colors.accentInk : colors.faint}
                />
              </Pressable>
            </View>
          </View>
        )}
      </KeyboardAvoidingView>
      <SupportSheet visible={support} onClose={() => setSupport(false)} />
    </SafeAreaView>
  );
}
