import { router, Stack, usePathname } from 'expo-router';
import { useEffect } from 'react';

import { useOnboardingStep } from '@/features/me/hooks';
import { colors } from '@/lib/theme';

export default function OnboardingLayout() {
  const { step } = useOnboardingStep(true);
  const path = usePathname();

  // Each step only saves; this layout moves the user to whatever is still missing.
  useEffect(() => {
    if (step && path !== `/${step}`) router.replace(`/${step}`);
  }, [step, path]);

  return <Stack screenOptions={{ headerShown: false, animation: 'slide_from_right', contentStyle: { backgroundColor: colors.bg } }} />;
}
