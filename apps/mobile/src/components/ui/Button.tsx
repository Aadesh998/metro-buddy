import { ActivityIndicator, Pressable, type PressableProps } from 'react-native';

import { colors } from '@/lib/theme';
import { Text } from './Text';

type Kind = 'primary' | 'secondary' | 'ghost' | 'danger';

const box: Record<Kind, string> = {
  primary: 'bg-accent',
  secondary: 'bg-raised border border-line',
  ghost: 'bg-transparent',
  danger: 'bg-danger',
};

const label: Record<Kind, string> = {
  primary: 'text-accent-ink',
  secondary: 'text-ink',
  ghost: 'text-accent',
  danger: 'text-white',
};

export function Button({
  title,
  kind = 'primary',
  loading,
  disabled,
  className = '',
  ...rest
}: PressableProps & { title: string; kind?: Kind; loading?: boolean; className?: string }) {
  const off = disabled || loading;
  return (
    <Pressable
      accessibilityRole="button"
      disabled={off}
      className={`h-12 flex-row items-center justify-center rounded-xl px-5 ${box[kind]} ${off ? 'opacity-50' : 'active:opacity-80'} ${className}`}
      {...rest}
    >
      {loading ? (
        <ActivityIndicator color={kind === 'primary' ? colors.accentInk : colors.ink} />
      ) : (
        <Text className={`text-base font-semibold ${label[kind]}`}>{title}</Text>
      )}
    </Pressable>
  );
}
