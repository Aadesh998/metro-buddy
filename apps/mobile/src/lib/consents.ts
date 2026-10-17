import type { ConsentPurpose } from '@/api/types';

export const CONSENT_COPY: Record<ConsentPurpose, { title: string; body: string; required?: boolean }> = {
  core_service: {
    title: 'Run my account',
    body: 'Store my phone number (encrypted), my anonymous persona and what I post so the app works. Required.',
    required: true,
  },
  travel_matching: {
    title: 'Travel Buddy matching',
    body: 'Use my commute stations and times to suggest people on the same route. Only the shared part of a trip is ever shown.',
  },
  ai_processing: {
    title: 'AI safety checks',
    body: 'Let an AI double-check my messages for hidden phone numbers or handles before they’re shared. It never acts on its own.',
  },
  analytics: {
    title: 'Anonymous usage stats',
    body: 'Count which features get used, with no link to me or my persona.',
  },
  marketing_push: {
    title: 'Product updates',
    body: 'Occasional notifications about new features and events in my city.',
  },
};
