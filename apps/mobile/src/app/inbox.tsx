import { FlashList } from '@shopify/flash-list';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import { Bell, Gavel, Heart, MessageCircle, MessageSquareReply, Radio, ShieldAlert, TrainFront } from 'lucide-react-native';
import { Pressable, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { get, post } from '@/api/client';
import type { Notification } from '@/api/types';
import { EmptyState } from '@/components/EmptyState';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Text } from '@/components/ui/Text';
import { chatKeys } from '@/features/chat/hooks';
import { linkToPath } from '@/lib/push';
import { colors } from '@/lib/theme';
import { ago } from '@/lib/time';

const meta: Record<string, { icon: React.ReactNode; text: (p: Record<string, any>) => string }> = {
  dm_request: { icon: <MessageCircle size={18} color={colors.sky} />, text: (p) => (p.accepted ? 'Your message request was accepted' : `${p.from ?? 'Someone'} sent you a message request`) },
  reply: { icon: <MessageSquareReply size={18} color={colors.accent} />, text: () => 'Someone replied to you' },
  match: { icon: <TrainFront size={18} color={colors.mint} />, text: (p) => p.text ?? 'You have a new travel match' },
  trip_reminder: { icon: <TrainFront size={18} color={colors.mint} />, text: (p) => p.text ?? 'Your trip is coming up' },
  episode_live: { icon: <Radio size={18} color={colors.danger} />, text: (p) => `${p.show ?? 'A show you follow'} is live` },
  mod_notice: { icon: <Gavel size={18} color={colors.muted} />, text: (p) => (p.appeal ? `Your appeal was ${p.appeal}` : 'There’s an update about your account') },
  sos: { icon: <ShieldAlert size={18} color={colors.danger} />, text: () => 'SOS alert' },
  message: { icon: <Heart size={18} color={colors.accent} />, text: () => 'New message' },
};

export default function InboxScreen() {
  const qc = useQueryClient();
  const q = useQuery({ queryKey: chatKeys.inbox, queryFn: () => get<{ items: Notification[]; unread: number }>('/v1/notifications') });
  const read = useMutation({
    mutationFn: (body: { ids?: string[]; all?: boolean }) => post('/v1/notifications/read', body),
    onSuccess: () => qc.invalidateQueries({ queryKey: chatKeys.inbox }),
  });
  return (
    <SafeAreaView edges={['top']} className="flex-1 bg-bg">
      <ScreenHeader
        title="Notifications"
        right={
          q.data?.unread ? (
            <Pressable onPress={() => read.mutate({ all: true })} hitSlop={8}>
              <Text className="text-xs font-semibold text-accent">Read all</Text>
            </Pressable>
          ) : null
        }
      />
      <FlashList
        data={q.data?.items ?? []}
        keyExtractor={(n) => n.id}
        renderItem={({ item: n }) => {
          const m = meta[n.kind] ?? { icon: <Bell size={18} color={colors.muted} />, text: () => n.kind };
          return (
            <Pressable
              onPress={() => {
                if (!n.read) read.mutate({ ids: [n.id] });
                const path = n.payload.link ? linkToPath(n.payload.link) : null;
                if (path) router.push(path as never);
              }}
              className={`flex-row items-center gap-3 px-4 py-3.5 ${n.read ? '' : 'bg-surface'}`}
            >
              <View className="h-10 w-10 items-center justify-center rounded-full bg-raised">{m.icon}</View>
              <View className="flex-1">
                <Text className={n.read ? 'text-muted' : 'text-ink'}>{m.text(n.payload)}</Text>
                <Text variant="caption">{ago(n.created_at)}</Text>
              </View>
              {!n.read ? <View className="h-2 w-2 rounded-full bg-accent" /> : null}
            </Pressable>
          );
        }}
        ListEmptyComponent={q.isPending ? null : <EmptyState icon={<Bell size={28} color={colors.faint} />} title="All caught up" />}
      />
    </SafeAreaView>
  );
}
