import { View } from 'react-native';

import { Text } from '@/components/ui/Text';

const STEPS = ['age', 'consents', 'persona', 'home-metro', 'vibes'];

export function StepHeader({ step, title, subtitle }: { step: string; title: string; subtitle?: string }) {
  const i = STEPS.indexOf(step);
  return (
    <View className="gap-3 pb-6 pt-4">
      <View className="flex-row gap-1.5">
        {STEPS.map((s, j) => (
          <View key={s} className={`h-1 flex-1 rounded-full ${j <= i ? 'bg-accent' : 'bg-raised'}`} />
        ))}
      </View>
      <Text variant="title">{title}</Text>
      {subtitle ? <Text className="text-muted">{subtitle}</Text> : null}
    </View>
  );
}
