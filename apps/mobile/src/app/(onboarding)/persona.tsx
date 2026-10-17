import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { RefreshCw } from 'lucide-react-native';
import { useState } from 'react';
import { Pressable, View } from 'react-native';

import { post } from '@/api/client';
import { Avatar } from '@/components/Avatar';
import { StepHeader } from '@/components/StepHeader';
import { Button } from '@/components/ui/Button';
import { Chip } from '@/components/ui/Chip';
import { Input } from '@/components/ui/Input';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { useCities } from '@/features/metro/hooks';
import { useT } from '@/i18n';
import { intents as intentOptions } from '@/lib/intents';
import { colors } from '@/lib/theme';

type Suggestion = { alias: string; avatar_seed: string };


export default function PersonaScreen() {
  const qc = useQueryClient();
  const cities = useCities();
  const { t } = useT();
  const [chosenCity, setCityId] = useState<string>();
  const [picked, setPicked] = useState(0);
  const [intents, setIntents] = useState<string[]>(['vent']);
  const [bio, setBio] = useState('');

  const cityId = chosenCity ?? cities.data?.find((c) => c.is_live)?.id ?? cities.data?.[0]?.id;

  const suggestions = useQuery({
    queryKey: ['persona-suggestions', cityId],
    queryFn: () => post<{ suggestions: Suggestion[] }>('/v1/me/persona/suggestions', { city_id: cityId }).then((r) => r.suggestions),
    enabled: !!cityId,
    staleTime: Infinity,
  });

  const create = useMutation({
    mutationFn: (s: Suggestion) => post('/v1/me/persona', { city_id: cityId, alias: s.alias, avatar_seed: s.avatar_seed, bio, intents }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['me'] }),
    onError: (e) => {
      if ((e as { status?: number }).status === 409) suggestions.refetch();
    },
  });

  const list = suggestions.data ?? [];
  const current = list[picked];

  return (
    <Screen scroll>
      <StepHeader step="persona" title={t('onboarding.persona.title')} subtitle={t('onboarding.persona.subtitle')} />

      {cities.data && cities.data.length > 1 ? (
        <View className="mb-5 flex-row flex-wrap gap-2">
          {cities.data.map((c) => (
            <Chip key={c.id} label={c.name} selected={c.id === cityId} onPress={c.is_live ? () => setCityId(c.id) : undefined} />
          ))}
        </View>
      ) : null}

      <View className="items-center gap-4 rounded-2xl border border-line bg-surface p-6">
        {current ? <Avatar seed={current.avatar_seed} size={96} /> : <View className="h-24 w-24 rounded-full bg-raised" />}
        <Text className="text-2xl font-bold">{current?.alias ?? '…'}</Text>
        <View className="flex-row gap-3">
          {list.map((s, i) => (
            <Pressable
              key={s.alias}
              onPress={() => setPicked(i)}
              className={`rounded-full border-2 p-0.5 ${i === picked ? 'border-accent' : 'border-transparent'}`}
              accessibilityLabel={t('onboarding.persona.choose', { alias: s.alias })}
            >
              <Avatar seed={s.avatar_seed} size={44} />
            </Pressable>
          ))}
          <Pressable
            onPress={() => {
              setPicked(0);
              suggestions.refetch();
            }}
            className="h-12 w-12 items-center justify-center rounded-full border border-line bg-raised"
            accessibilityLabel={t('onboarding.persona.reroll')}
          >
            <RefreshCw color={colors.accent} size={20} />
          </Pressable>
        </View>
      </View>

      <Text variant="label" className="mb-2 mt-6">
        {t('onboarding.persona.hereFor')}
      </Text>
      <View className="flex-row flex-wrap gap-2">
        {intentOptions().map((it) => (
          <Chip
            key={it.key}
            label={it.label}
            selected={intents.includes(it.key)}
            onPress={() => setIntents((xs) => (xs.includes(it.key) ? xs.filter((x) => x !== it.key) : [...xs, it.key]))}
          />
        ))}
      </View>

      <View className="mt-6">
        <Input
          label={t('onboarding.persona.bioLabel')}
          value={bio}
          onChangeText={(s) => setBio(s.slice(0, 160))}
          placeholder={t('onboarding.persona.bioPlaceholder')}
          multiline
        />
        <Text variant="caption" className="mt-1">
          {t('onboarding.persona.bioNote', { count: bio.length })}
        </Text>
      </View>

      <Button
        title={t('onboarding.persona.continueAs', { alias: current?.alias ?? '…' })}
        className="mt-8"
        disabled={!current || intents.length === 0}
        loading={create.isPending}
        onPress={() => current && create.mutate(current)}
      />
    </Screen>
  );
}
