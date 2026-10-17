import { getLocales } from 'expo-localization';
import { useSyncExternalStore } from 'react';

import en from './en';
import hi from './hi';

export type Locale = 'en' | 'hi';

// Every Hindi dictionary must have exactly the English keys, so a missing translation fails typecheck.
type Shape<T> = { [K in keyof T]: T[K] extends string ? string : Shape<T[K]> };
export type Messages = Shape<typeof en>;
const dicts: Record<Locale, Messages> = { en, hi };

type Leaves<T, P extends string = ''> = {
  [K in keyof T & string]: T[K] extends string ? `${P}${K}` : Leaves<T[K], `${P}${K}.`>;
}[keyof T & string];
export type Key = Leaves<typeof en>;
type PluralBase<K> = K extends `${infer B}_one` ? B : never;
export type PluralKey = PluralBase<Key>;

let locale: Locale = getLocales()[0]?.languageCode === 'hi' ? 'hi' : 'en';
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

export function t(key: Key, vars?: Vars): string {
  const s = lookup(dicts[locale], key) ?? lookup(dicts.en, key) ?? key;
  return vars ? s.replace(/\{(\w+)\}/g, (m, k: string) => (k in vars ? String(vars[k]) : m)) : s;
}

// Plurals: key_one / key_other, with {count} available to the string.
export function tp(key: PluralKey, count: number, vars?: Vars): string {
  return t(`${key}_${count === 1 ? 'one' : 'other'}` as Key, { count, ...vars });
}

// Components call this so they re-render when the language changes.
export function useT() {
  useSyncExternalStore(subscribe, getLocale);
  return { t, tp, locale };
}

export function dateLocale() {
  return locale === 'hi' ? 'hi-IN' : 'en-IN';
}
