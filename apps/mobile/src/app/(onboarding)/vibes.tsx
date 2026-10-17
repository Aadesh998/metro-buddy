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

export default function VibesScreen() {
  const qc = useQueryClient();
  const [selected, setSelected] = useState<string[]>([]);
  const save = useMutation({
    mutationFn: () => put('/v1/me/vibes', { slugs: selected }),
    onSuccess: () => qc.invalidateQueries({ queryKey: meKeys.me }),
  });

  return (
    <View className="flex-1 bg-bg">
      <Screen scroll edges={['top']}>
        <StepHeader step="vibes" title="What’s your vibe?" subtitle={`Pick ${MIN_VIBES} to ${MAX_VIBES}. They help match you with people you’ll actually get along with.`} />
        <VibePicker selected={selected} onChange={setSelected} />
      </Screen>
      <View className="gap-2 border-t border-line bg-bg px-5 pb-8 pt-3">
        <Text variant="caption" className="text-center">
          {selected.length}/{MAX_VIBES} selected
        </Text>
        <Button title="Finish" disabled={selected.length < MIN_VIBES} loading={save.isPending} onPress={() => save.mutate()} />
      </View>
    </View>
  );
}
