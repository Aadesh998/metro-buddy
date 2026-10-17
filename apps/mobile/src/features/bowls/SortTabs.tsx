import { Pressable, View } from 'react-native';

import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';
import type { Sort } from './hooks';

export function SortTabs({ value, onChange }: { value: Sort; onChange: (s: Sort) => void }) {
  const { t } = useT();
  return (
    <View className="flex-row gap-1 rounded-full bg-raised p-1">
      {(['hot', 'new', 'top'] as Sort[]).map((s) => (
        <Pressable key={s} onPress={() => onChange(s)} className={`rounded-full px-3 py-1 ${value === s ? 'bg-bg' : ''}`} accessibilityState={{ selected: value === s }}>
          <Text className={`text-xs font-semibold capitalize ${value === s ? 'text-ink' : 'text-muted'}`}>{t(`bowls.sort.${s}`)}</Text>
        </Pressable>
      ))}
    </View>
  );
}
