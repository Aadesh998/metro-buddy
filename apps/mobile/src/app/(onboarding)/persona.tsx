import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { RefreshCw } from 'lucide-react-native';
import { useState } from 'react';
import { Pressable, View } from 'react-native';

import { ApiError, post } from '@/api/client';
import { Avatar } from '@/components/Avatar';
import { AvatarPicker } from '@/components/AvatarPicker';
import { StepHeader } from '@/components/StepHeader';
import { Button } from '@/components/ui/Button';
import { Chip } from '@/components/ui/Chip';
import { Input } from '@/components/ui/Input';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { AliasField, useAliasCheck } from '@/features/me/AliasField';
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
  const [alias, setAlias] = useState('');
  const [intents, setIntents] = useState<string[]>(['vent']);
  const [bio, setBio] = useState('');
  const [avatar, setAvatar] = useState<string>();

  const cityId = chosenCity ?? cities.data?.find((c) => c.is_live)?.id ?? cities.data?.[0]?.id;

  const suggestions = useQuery({
    queryKey: ['persona-suggestions', cityId],
    queryFn: () => post<{ suggestions: Suggestion[] }>('/v1/me/persona/suggestions', { city_id: cityId }).then((r) => r.suggestions),
    enabled: !!cityId,
    staleTime: Infinity,
  });

  const create = useMutation({
    mutationFn: () =>
      post('/v1/me/persona', { city_id: cityId, alias: alias.trim(), avatar_seed: avatar ?? patternSeed, bio, intents }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['me'] }),
  });

  const ideas = suggestions.data ?? [];
  // A generated pattern avatar is the default until an illustration is picked.
  const patternSeed = ideas[0]?.avatar_seed ?? 'metrobuddy01';
  const check = useAliasCheck(alias.trim(), cityId);

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

      <View className="items-center gap-3 rounded-2xl border border-line bg-surface p-6">
        <Avatar seed={avatar ?? patternSeed} size={96} />
        <Text variant="heading" numberOfLines={1}>
          {alias.trim() || '…'}
        </Text>
      </View>

      <View className="mt-6 gap-2">
        <AliasField value={alias} onChange={setAlias} cityId={cityId} />
        {ideas.length ? (
          <View className="flex-row flex-wrap items-center gap-2">
            <Text variant="caption">{t('onboarding.persona.ideas')}</Text>
            {ideas.map((s) => (
              <Chip key={s.alias} label={s.alias} onPress={() => setAlias(s.alias)} />
            ))}
            <Pressable
              onPress={() => suggestions.refetch()}
              hitSlop={8}
              className="h-8 w-8 items-center justify-center rounded-full border border-line"
              accessibilityLabel={t('onboarding.persona.reroll')}
            >
              <RefreshCw color={colors.brand} size={14} />
            </Pressable>
          </View>
        ) : null}
      </View>

      <View className="mt-6 gap-2">
        <View className="flex-row items-center justify-between">
          <Text variant="label">{t('onboarding.persona.avatarTitle')}</Text>
          {avatar ? (
            <Pressable onPress={() => setAvatar(undefined)} hitSlop={8}>
              <Text className="text-sm font-semibold text-brand">{t('onboarding.persona.usePattern')}</Text>
            </Pressable>
          ) : null}
        </View>
        <Text variant="caption">{t('onboarding.persona.avatarHint')}</Text>
        <AvatarPicker value={avatar} onChange={setAvatar} size={48} />
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
        title={t('onboarding.persona.continueAs', { alias: alias.trim() || '…' })}
        className="mt-8"
        disabled={check.status !== 'available' || intents.length === 0 || !cityId}
        loading={create.isPending}
        onPress={() => create.mutate()}
      />
      {create.error ? (
        <Text className="mt-2 text-center text-sm text-danger">{create.error instanceof ApiError ? create.error.message : t('chat.rooms.failed')}</Text>
      ) : null}
    </Screen>
  );
}
