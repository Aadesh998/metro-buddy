import { t } from '@/i18n';

export type Helpline = { name: string; number: string; note: string };

const LIST = [
  { id: 'emergency', number: '112' },
  { id: 'teleManas', number: '14416' },
  { id: 'women', number: '181' },
  { id: 'cyber', number: '1930' },
  { id: 'dmrc', number: '155370' },
] as const;

export function helplines(): Helpline[] {
  return LIST.map((h) => ({ name: t(`safety.helplines.${h.id}.name`), number: h.number, note: t(`safety.helplines.${h.id}.note`) }));
}
