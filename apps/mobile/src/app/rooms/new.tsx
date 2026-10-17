import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { ScrollView, View } from 'react-native';

import { ApiError, post } from '@/api/client';
import type { Conversation } from '@/api/types';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Chip } from '@/components/ui/Chip';
import { Input } from '@/components/ui/Input';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { useJoinedBowls } from '@/features/bowls/hooks';
import { useT, type Key } from '@/i18n';

const STARTS: { label: Key; mins: number }[] = [
  { label: 'chat.rooms.startNow', mins: 0 },
  { label: 'chat.rooms.start30', mins: 30 },
  { label: 'chat.rooms.start120', mins: 120 },
  { label: 'chat.rooms.startTonight', mins: -1 },
];

function startTime(mins: number) {
  if (mins >= 0) return new Date(Date.now() + mins * 60000);
  const d = new Date();
  d.setHours(21, 0, 0, 0);
  if (d < new Date()) d.setDate(d.getDate() + 1);
  return d;
}

export default function NewRoom() {
  const { t } = useT();
  const params = useLocalSearchParams<{ bowl?: string }>();
  const qc = useQueryClient();
  const bowls = useJoinedBowls();
  const [bowl, setBowl] = useState(params.bowl);
  const [title, setTitle] = useState('');
  const [topic, setTopic] = useState('');
  const [start, setStart] = useState(0);
  const [duration, setDuration] = useState(30);
  const [slow, setSlow] = useState(10);
  const [error, setError] = useState<string>();
  const bowlId = bowl ?? bowls.data?.[0]?.id;

  const create = useMutation({
    mutationFn: () =>
      post<Conversation>('/v1/rooms', { bowl_id: bowlId, title: title.trim(), topic: topic.trim(), starts_at: startTime(start).toISOString(), duration_min: duration, slow_mode_sec: slow }),
    onSuccess: (c) => {
      qc.invalidateQueries({ queryKey: ['rooms'] });
      router.replace(`/chat/${c.id}`);
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : t('chat.rooms.failed')),
  });

  return (
    <Screen edges={['top', 'bottom']} className="px-0">
      <ScreenHeader title={t('chat.rooms.schedule')} />
      <ScrollView contentContainerClassName="gap-5 px-5 pb-10" keyboardShouldPersistTaps="handled">
        <View className="gap-2">
          <Text variant="label">{t('chat.rooms.bowl')}</Text>
          <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerClassName="gap-2">
            {(bowls.data ?? []).map((b) => (
              <Chip key={b.id} label={b.name} selected={b.id === bowlId} onPress={() => setBowl(b.id)} />
            ))}
          </ScrollView>
        </View>
        <Input label={t('chat.rooms.titleLabel')} value={title} onChangeText={(v) => setTitle(v.slice(0, 60))} placeholder={t('chat.rooms.titlePlaceholder')} />
        <Input label={t('chat.rooms.topic')} value={topic} onChangeText={(v) => setTopic(v.slice(0, 200))} multiline placeholder={t('chat.rooms.topicPlaceholder')} />
        <View className="gap-2">
          <Text variant="label">{t('chat.rooms.starts')}</Text>
          <View className="flex-row flex-wrap gap-2">
            {STARTS.map((s) => (
              <Chip key={s.label} label={t(s.label)} selected={start === s.mins} onPress={() => setStart(s.mins)} />
            ))}
          </View>
        </View>
        <View className="gap-2">
          <Text variant="label">{t('chat.rooms.length')}</Text>
          <View className="flex-row gap-2">
            {[15, 30, 60, 120].map((d) => (
              <Chip key={d} label={t('chat.rooms.mins', { n: d })} selected={duration === d} onPress={() => setDuration(d)} />
            ))}
          </View>
        </View>
        <View className="gap-2">
          <Text variant="label">{t('chat.rooms.slowModeLabel')}</Text>
          <View className="flex-row gap-2">
            {[0, 10, 30, 60].map((s) => (
              <Chip key={s} label={s ? t('chat.rooms.seconds', { n: s }) : t('chat.rooms.off')} selected={slow === s} onPress={() => setSlow(s)} />
            ))}
          </View>
        </View>
        {error ? <Text className="text-danger">{error}</Text> : null}
        <Button title={t('chat.rooms.scheduleRoom')} disabled={!bowlId || title.trim().length < 3} loading={create.isPending} onPress={() => create.mutate()} />
      </ScrollView>
    </Screen>
  );
}
