import type { ReactNode } from 'react';
import { ScrollView, View } from 'react-native';
import { KeyboardAvoidingView } from 'react-native-keyboard-controller';
import { SafeAreaView, type Edge } from 'react-native-safe-area-context';

export function Screen({
  children,
  scroll,
  edges = ['top', 'bottom'],
  className = '',
}: {
  children: ReactNode;
  scroll?: boolean;
  edges?: Edge[];
  className?: string;
}) {
  const body = scroll ? (
    <ScrollView contentContainerClassName={`px-5 pb-8 ${className}`} keyboardShouldPersistTaps="handled">
      {children}
    </ScrollView>
  ) : (
    <View className={`flex-1 px-5 ${className}`}>{children}</View>
  );
  return (
    <SafeAreaView edges={edges} className="flex-1 bg-bg">
      <KeyboardAvoidingView className="flex-1" behavior="padding">
        {body}
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
