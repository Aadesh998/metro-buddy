import { useQuery } from '@tanstack/react-query';
import { router, useLocalSearchParams } from 'expo-router';
import { MoreHorizontal } from 'lucide-react-native';
import { Pressable, View } from 'react-native';

import { get } from '@/api/client';
import type { PublicPersona } from '@/api/types';
import { Avatar } from '@/components/Avatar';
import { EmptyState } from '@/components/EmptyState';
import { PersonaName } from '@/components/PersonaBadge';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Chip } from '@/components/ui/Chip';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { useSafetySheet } from '@/features/safety/SafetySheet';
import { useT } from '@/i18n';
import { intentLabel } from '@/lib/intents';
import { colors } from '@/lib/theme';

export default function PersonaProfile() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { t, locale } = useT();
  const safety = useSafetySheet();
  const q = useQuery({ queryKey: ['persona', id], queryFn: () => get<PublicPersona>(`/v1/personas/${id}`) });
  const p = q.data;

  return (
    <Screen edges={['top']} className="px-0">
      <ScreenHeader
        right={
          p ? (
            <Pressable
              onPress={() => safety.open({ type: 'persona', id: p.id, personaId: p.id, alias: p.alias })}
              hitSlop={12}
              accessibilityLabel={t('common.safetyOptions')}
            >
              <MoreHorizontal color={colors.ink} size={22} />
            </Pressable>
          ) : null
        }
      />
      {q.isError ? (
        <EmptyState title={t('bowls.profile.unavailableTitle')} body={t('bowls.profile.unavailableBody')} />
      ) : p ? (
        <View className="items-center gap-4 px-5 pt-4">
          <Avatar seed={p.avatar_seed} size={96} />
          <PersonaName p={p} size="lg" />
          {p.bio ? <Text className="text-center text-muted">{p.bio}</Text> : null}
          <View className="flex-row flex-wrap justify-center gap-2">
            {p.intents.map((i) => (
              <Chip key={i} label={intentLabel(i)} />
            ))}
          </View>
          <Button
            title={t('bowls.profile.messageRequest')}
            className="w-full"
            onPress={() => router.push({ pathname: '/dm/new', params: { persona: p.id, alias: p.alias, seed: p.avatar_seed } })}
          />
          <View className="mt-2 w-full gap-2">
            <Text variant="caption" className="uppercase tracking-wider">
              {t('bowls.profile.vibes')}
            </Text>
            <View className="flex-row flex-wrap gap-2">
              {p.vibes.map((v) => (
                <Chip key={v.slug} label={locale === 'hi' ? v.label_hi : v.label_en} />
              ))}
            </View>
          </View>
        </View>
      ) : null}
    </Screen>
  );
}
