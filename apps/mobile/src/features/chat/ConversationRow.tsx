import { router } from 'expo-router';
import { BellOff, Radio, TrainFront, Users } from 'lucide-react-native';
import { Pressable, View } from 'react-native';

import type { Conversation } from '@/api/types';
import { Avatar } from '@/components/Avatar';
import { Text } from '@/components/ui/Text';
import { useMyPersona } from '@/features/me/hooks';
import { useNow } from '@/lib/clock';
import { colors } from '@/lib/theme';
import { ago } from '@/lib/time';
import { useOnline } from './store';

export function useConversationTitle(c: Conversation) {
  const me = useMyPersona().data?.id;
  const other = c.members.find((m) => m.id !== me);
  if (c.kind === 'dm' || c.kind === 'travel') return { title: other?.alias ?? '[left]', other };
  return { title: c.title ?? 'Group', other: undefined };
}

export function ConversationRow({ c }: { c: Conversation }) {
  const { title, other } = useConversationTitle(c);
  const online = useOnline(other?.id);
  const now = useNow();
  const muted = c.muted_until && Date.parse(c.muted_until) > now;
  const icon =
    c.kind === 'discussion' ? <Radio size={20} color={colors.accent} /> : c.kind === 'travel' || c.kind === 'crew' ? <TrainFront size={20} color={colors.mint} /> : <Users size={20} color={colors.sky} />;
  const preview = c.last_message
    ? c.last_message.deleted
      ? 'Message removed'
      : c.last_message.kind === 'image'
        ? 'Photo'
        : `${c.last_message.is_mine ? 'You: ' : c.kind !== 'dm' && c.last_message.sender ? `${c.last_message.sender.alias}: ` : ''}${c.last_message.body}`
    : 'Say hi';
  return (
    <Pressable onPress={() => router.push(`/chat/${c.id}`)} className="flex-row items-center gap-3 px-4 py-3 active:bg-surface">
      <View>
        {other ? <Avatar seed={other.avatar_seed} size={48} /> : <View className="h-12 w-12 items-center justify-center rounded-full bg-raised">{icon}</View>}
        {online ? <View className="absolute bottom-0 right-0 h-3.5 w-3.5 rounded-full border-2 border-bg bg-mint" /> : null}
      </View>
      <View className="flex-1 gap-0.5">
        <View className="flex-row items-center gap-1.5">
          <Text className="flex-1 font-semibold" numberOfLines={1}>
            {title}
          </Text>
          {muted ? <BellOff size={13} color={colors.faint} /> : null}
          {c.last_message_at ? <Text variant="caption">{ago(c.last_message_at)}</Text> : null}
        </View>
        <View className="flex-row items-center gap-2">
          <Text numberOfLines={1} className={`flex-1 text-sm ${c.unread ? 'text-ink' : 'text-muted'}`}>
            {preview}
          </Text>
          {c.unread > 0 ? (
            <View className={`min-w-5 items-center rounded-full px-1.5 py-0.5 ${muted ? 'bg-raised' : 'bg-accent'}`}>
              <Text className={`text-[11px] font-bold ${muted ? 'text-muted' : 'text-accent-ink'}`}>{c.unread > 99 ? '99+' : c.unread}</Text>
            </View>
          ) : null}
        </View>
      </View>
    </Pressable>
  );
}
