import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router, useLocalSearchParams } from 'expo-router';
import { Lightbulb, Minus, MoreHorizontal, Plus } from 'lucide-react-native';
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
import { studioKeys, useFollow, useShow, useTopicIdeas } from '@/features/studio/hooks';
import { dateLocale, useT } from '@/i18n';
import { useNow } from '@/lib/clock';
import { colors } from '@/lib/theme';

const DAY = 86_400_000;

function Scheduler({ show, now }: { show: Show; now: number }) {
  const { t } = useT();
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
  const [showIdeas, setShowIdeas] = useState(false);
  const ideas = useTopicIdeas(showIdeas);
  const step = (d: number) => setMinutes((m) => Math.min(24 * 60 - 15, Math.max(0, m + d)));
  return (
    <View className="gap-3 rounded-2xl border border-line bg-surface p-4">
      <Text variant="heading">{t('studio.show.scheduleTitle')}</Text>
      <Input value={title} onChangeText={(v) => setTitle(v.slice(0, 80))} placeholder={t('studio.show.topicPlaceholder')} />
      {showIdeas ? (
        <View className="gap-2">
          <Text variant="caption">{ideas.data?.from_ai ? t('studio.show.ideasAi') : t('studio.show.ideasBowls')}</Text>
          {(ideas.data?.ideas ?? []).map((i) => (
            <Pressable key={i.topic} onPress={() => setTitle(i.topic.slice(0, 80))} className="gap-0.5 rounded-xl bg-raised p-3 active:opacity-70">
              <Text variant="label">{i.topic}</Text>
              <Text variant="caption">{i.hook}</Text>
            </Pressable>
          ))}
        </View>
      ) : (
        <Pressable onPress={() => setShowIdeas(true)} className="flex-row items-center gap-1.5 self-start" hitSlop={8}>
          <Lightbulb size={15} color={colors.brand} />
          <Text className="text-sm font-medium text-brand">{t('studio.show.needIdea')}</Text>
        </Pressable>
      )}
      <View className="flex-row flex-wrap gap-2">
        {[0, 1, 2, 3, 4, 5, 6].map((d) => (
          <Chip
            key={d}
            label={d === 0 ? t('studio.show.today') : d === 1 ? t('studio.show.tomorrow') : new Date(now + d * DAY).toLocaleDateString(dateLocale(), { weekday: 'short' })}
            selected={day === d}
            onPress={() => setDay(d)}
          />
        ))}
      </View>
      <View className="flex-row items-center justify-center gap-6">
        <Pressable onPress={() => step(-15)} hitSlop={10} accessibilityLabel={t('studio.show.earlier')} className="h-10 w-10 items-center justify-center rounded-full bg-raised">
          <Minus size={18} color={colors.ink} />
        </Pressable>
        <Text className="w-28 text-center font-mono text-2xl font-bold">{at.toLocaleTimeString(dateLocale(), { hour: 'numeric', minute: '2-digit' })}</Text>
        <Pressable onPress={() => step(15)} hitSlop={10} accessibilityLabel={t('studio.show.later')} className="h-10 w-10 items-center justify-center rounded-full bg-raised">
          <Plus size={18} color={colors.ink} />
        </Pressable>
      </View>
      {past ? <Text className="text-center text-xs text-danger">{t('studio.show.passed')}</Text> : null}
      <Text variant="caption">{t('studio.show.hint')}</Text>
      <Button title={t('studio.show.schedule')} disabled={title.trim().length < 3 || past} loading={create.isPending} onPress={() => create.mutate()} />
    </View>
  );
}

function FollowButton({ show }: { show: Show }) {
  const { t } = useT();
  const follow = useFollow(show);
  return <Button title={show.following ? t('common.following') : t('common.follow')} kind={show.following ? 'secondary' : 'primary'} loading={follow.isPending} onPress={() => follow.mutate()} />;
}

export default function ShowScreen() {
  const { t, tp } = useT();
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
            <Pressable onPress={() => safety.open({ type: 'persona', id: s.host.id, personaId: s.host.id, alias: s.host.alias })} hitSlop={12} accessibilityLabel={t('common.safetyOptions')}>
              <MoreHorizontal color={colors.ink} size={22} />
            </Pressable>
          ) : null
        }
      />
      {s ? (
        <ScrollView contentContainerClassName="gap-5 p-4">
          <Pressable onPress={() => router.push(`/u/${s.host.id}`)}>
            <PersonaRow p={s.host} subtitle={t('studio.host')} />
          </Pressable>
          {s.description ? <Text className="text-muted">{s.description}</Text> : null}
          <View className="flex-row flex-wrap gap-2">
            {s.topic_tags.map((tag) => (
              <Chip key={tag} label={tag} />
            ))}
          </View>
          <Text variant="caption">
            {tp('studio.followers', s.follower_count)}
            {s.schedule ? t('studio.show.usually', { schedule: s.schedule }) : ''}
          </Text>
          {!s.is_mine ? <FollowButton show={s} /> : null}
          {s.next_episode ? <EpisodeCard ep={s.next_episode} now={now} /> : <Text className="text-muted">{t('studio.show.nothingScheduled')}</Text>}
          {s.is_mine ? <Scheduler show={s} now={now} /> : null}
        </ScrollView>
      ) : null}
    </SafeAreaView>
  );
}
