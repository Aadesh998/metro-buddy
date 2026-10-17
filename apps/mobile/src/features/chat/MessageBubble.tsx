import { Image } from 'expo-image';
import { AlertCircle, Clock, EyeOff, MapPin, Sparkles, Timer } from 'lucide-react-native';
import { memo } from 'react';
import { Pressable, View } from 'react-native';

import type { Message } from '@/api/types';
import { Avatar } from '@/components/Avatar';
import { Text } from '@/components/ui/Text';
import { colors } from '@/lib/theme';

function time(iso: string) {
  return new Date(iso).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
}

export const MessageBubble = memo(function MessageBubble({
  m,
  showSender,
  seen,
  onLongPress,
  onRetry,
  onPickOpener,
}: {
  m: Message;
  showSender: boolean;
  seen?: boolean;
  onLongPress: (m: Message) => void;
  onRetry: (m: Message) => void;
  onPickOpener?: (text: string) => void;
}) {
  if (m.kind === 'icebreaker') {
    return (
      <View className="mx-4 my-2 gap-2 rounded-2xl border border-accent/30 bg-accent/5 p-4">
        <View className="flex-row items-center gap-2">
          <Sparkles size={16} color={colors.accent} />
          <Text variant="label">Need an opener? Tap one to use it</Text>
        </View>
        {m.body.split('\n').map((line) => (
          <Pressable key={line} onPress={() => onPickOpener?.(line)} className="rounded-xl bg-raised px-3 py-2.5 active:opacity-70" accessibilityRole="button">
            <Text className="text-sm">{line}</Text>
          </Pressable>
        ))}
      </View>
    );
  }
  if (m.kind === 'system') {
    return (
      <View className="my-2 items-center px-8">
        <Text variant="caption" className="text-center">
          {m.body}
        </Text>
      </View>
    );
  }
  if (m.kind === 'meet_point') {
    return (
      <View className="mx-4 my-2 flex-row items-center gap-3 rounded-2xl border border-mint/40 bg-mint/10 p-4">
        <MapPin size={20} color={colors.mint} />
        <Text className="flex-1 leading-5">{m.body}</Text>
      </View>
    );
  }
  const mine = m.is_mine;
  return (
    <View className={`my-0.5 flex-row px-3 ${mine ? 'justify-end' : 'justify-start'}`}>
      {!mine ? <View className="mr-2 w-7">{showSender && m.sender ? <Avatar seed={m.sender.avatar_seed} size={28} /> : null}</View> : null}
      <Pressable onLongPress={() => onLongPress(m)} onPress={() => m.local === 'failed' && onRetry(m)} delayLongPress={300} className="max-w-[78%]">
        {showSender && !mine && m.sender ? <Text className="mb-0.5 ml-1 text-xs font-semibold text-muted">{m.sender.alias}</Text> : null}
        <View className={`rounded-2xl px-3.5 py-2 ${mine ? 'rounded-br-md bg-accent' : 'rounded-bl-md bg-raised'} ${m.local ? 'opacity-70' : ''}`}>
          {m.deleted ? (
            <Text className={`italic ${mine ? 'text-accent-ink/70' : 'text-muted'}`}>Message removed</Text>
          ) : (
            <>
              {m.media.map((im, i) =>
                im.url && !im.pending ? (
                  <Image key={i} source={{ uri: im.url }} style={{ width: 220, aspectRatio: im.w / im.h, borderRadius: 12, marginBottom: 4 }} />
                ) : (
                  <Text key={i} variant="caption">
                    Image under review
                  </Text>
                ),
              )}
              {m.body ? <Text className={`text-[15px] leading-5 ${mine ? 'text-accent-ink' : 'text-ink'}`}>{m.body}</Text> : null}
            </>
          )}
          <View className="mt-0.5 flex-row items-center justify-end gap-1">
            {m.pii_hidden ? <EyeOff size={10} color={mine ? colors.accentInk : colors.faint} /> : null}
            {m.local === 'sending' ? <Clock size={10} color={colors.accentInk} /> : null}
            {m.expires_at ? <Timer size={10} color={mine ? colors.accentInk : colors.faint} accessibilityLabel="Disappearing message" /> : null}
            <Text className={`text-[10px] ${mine ? 'text-accent-ink/60' : 'text-faint'}`}>{time(m.created_at)}</Text>
          </View>
        </View>
        {m.local === 'failed' ? (
          <View className="mt-1 flex-row items-center justify-end gap-1">
            <AlertCircle size={12} color={colors.danger} />
            <Text className="text-xs text-danger">Not sent. Tap to retry.</Text>
          </View>
        ) : null}
        {seen ? <Text className="mr-1 mt-0.5 text-right text-[11px] text-muted">Seen</Text> : null}
      </Pressable>
    </View>
  );
});
