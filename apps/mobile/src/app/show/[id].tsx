import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router, useLocalSearchParams } from 'expo-router';
import { Minus, MoreHorizontal, Plus } from 'lucide-react-native';
import { useState } from 'react';
import { Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { post } from '@/api/client';
import type { Episode, Show } from '@/api/types';
import { PersonaRow } from '@/components/PersonaBadge';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Chip } from '@/components/ui/Chip';
import { Input } from '@/components/ui/Input';
import { Text } from '@/components/ui/Text';
import { useSafetySheet } from '@/features/safety/SafetySheet';
import { EpisodeCard } from '@/features/studio/EpisodeCard';
import { studioKeys, useFollow, useShow } from '@/features/studio/hooks';
import { useNow } from '@/lib/clock';
import { colors } from '@/lib/theme';

const DAY = 86_400_000;

function Scheduler({ show, now }: { show: Show; now: number }) {
  const qc = useQueryClient();
  const [title, setTitle] = useState('');
  const [day, setDay] = useState(0);
  const [minutes, setMinutes] = useState(() => {
    const d = new Date(now);
    return Math.floor((d.getHours() * 60 + d.getMinutes()) / 15) * 15 + 15;
  });
  const base = new Date(now + day * DAY);
  base.setHours(0, 0, 0, 0);
  const at = new Date(base.getTime() + minutes * 60_000);
  const past = at.getTime() < now;
  const create = useMutation({
    mutationFn: () => post<Episode>(`/v1/shows/${show.id}/episodes`, { title: title.trim(), scheduled_at: at.toISOString() }),
    onSuccess: () => {
      setTitle('');
      qc.invalidateQueries({ queryKey: studioKeys.show(show.id) });
      qc.invalidateQueries({ queryKey: studioKeys.schedule });
    },
  });
  const step = (d: number) => setMinutes((m) => Math.min(24 * 60 - 15, Math.max(0, m + d)));
  return (
    <View className="gap-3 rounded-2xl border border-line bg-surface p-4">
      <Text variant="heading">Schedule an episode</Text>
      <Input value={title} onChangeText={(t) => setTitle(t.slice(0, 80))} placeholder="Tonight’s topic" />
      <View className="flex-row flex-wrap gap-2">
        {[0, 1, 2, 3, 4, 5, 6].map((d) => (
          <Chip
            key={d}
            label={d === 0 ? 'Today' : d === 1 ? 'Tomorrow' : new Date(now + d * DAY).toLocaleDateString([], { weekday: 'short' })}
            selected={day === d}
            onPress={() => setDay(d)}
          />
        ))}
      </View>
      <View className="flex-row items-center justify-center gap-6">
        <Pressable onPress={() => step(-15)} hitSlop={10} accessibilityLabel="15 minutes earlier" className="h-10 w-10 items-center justify-center rounded-full bg-raised">
          <Minus size={18} color={colors.ink} />
        </Pressable>
        <Text className="w-28 text-center font-mono text-2xl font-bold">{at.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })}</Text>
        <Pressable onPress={() => step(15)} hitSlop={10} accessibilityLabel="15 minutes later" className="h-10 w-10 items-center justify-center rounded-full bg-raised">
          <Plus size={18} color={colors.ink} />
        </Pressable>
      </View>
      {past ? <Text className="text-center text-xs text-danger">That time has already passed.</Text> : null}
      <Text variant="caption">Followers get a nudge 10 minutes before. You can go live up to 15 minutes early.</Text>
      <Button title="Schedule" disabled={title.trim().length < 3 || past} loading={create.isPending} onPress={() => create.mutate()} />
    </View>
  );
}

function FollowButton({ show }: { show: Show }) {
  const follow = useFollow(show);
  return <Button title={show.following ? 'Following' : 'Follow'} kind={show.following ? 'secondary' : 'primary'} loading={follow.isPending} onPress={() => follow.mutate()} />;
}

export default function ShowScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const show = useShow(id);
  const safety = useSafetySheet();
  const now = useNow(60_000);
  const s = show.data;
  return (
    <SafeAreaView className="flex-1 bg-bg">
      <ScreenHeader
        title={s?.title}
        right={
          s && !s.is_mine ? (
            <Pressable onPress={() => safety.open({ type: 'persona', id: s.host.id, personaId: s.host.id, alias: s.host.alias })} hitSlop={12} accessibilityLabel="Safety options">
              <MoreHorizontal color={colors.ink} size={22} />
            </Pressable>
          ) : null
        }
      />
      {s ? (
        <ScrollView contentContainerClassName="gap-5 p-4">
          <Pressable onPress={() => router.push(`/u/${s.host.id}`)}>
            <PersonaRow p={s.host} subtitle="Host" />
          </Pressable>
          {s.description ? <Text className="text-muted">{s.description}</Text> : null}
          <View className="flex-row flex-wrap gap-2">
            {s.topic_tags.map((t) => (
              <Chip key={t} label={t} />
            ))}
          </View>
          <Text variant="caption">
            {s.follower_count} following{s.schedule ? ` · usually ${s.schedule}` : ''}
          </Text>
          {!s.is_mine ? <FollowButton show={s} /> : null}
          {s.next_episode ? <EpisodeCard ep={s.next_episode} now={now} /> : <Text className="text-muted">Nothing scheduled yet.</Text>}
          {s.is_mine ? <Scheduler show={s} now={now} /> : null}
        </ScrollView>
      ) : null}
    </SafeAreaView>
  );
}
