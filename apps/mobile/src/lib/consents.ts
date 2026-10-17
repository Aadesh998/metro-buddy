import type { ConsentPurpose } from '@/api/types';
import { t } from '@/i18n';

export function consentCopy(p: ConsentPurpose): { title: string; body: string; required?: boolean } {
  return { title: t(`settings.consent.${p}.title`), body: t(`settings.consent.${p}.body`), required: p === 'core_service' };
}
