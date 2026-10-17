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

const STARTS = [
  { label: 'Now', mins: 0 },
  { label: 'In 30 min', mins: 30 },
  { label: 'In 2 hours', mins: 120 },
  { label: 'Tonight 9pm', mins: -1 },
];

function startTime(mins: number) {
  if (mins >= 0) return new Date(Date.now() + mins * 60000);
  const d = new Date();
  d.setHours(21, 0, 0, 0);
  if (d < new Date()) d.setDate(d.getDate() + 1);
  return d;
}

export default function NewRoom() {
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
    onError: (e) => setError(e instanceof ApiError ? e.message : 'Failed'),
  });

  return (
    <Screen edges={['top', 'bottom']} className="px-0">
      <ScreenHeader title="Schedule a room" />
      <ScrollView contentContainerClassName="gap-5 px-5 pb-10" keyboardShouldPersistTaps="handled">
        <View className="gap-2">
          <Text variant="label">Bowl</Text>
          <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerClassName="gap-2">
            {(bowls.data ?? []).map((b) => (
              <Chip key={b.id} label={b.name} selected={b.id === bowlId} onPress={() => setBowl(b.id)} />
            ))}
          </ScrollView>
        </View>
        <Input label="Title" value={title} onChangeText={(t) => setTitle(t.slice(0, 60))} placeholder="Monday Office Rant" />
        <Input label="What's it about?" value={topic} onChangeText={(t) => setTopic(t.slice(0, 200))} multiline placeholder="Bring your worst meeting story." />
        <View className="gap-2">
          <Text variant="label">Starts</Text>
          <View className="flex-row flex-wrap gap-2">
            {STARTS.map((s) => (
              <Chip key={s.label} label={s.label} selected={start === s.mins} onPress={() => setStart(s.mins)} />
            ))}
          </View>
        </View>
        <View className="gap-2">
          <Text variant="label">Length</Text>
          <View className="flex-row gap-2">
            {[15, 30, 60, 120].map((d) => (
              <Chip key={d} label={`${d} min`} selected={duration === d} onPress={() => setDuration(d)} />
            ))}
          </View>
        </View>
        <View className="gap-2">
          <Text variant="label">Slow mode</Text>
          <View className="flex-row gap-2">
            {[0, 10, 30, 60].map((s) => (
              <Chip key={s} label={s ? `${s}s` : 'Off'} selected={slow === s} onPress={() => setSlow(s)} />
            ))}
          </View>
        </View>
        {error ? <Text className="text-danger">{error}</Text> : null}
        <Button title="Schedule room" disabled={!bowlId || title.trim().length < 3} loading={create.isPending} onPress={() => create.mutate()} />
      </ScrollView>
    </Screen>
  );
}
