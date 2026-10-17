import { dateLocale, t } from '@/i18n';

export function ago(iso: string) {
  const s = Math.max(0, (Date.now() - Date.parse(iso)) / 1000);
  if (s < 60) return t('common.time.now');
  if (s < 3600) return t('common.time.minutes', { n: Math.floor(s / 60) });
  if (s < 86400) return t('common.time.hours', { n: Math.floor(s / 3600) });
  if (s < 86400 * 30) return t('common.time.days', { n: Math.floor(s / 86400) });
  return new Date(iso).toLocaleDateString(dateLocale());
}

export function until(iso: string) {
  const s = (Date.parse(iso) - Date.now()) / 1000;
  if (s <= 0) return t('common.time.closed');
  if (s < 3600) return t('common.time.minutesLeft', { n: Math.ceil(s / 60) });
  if (s < 86400) return t('common.time.hoursLeft', { n: Math.floor(s / 3600) });
  return t('common.time.daysLeft', { n: Math.floor(s / 86400) });
}
