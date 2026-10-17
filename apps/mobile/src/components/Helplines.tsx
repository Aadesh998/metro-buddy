import { Phone } from 'lucide-react-native';
import { Linking, Pressable, View } from 'react-native';

import { Text } from '@/components/ui/Text';
import { HELPLINES } from '@/lib/helplines';
import { colors } from '@/lib/theme';

export function Helplines({ only }: { only?: string[] }) {
  const list = only ? HELPLINES.filter((h) => only.includes(h.number)) : HELPLINES;
  return (
    <View className="gap-2">
      {list.map((h) => (
        <Pressable
          key={h.number}
          onPress={() => Linking.openURL(`tel:${h.number}`)}
          className="flex-row items-center gap-3 rounded-xl border border-line bg-surface p-3 active:bg-raised"
          accessibilityRole="button"
          accessibilityLabel={`Call ${h.name} ${h.number}`}
        >
          <View className="h-10 w-10 items-center justify-center rounded-full bg-mint/15">
            <Phone color={colors.mint} size={18} />
          </View>
          <View className="flex-1">
            <Text variant="label">
              {h.name} · <Text className="text-mint">{h.number}</Text>
            </Text>
            <Text variant="caption">{h.note}</Text>
          </View>
        </Pressable>
      ))}
    </View>
  );
}
