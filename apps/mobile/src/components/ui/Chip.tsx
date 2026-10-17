import { Pressable } from 'react-native';

import { Text } from './Text';

export function Chip({
  label,
  selected,
  onPress,
  color,
}: {
  label: string;
  selected?: boolean;
  onPress?: () => void;
  color?: string;
}) {
  return (
    <Pressable
      onPress={onPress}
      disabled={!onPress}
      accessibilityRole={onPress ? 'button' : undefined}
      accessibilityState={{ selected }}
      className={`flex-row items-center gap-1.5 rounded-full border px-3 py-1.5 ${selected ? 'border-accent bg-accent/15' : 'border-line bg-surface'}`}
    >
      {color ? <Text style={{ color }} className="text-xs">●</Text> : null}
      <Text className={`text-sm ${selected ? 'text-accent' : 'text-ink'}`}>{label}</Text>
    </Pressable>
  );
}
