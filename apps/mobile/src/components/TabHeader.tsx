import type { ReactNode } from 'react';
import { View } from 'react-native';

import { Text } from '@/components/ui/Text';

export function TabHeader({ title, right }: { title: string; right?: ReactNode }) {
  return (
    <View className="flex-row items-center justify-between px-5 pb-3 pt-2">
      <Text variant="title" className="text-2xl">
        {title}
      </Text>
      {right}
    </View>
  );
}
