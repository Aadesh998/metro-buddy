import { forwardRef } from 'react';
import { TextInput, View, type TextInputInstance, type TextInputProps } from 'react-native';

import { colors } from '@/lib/theme';
import { Text } from './Text';

export const Input = forwardRef<TextInputInstance, TextInputProps & { label?: string; error?: string; className?: string }>(
  function Input({ label, error, className = '', ...rest }, ref) {
    return (
      <View className="gap-1.5">
        {label ? <Text variant="label">{label}</Text> : null}
        <TextInput
          ref={ref}
          placeholderTextColor={colors.faint}
          className={`min-h-12 rounded-xl border bg-surface px-4 py-3 text-base text-ink ${error ? 'border-danger' : 'border-line'} ${className}`}
          {...rest}
        />
        {error ? <Text className="text-xs text-danger">{error}</Text> : null}
      </View>
    );
  },
);
