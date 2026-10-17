import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { View } from 'react-native';

import { put } from '@/api/client';
import { StepHeader } from '@/components/StepHeader';
import { MAX_VIBES, MIN_VIBES, VibePicker } from '@/components/VibePicker';
import { Button } from '@/components/ui/Button';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { meKeys } from '@/features/me/hooks';
import { useT } from '@/i18n';

export default function VibesScreen() {
  const qc = useQueryClient();
  const { t } = useT();
  const [selected, setSelected] = useState<string[]>([]);
  const save = useMutation({
    mutationFn: () => put('/v1/me/vibes', { slugs: selected }),
    onSuccess: () => qc.invalidateQueries({ queryKey: meKeys.me }),
  });

  return (
    <View className="flex-1 bg-bg">
      <Screen scroll edges={['top']}>
        <StepHeader step="vibes" title={t('onboarding.vibes.title')} subtitle={t('onboarding.vibes.subtitle', { min: MIN_VIBES, max: MAX_VIBES })} />
        <VibePicker selected={selected} onChange={setSelected} />
      </Screen>
      <View className="gap-2 border-t border-line bg-bg px-5 pb-8 pt-3">
        <Text variant="caption" className="text-center">
          {t('onboarding.vibes.selected', { count: selected.length, max: MAX_VIBES })}
        </Text>
        <Button title={t('onboarding.vibes.finish')} disabled={selected.length < MIN_VIBES} loading={save.isPending} onPress={() => save.mutate()} />
      </View>
    </View>
  );
}
