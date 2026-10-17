import AsyncStorage from '@react-native-async-storage/async-storage';
import { vars } from 'nativewind';
import { useSyncExternalStore } from 'react';
import { Appearance } from 'react-native';

// Brand: mostly warm paper and near-black; plum, teal and coral are small accents
// (coral only for alerts and incomplete fields).
type Palette = {
  bg: string;
  surface: string;
  raised: string;
  line: string;
  ink: string;
  muted: string;
  faint: string;
  accent: string;
  accentInk: string;
  brand: string;
  brandSoft: string;
  mint: string;
  danger: string;
  sky: string;
};

const light: Palette = {
  bg: '#FAF7F3',
  surface: '#FFFFFF',
  raised: '#F1EAE4',
  line: '#E3DAD2',
  ink: '#1B1A19',
  muted: '#5F5A56',
  faint: '#9C948D',
  accent: '#1B1A19',
  accentInk: '#FAF7F3',
  brand: '#5E2B63',
  brandSoft: '#ECE4F1',
  mint: '#0C6B68',
  danger: '#CB5442',
  sky: '#5B4FA0',
};

const dark: Palette = {
  bg: '#141312',
  surface: '#1C1A19',
  raised: '#272422',
  line: '#35302D',
  ink: '#F5F1EC',
  muted: '#ADA59E',
  faint: '#766F69',
  accent: '#F5F1EC',
  accentInk: '#141312',
  brand: '#B993C4',
  brandSoft: '#2E2433',
  mint: '#48B7AB',
  danger: '#E4725F',
  sky: '#A79BDD',
};

export type ThemeChoice = 'light' | 'dark' | 'system';
export type Scheme = 'light' | 'dark';

const KEY = 'mb.theme';
let choice: ThemeChoice = 'light';
let scheme: Scheme = 'light';
const listeners = new Set<() => void>();

// Read live by every component: always reflects the active theme.
export const colors: Palette = { ...light };

function apply() {
  const next: Scheme = choice === 'system' ? (Appearance.getColorScheme() === 'dark' ? 'dark' : 'light') : choice;
  scheme = next;
  Object.assign(colors, next === 'dark' ? dark : light);
  listeners.forEach((l) => l());
}

AsyncStorage.getItem(KEY)
  .then((v) => {
    if (v === 'light' || v === 'dark' || v === 'system') setTheme(v, false);
  })
  .catch(() => {});

Appearance.addChangeListener(() => {
  if (choice === 'system') apply();
});

export function setTheme(next: ThemeChoice, persist = true) {
  if (next === choice) return;
  choice = next;
  apply();
  if (persist) AsyncStorage.setItem(KEY, next).catch(() => {});
}

function subscribe(l: () => void) {
  listeners.add(l);
  return () => {
    listeners.delete(l);
  };
}

export function useThemeChoice() {
  return useSyncExternalStore(subscribe, () => choice);
}

export function useScheme() {
  return useSyncExternalStore(subscribe, () => scheme);
}

const channels = (hex: string) => {
  const n = parseInt(hex.slice(1), 16);
  return `${(n >> 16) & 255} ${(n >> 8) & 255} ${n & 255}`;
};

function cssVars(p: Palette) {
  return vars({
    '--c-bg': channels(p.bg),
    '--c-surface': channels(p.surface),
    '--c-raised': channels(p.raised),
    '--c-line': channels(p.line),
    '--c-ink': channels(p.ink),
    '--c-muted': channels(p.muted),
    '--c-faint': channels(p.faint),
    '--c-accent': channels(p.accent),
    '--c-accent-ink': channels(p.accentInk),
    '--c-brand': channels(p.brand),
    '--c-brand-soft': channels(p.brandSoft),
    '--c-mint': channels(p.mint),
    '--c-danger': channels(p.danger),
    '--c-sky': channels(p.sky),
  });
}

export const themeVars = { light: cssVars(light), dark: cssVars(dark) };
