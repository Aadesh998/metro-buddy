import { router } from 'expo-router';
import { Plus, Radio, Users } from 'lucide-react-native';
import { Pressable, ScrollView, View } from 'react-native';

import type { Room } from '@/api/types';
import { EmptyState } from '@/components/EmptyState';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { useRooms } from '@/features/chat/hooks';
import { colors } from '@/lib/theme';
import { until } from '@/lib/time';

export function RoomCard({ r }: { r: Room }) {
  const start = new Date(r.starts_at);
  return (
    <Pressable onPress={() => router.push(`/chat/${r.id}`)} className="gap-2 rounded-2xl border border-line bg-surface p-4 active:bg-raised">
      <View className="flex-row items-center gap-2">
        {r.live ? (
          <View className="flex-row items-center gap-1 rounded-full bg-danger/20 px-2 py-0.5">
            <View className="h-1.5 w-1.5 rounded-full bg-danger" />
            <Text className="text-[11px] font-bold text-danger">LIVE</Text>
          </View>
        ) : (
          <Text variant="caption">
            {start.toLocaleDateString([], { weekday: 'short' })} {start.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })}
          </Text>
        )}
        <View className="flex-1" />
        <Users size={13} color={colors.muted} />
        <Text variant="caption">{r.member_count}</Text>
      </View>
      <Text variant="heading">{r.title}</Text>
      {r.topic ? <Text className="text-muted">{r.topic}</Text> : null}
      <Text variant="caption">
        {r.live ? until(r.ends_at) : `${Math.round((Date.parse(r.ends_at) - Date.parse(r.starts_at)) / 60000)} min`}
        {r.slow_mode_sec ? ` · slow mode ${r.slow_mode_sec}s` : ''}
        {r.joined ? ' · joined' : ''}
      </Text>
    </Pressable>
  );
}

export default function Rooms() {
  const live = useRooms('live');
  const upcoming = useRooms('upcoming');
  const later = (upcoming.data ?? []).filter((r) => !r.live);
  return (
    <Screen edges={['top']} className="px-0">
      <ScreenHeader
        title="Discussion rooms"
        right={
          <Pressable onPress={() => router.push('/rooms/new')} hitSlop={10} accessibilityLabel="Schedule a room">
            <Plus size={22} color={colors.accent} />
          </Pressable>
        }
      />
      <ScrollView contentContainerClassName="gap-3 px-4 pb-10">
        <Text variant="caption" className="mt-2 uppercase tracking-wider">
          Live now
        </Text>
        {(live.data ?? []).length === 0 ? <Text className="text-muted">Nothing live right now.</Text> : live.data!.map((r) => <RoomCard key={r.id} r={r} />)}
        <Text variant="caption" className="mt-4 uppercase tracking-wider">
          Coming up
        </Text>
        {later.length === 0 ? (
          <EmptyState icon={<Radio color={colors.accent} size={28} />} title="No rooms scheduled" body="Rooms are time-boxed group chats around a topic, like “Monday Office Rant”." />
        ) : (
          later.map((r) => <RoomCard key={r.id} r={r} />)
        )}
      </ScrollView>
    </Screen>
  );
}
