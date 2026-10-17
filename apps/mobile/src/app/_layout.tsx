import '../global.css';

import { QueryClientProvider } from '@tanstack/react-query';
import { Stack } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { StatusBar } from 'expo-status-bar';
import { useEffect } from 'react';
import { SafeAreaProvider } from 'react-native-safe-area-context';

import { ToastProvider } from '@/components/ui/Toast';
import { useOnboardingStep } from '@/features/me/hooks';
import { SafetySheetProvider } from '@/features/safety/SafetySheet';
import { AuthProvider, useAuth } from '@/lib/auth';
import { queryClient } from '@/lib/query';
import { colors } from '@/lib/theme';

SplashScreen.preventAutoHideAsync();

export default function RootLayout() {
  return (
    <SafeAreaProvider>
      <QueryClientProvider client={queryClient}>
        <ToastProvider>
          <AuthProvider>
            <SafetySheetProvider>
              <StatusBar style="light" />
              <Navigation />
            </SafetySheetProvider>
          </AuthProvider>
        </ToastProvider>
      </QueryClientProvider>
    </SafeAreaProvider>
  );
}

function Navigation() {
  const { status } = useAuth();
  const signedIn = status === 'signedIn';
  const { step, loading } = useOnboardingStep(signedIn);
  const ready = status !== 'loading' && !loading;

  useEffect(() => {
    if (ready) SplashScreen.hideAsync();
  }, [ready]);

  if (!ready) return null;
  const onboarding = signedIn && step !== null;

  return (
    <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: colors.bg } }}>
      <Stack.Screen name="index" />
      <Stack.Protected guard={!signedIn}>
        <Stack.Screen name="(auth)" />
      </Stack.Protected>
      <Stack.Protected guard={onboarding}>
        <Stack.Screen name="(onboarding)" />
      </Stack.Protected>
      <Stack.Protected guard={signedIn && !onboarding}>
        <Stack.Screen name="(tabs)" />
        <Stack.Screen name="u/[id]" options={{ animation: 'slide_from_right' }} />
        <Stack.Screen name="safety" options={{ animation: 'slide_from_bottom' }} />
        <Stack.Screen name="settings/blocked" options={{ animation: 'slide_from_right' }} />
        <Stack.Screen name="settings/profile" options={{ animation: 'slide_from_right' }} />
        <Stack.Screen name="settings/privacy" options={{ animation: 'slide_from_right' }} />
      </Stack.Protected>
    </Stack>
  );
}
