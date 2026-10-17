import { Image } from 'expo-image';
import { router } from 'expo-router';
import { EyeOff, ImageOff, MessageSquare, MoreHorizontal, Pin } from 'lucide-react-native';
import { memo } from 'react';
import { Pressable, View } from 'react-native';

import type { Post } from '@/api/types';
import { Avatar } from '@/components/Avatar';
import { PersonaName } from '@/components/PersonaBadge';
import { Text } from '@/components/ui/Text';
import { useSafetySheet } from '@/features/safety/SafetySheet';
import { colors } from '@/lib/theme';
import { ago } from '@/lib/time';
import { useVote } from './hooks';
import { PollView } from './PollView';
import { VoteBar } from './VoteBar';

export const PostCard = memo(function PostCard({ p, full, showBowl = true }: { p: Post; full?: boolean; showBowl?: boolean }) {
  const vote = useVote();
  const safety = useSafetySheet();
  const open = () => !full && router.push(`/post/${p.id}`);
  const deleted = p.author.alias === '[deleted]';

  return (
    <Pressable onPress={open} disabled={full} className={`gap-2 bg-surface px-4 py-4 ${full ? '' : 'mb-2 active:bg-raised'}`}>
      {p.is_prompt ? (
        <View className="flex-row items-center gap-1.5">
          <Pin size={13} color={colors.accent} />
          <Text className="text-xs font-semibold text-accent">Today’s prompt</Text>
        </View>
      ) : null}
      <View className="flex-row items-center gap-2">
        <Pressable onPress={() => !deleted && !p.is_mine && router.push(`/u/${p.author.id}`)} hitSlop={6}>
          <Avatar seed={p.author.avatar_seed} size={28} deleted={deleted} />
        </Pressable>
        <View className="flex-1">
          <PersonaName p={p.author} />
          <Text variant="caption">
            {showBowl ? `${p.bowl.name} · ` : ''}
            {ago(p.created_at)}
          </Text>
        </View>
        {!p.is_mine ? (
          <Pressable
            onPress={() => safety.open({ type: 'post', id: p.id, personaId: deleted ? undefined : p.author.id, alias: p.author.alias })}
            hitSlop={10}
            accessibilityLabel="Report, block or mute"
          >
            <MoreHorizontal size={20} color={colors.muted} />
          </Pressable>
        ) : null}
      </View>

      {p.flair ? (
        <View className="self-start rounded-full bg-raised px-2 py-0.5">
          <Text className="text-[11px] font-medium text-muted">{p.flair}</Text>
        </View>
      ) : null}
      <Text className="text-[17px] font-semibold leading-6">{p.title}</Text>
      {p.body ? (
        <Text className="leading-6 text-ink/90" numberOfLines={full ? undefined : 4}>
          {p.body}
        </Text>
      ) : null}

      {p.media.length > 0 ? (
        <View className="flex-row gap-2">
          {p.media.map((m, i) =>
            m.pending || !m.url ? (
              <View key={i} className="h-40 flex-1 items-center justify-center gap-1 rounded-xl bg-raised">
                <ImageOff size={20} color={colors.faint} />
                <Text variant="caption">Image under review</Text>
              </View>
            ) : (
              <Image
                key={i}
                source={{ uri: m.url }}
                style={{ flex: 1, aspectRatio: p.media.length === 1 ? m.w / m.h : 1, borderRadius: 12, maxHeight: 420 }}
                contentFit="cover"
                accessibilityLabel="Post image"
              />
            ),
          )}
        </View>
      ) : null}
      {p.poll ? <PollView p={p} /> : null}
      {p.pii_hidden ? (
        <View className="flex-row items-center gap-1">
          <EyeOff size={12} color={colors.faint} />
          <Text variant="caption">Some contact details were hidden</Text>
        </View>
      ) : null}

      <View className="mt-1 flex-row items-center gap-3">
        <VoteBar score={p.score} mine={p.my_vote} onVote={(v) => vote.mutate({ target_type: 'post', target_id: p.id, value: v, postId: p.id })} />
        <View className="flex-row items-center gap-1.5 rounded-full bg-raised px-3 py-1.5">
          <MessageSquare size={15} color={colors.muted} />
          <Text className="text-sm">{p.comment_count}</Text>
        </View>
      </View>
    </Pressable>
  );
});
