import { getLocales } from 'expo-localization';
import { useSyncExternalStore } from 'react';

import de from './de';
import en from './en';
import es from './es';
import fr from './fr';
import hi from './hi';
import id from './id';
import ja from './ja';
import pt from './pt';
import ru from './ru';
import zh from './zh';

export const LOCALES = ['en', 'hi', 'es', 'fr', 'de', 'pt', 'ja', 'zh', 'id', 'ru'] as const;
export type Locale = (typeof LOCALES)[number];

// Each language's own name, shown in the picker whatever the current language is.
export const LANGUAGE_NAMES: Record<Locale, string> = {
  en: 'English',
  hi: 'हिन्दी',
  es: 'Español',
  fr: 'Français',
  de: 'Deutsch',
  pt: 'Português',
  ja: '日本語',
  zh: '简体中文',
  id: 'Bahasa Indonesia',
  ru: 'Русский',
};

// Every dictionary must have exactly the English keys, so a missing translation fails typecheck.
type Shape<T> = { [K in keyof T]: T[K] extends string ? string : Shape<T[K]> };
export type Messages = Shape<typeof en>;
const dicts: Record<Locale, Messages> = { en, hi, es, fr, de, pt, ja, zh, id, ru };

type Leaves<T, P extends string = ''> = {
  [K in keyof T & string]: T[K] extends string ? `${P}${K}` : Leaves<T[K], `${P}${K}.`>;
}[keyof T & string];
export type Key = Leaves<typeof en>;
type PluralBase<K> = K extends `${infer B}_one` ? B : never;
export type PluralKey = PluralBase<Key>;

const deviceLanguage = getLocales()[0]?.languageCode ?? 'en';
let locale: Locale = (LOCALES as readonly string[]).includes(deviceLanguage) ? (deviceLanguage as Locale) : 'en';
const listeners = new Set<() => void>();

export function getLocale() {
  return locale;
}

export function setLocale(l: Locale) {
  if (l === locale) return;
  locale = l;
  listeners.forEach((fn) => fn());
}

function subscribe(fn: () => void) {
  listeners.add(fn);
  return () => {
    listeners.delete(fn);
  };
}

function lookup(d: Messages, key: string): string | undefined {
  let cur: unknown = d;
  for (const part of key.split('.')) {
    if (cur == null || typeof cur !== 'object') return undefined;
    cur = (cur as Record<string, unknown>)[part];
  }
  return typeof cur === 'string' ? cur : undefined;
}

type Vars = Record<string, string | number>;

function translate(l: Locale, key: Key, vars?: Vars): string {
  const s = lookup(dicts[l], key) ?? lookup(dicts.en, key) ?? key;
  return vars ? s.replace(/\{(\w+)\}/g, (m, k: string) => (k in vars ? String(vars[k]) : m)) : s;
}

// Plurals: key_one / key_other, with {count} available to the string.
function plural(l: Locale, key: PluralKey, count: number, vars?: Vars): string {
  return translate(l, `${key}_${count === 1 ? 'one' : 'other'}` as Key, { count, ...vars });
}

export function t(key: Key, vars?: Vars): string {
  return translate(locale, key, vars);
}

export function tp(key: PluralKey, count: number, vars?: Vars): string {
  return plural(locale, key, count, vars);
}

// One bound set per language: a new `t` identity on switch is what makes memoized
// (React Compiler) components re-render with the new strings.
const bound = Object.fromEntries(
  LOCALES.map((l) => [
    l,
    { t: (key: Key, vars?: Vars) => translate(l, key, vars), tp: (key: PluralKey, n: number, vars?: Vars) => plural(l, key, n, vars), locale: l },
  ]),
) as Record<Locale, { t: typeof t; tp: typeof tp; locale: Locale }>;

export function useT() {
  return bound[useSyncExternalStore(subscribe, getLocale)];
}

const dateLocales: Record<Locale, string> = {
  en: 'en-IN',
  hi: 'hi-IN',
  es: 'es',
  fr: 'fr',
  de: 'de',
  pt: 'pt-BR',
  ja: 'ja-JP',
  zh: 'zh-CN',
  id: 'id-ID',
  ru: 'ru-RU',
};

export function dateLocale() {
  return dateLocales[locale];
}
