import { Redirect } from 'expo-router';

import { useOnboardingStep } from '@/features/me/hooks';
import { useAuth } from '@/lib/auth';

export default function Index() {
  const { status } = useAuth();
  const { step } = useOnboardingStep(status === 'signedIn');
  if (status !== 'signedIn') return <Redirect href="/phone" />;
  if (step) return <Redirect href={`/${step}`} />;
  return <Redirect href="/bowls" />;
}
