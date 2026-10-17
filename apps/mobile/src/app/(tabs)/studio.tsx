import { router } from 'expo-router';
import { Mic, Plus, Radio } from 'lucide-react-native';
import { useState } from 'react';
import { Pressable, RefreshControl, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { Show } from '@/api/types';
import { EmptyState } from '@/components/EmptyState';
import { TabHeader } from '@/components/TabHeader';
import { Button } from '@/components/ui/Button';
import { Chip } from '@/components/ui/Chip';
import { Text } from '@/components/ui/Text';
import { useMyPersona } from '@/features/me/hooks';
import { EpisodeCard } from '@/features/studio/EpisodeCard';
import { useHostApplication, useSchedule, useShows } from '@/features/studio/hooks';
import { useNow } from '@/lib/clock';
import { colors } from '@/lib/theme';

function ShowRow({ s }: { s: Show }) {
  return (
    <Pressable onPress={() => router.push(`/show/${s.id}`)} className="gap-1 rounded-2xl border border-line bg-surface p-4">
      <View className="flex-row items-center justify-between">
        <Text variant="label" numberOfLines={1} className="flex-1">
          {s.title}
        </Text>
        <Text variant="caption">{s.follower_count} following</Text>
      </View>
      <Text variant="caption">by {s.host.alias}</Text>
      {s.description ? (
        <Text className="text-sm text-muted" numberOfLines={2}>
          {s.description}
        </Text>
      ) : null}
    </Pressable>
  );
}

function HostCard() {
  const app = useHostApplication();
  const persona = useMyPersona().data;
  if (persona?.is_host) {
    return (
      <View className="flex-row gap-3">
        <Button title="My shows" kind="secondary" className="flex-1" onPress={() => router.push('/studio/mine')} />
        <Button title="New show" className="flex-1" onPress={() => router.push('/studio/new-show')} />
      </View>
    );
  }
  const a = app.data;
  if (a?.status === 'applied') {
    return (
      <View className="rounded-2xl border border-line bg-surface p-4">
        <Text variant="label">Your host application is in review</Text>
        <Text variant="caption">We’ll let you know once our team has looked at it.</Text>
      </View>
    );
  }
  if (a?.status === 'suspended') return null;
  return (
    <Pressable onPress={() => router.push('/studio/apply')} className="flex-row items-center gap-3 rounded-2xl border border-dashed border-line p-4">
      <Mic size={20} color={colors.accent} />
      <View className="flex-1">
        <Text variant="label">Want to host a show?</Text>
        <Text variant="caption">{a?.status === 'rejected' ? 'You can apply again.' : 'Apply to run your own anonymous audio show.'}</Text>
      </View>
      <Plus size={18} color={colors.muted} />
    </Pressable>
  );
}

export default function StudioTab() {
  const schedule = useSchedule();
  const [filter, setFilter] = useState<'' | 'following'>('');
  const shows = useShows(filter);
  const now = useNow(60_000);
  const eps = schedule.data ?? [];
  const live = eps.filter((e) => e.status === 'live');
  const soon = eps.filter((e) => e.status === 'scheduled');

  return (
    <SafeAreaView edges={['top']} className="flex-1 bg-bg">
      <TabHeader title="Vent Studio" />
      <ScrollView
        contentContainerClassName="gap-6 px-4 pb-10"
        refreshControl={
          <RefreshControl
            tintColor={colors.accent}
            refreshing={schedule.isRefetching}
            onRefresh={() => {
              schedule.refetch();
              shows.refetch();
            }}
          />
        }
      >
        <HostCard />
        <View className="gap-3">
          <Text variant="heading">Live now</Text>
          {live.length === 0 ? <Text className="text-muted">No shows on air right now.</Text> : live.map((e) => <EpisodeCard key={e.id} ep={e} now={now} />)}
        </View>
        {soon.length > 0 ? (
          <View className="gap-3">
            <Text variant="heading">Coming up</Text>
            {soon.map((e) => (
              <EpisodeCard key={e.id} ep={e} now={now} />
            ))}
          </View>
        ) : null}
        <View className="gap-3">
          <View className="flex-row items-center justify-between">
            <Text variant="heading">Shows</Text>
            <View className="flex-row gap-2">
              <Chip label="All" selected={filter === ''} onPress={() => setFilter('')} />
              <Chip label="Following" selected={filter === 'following'} onPress={() => setFilter('following')} />
            </View>
          </View>
          {(shows.data ?? []).map((s) => (
            <ShowRow key={s.id} s={s} />
          ))}
          {shows.data?.length === 0 ? (
            <EmptyState
              icon={<Radio color={colors.accent} size={32} />}
              title={filter ? 'Not following any shows' : 'No shows yet'}
              body={filter ? 'Follow a show to get a nudge 10 minutes before it goes live.' : 'Call in to a live audio show and let it out, anonymously.'}
            />
          ) : null}
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}
