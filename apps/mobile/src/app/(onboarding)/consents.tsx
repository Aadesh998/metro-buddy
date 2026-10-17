import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Linking, Switch, View } from 'react-native';

import { post } from '@/api/client';
import type { ConsentPurpose } from '@/api/types';
import { StepHeader } from '@/components/StepHeader';
import { Button } from '@/components/ui/Button';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { meKeys, useMe } from '@/features/me/hooks';
import { useT } from '@/i18n';
import { consentCopy } from '@/lib/consents';
import { API_URL } from '@/lib/config';
import { colors } from '@/lib/theme';


const ORDER: ConsentPurpose[] = ['core_service', 'travel_matching', 'ai_processing', 'analytics', 'marketing_push'];

export default function ConsentsScreen() {
  const qc = useQueryClient();
  const { data: me } = useMe();
  const { t } = useT();
  const [state, setState] = useState<Record<ConsentPurpose, boolean>>({
    core_service: false,
    travel_matching: true,
    ai_processing: true,
    analytics: false,
    marketing_push: false,
  });

  const save = useMutation({
    mutationFn: () =>
      post('/v1/me/consents', {
        policy_version: me?.policy_version,
        items: ORDER.map((p) => ({ purpose: p, granted: state[p] })),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: meKeys.me }),
  });

  return (
    <Screen scroll>
      <StepHeader step="consents" title={t('onboarding.consents.title')} subtitle={t('onboarding.consents.subtitle')} />
      <View className="gap-3">
        {ORDER.map((p) => {
          const copy = consentCopy(p);
          return (
            <View key={p} className="flex-row items-start gap-4 rounded-2xl border border-line bg-surface p-4">
              <View className="flex-1 gap-1">
                <Text variant="label">
                  {copy.title}
                  {copy.required ? <Text className="text-xs text-accent">{`  ${t('onboarding.consents.required')}`}</Text> : null}
                </Text>
                <Text variant="caption" className="leading-5">
                  {copy.body}
                </Text>
              </View>
              <Switch
                value={state[p]}
                onValueChange={(v) => setState((s) => ({ ...s, [p]: v }))}
                trackColor={{ true: colors.accent, false: colors.line }}
                thumbColor={colors.ink}
                accessibilityLabel={copy.title}
              />
            </View>
          );
        })}
      </View>
      <Text variant="caption" className="mt-4" onPress={() => Linking.openURL(`${API_URL}/privacy`)}>
        {t('onboarding.consents.readBefore')}
        <Text className="text-xs text-accent">{t('onboarding.consents.policy')}</Text>
        {t('onboarding.consents.readAfter', { version: me?.policy_version ?? '' })}
      </Text>
      {!state.core_service ? (
        <Text className="mt-4 text-sm text-muted">{t('onboarding.consents.turnOnCore')}</Text>
      ) : null}
      <Button title={t('onboarding.consents.agree')} className="mt-6" disabled={!state.core_service} loading={save.isPending} onPress={() => save.mutate()} />
    </Screen>
  );
}
