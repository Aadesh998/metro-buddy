import { router } from 'expo-router';
import { Plus, Radio, Users } from 'lucide-react-native';
import { Pressable, ScrollView, View } from 'react-native';

import type { Room } from '@/api/types';
import { EmptyState } from '@/components/EmptyState';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { useRooms } from '@/features/chat/hooks';
import { dateLocale, useT } from '@/i18n';
import { colors } from '@/lib/theme';
import { until } from '@/lib/time';

export function RoomCard({ r }: { r: Room }) {
  const { t } = useT();
  const start = new Date(r.starts_at);
  return (
    <Pressable onPress={() => router.push(`/chat/${r.id}`)} className="gap-2 rounded-2xl border border-line bg-surface p-4 active:bg-raised">
      <View className="flex-row items-center gap-2">
        {r.live ? (
          <View className="flex-row items-center gap-1 rounded-full bg-danger/20 px-2 py-0.5">
            <View className="h-1.5 w-1.5 rounded-full bg-danger" />
            <Text className="text-[11px] font-bold text-danger">{t('chat.rooms.live')}</Text>
          </View>
        ) : (
          <Text variant="caption">
            {start.toLocaleDateString(dateLocale(), { weekday: 'short' })} {start.toLocaleTimeString(dateLocale(), { hour: 'numeric', minute: '2-digit' })}
          </Text>
        )}
        <View className="flex-1" />
        <Users size={13} color={colors.muted} />
        <Text variant="caption">{r.member_count}</Text>
      </View>
      <Text variant="heading">{r.title}</Text>
      {r.topic ? <Text className="text-muted">{r.topic}</Text> : null}
      <Text variant="caption">
        {r.live ? until(r.ends_at) : t('chat.rooms.mins', { n: Math.round((Date.parse(r.ends_at) - Date.parse(r.starts_at)) / 60000) })}
        {r.slow_mode_sec ? t('chat.rooms.slowMode', { n: r.slow_mode_sec }) : ''}
        {r.joined ? t('chat.rooms.joined') : ''}
      </Text>
    </Pressable>
  );
}

export default function Rooms() {
  const { t } = useT();
  const live = useRooms('live');
  const upcoming = useRooms('upcoming');
  const later = (upcoming.data ?? []).filter((r) => !r.live);
  return (
    <Screen edges={['top']} className="px-0">
      <ScreenHeader
        title={t('chat.rooms.title')}
        right={
          <Pressable onPress={() => router.push('/rooms/new')} hitSlop={10} accessibilityLabel={t('chat.rooms.schedule')}>
            <Plus size={22} color={colors.brand} />
          </Pressable>
        }
      />
      <ScrollView contentContainerClassName="gap-3 px-4 pb-10">
        <Text variant="eyebrow" className="mt-2">
          {t('chat.rooms.liveNow')}
        </Text>
        {(live.data ?? []).length === 0 ? <Text className="text-muted">{t('chat.rooms.nothingLive')}</Text> : live.data!.map((r) => <RoomCard key={r.id} r={r} />)}
        <Text variant="eyebrow" className="mt-4">
          {t('chat.rooms.comingUp')}
        </Text>
        {later.length === 0 ? (
          <EmptyState icon={<Radio color={colors.brand} size={28} />} title={t('chat.rooms.emptyTitle')} body={t('chat.rooms.emptyBody')} />
        ) : (
          later.map((r) => <RoomCard key={r.id} r={r} />)
        )}
      </ScrollView>
    </Screen>
  );
}
