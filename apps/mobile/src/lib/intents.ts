import { t } from '@/i18n';

const KEYS = ['vent', 'listen', 'travel_buddy', 'friends', 'discuss'] as const;

export function intents(): { key: string; label: string }[] {
  return KEYS.map((key) => ({ key, label: t(`common.intents.${key}`) }));
}

export function intentLabel(key: string) {
  return (KEYS as readonly string[]).includes(key) ? t(`common.intents.${key as (typeof KEYS)[number]}`) : key;
}
