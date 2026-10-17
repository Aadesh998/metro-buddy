import { router } from 'expo-router';
import { ChevronLeft } from 'lucide-react-native';
import type { ReactNode } from 'react';
import { Pressable, View } from 'react-native';

import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';
import { colors } from '@/lib/theme';

export function ScreenHeader({ title, right }: { title?: string; right?: ReactNode }) {
  const { t } = useT();
  return (
    <View className="h-12 flex-row items-center justify-between px-3">
      <Pressable onPress={() => router.back()} hitSlop={12} accessibilityLabel={t('common.back')} className="h-10 w-10 items-center justify-center">
        <ChevronLeft color={colors.ink} size={26} />
      </Pressable>
      <Text variant="heading" numberOfLines={1} className="flex-1 text-center">
        {title ?? ''}
      </Text>
      <View className="h-10 w-10 items-center justify-center">{right}</View>
    </View>
  );
}
