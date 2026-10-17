import type { ReactNode } from 'react';
import { View } from 'react-native';

import { Text } from '@/components/ui/Text';

export function EmptyState({ icon, title, body, action }: { icon?: ReactNode; title: string; body?: string; action?: ReactNode }) {
  return (
    <View className="flex-1 items-center justify-center gap-3 px-8 py-16">
      {icon}
      <Text variant="heading" className="text-center">
        {title}
      </Text>
      {body ? <Text className="text-center text-muted">{body}</Text> : null}
      {action}
    </View>
  );
}
