import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Switch, View } from 'react-native';

import { get, post } from '@/api/client';
import type { ConsentState } from '@/api/types';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { CONSENT_COPY } from '@/lib/consents';
import { colors } from '@/lib/theme';

export default function PrivacyScreen() {
  const qc = useQueryClient();
  const q = useQuery({
    queryKey: ['me', 'consents'],
    queryFn: () => get<{ policy_version: string; items: ConsentState[] }>('/v1/me/consents'),
  });
  const set = useMutation({
    mutationFn: (it: ConsentState) =>
      post('/v1/me/consents', { policy_version: q.data!.policy_version, items: [{ purpose: it.purpose, granted: !it.granted }] }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['me'] }),
  });

  return (
    <Screen scroll className="gap-4">
      <ScreenHeader title="Privacy & your data" />
      <Text variant="heading">What you’ve allowed</Text>
      {(q.data?.items ?? []).map((it) => (
        <View key={it.purpose} className="flex-row items-start gap-4 rounded-2xl border border-line bg-surface p-4">
          <View className="flex-1 gap-1">
            <Text variant="label">{CONSENT_COPY[it.purpose].title}</Text>
            <Text variant="caption" className="leading-5">
              {CONSENT_COPY[it.purpose].body}
            </Text>
            {it.purpose === 'core_service' ? (
              <Text variant="caption" className="text-accent">
                Turning this off pauses your account.
              </Text>
            ) : null}
          </View>
          <Switch
            value={it.granted}
            onValueChange={() => set.mutate(it)}
            trackColor={{ true: colors.accent, false: colors.line }}
            thumbColor={colors.ink}
            accessibilityLabel={CONSENT_COPY[it.purpose].title}
          />
        </View>
      ))}
    </Screen>
  );
}
