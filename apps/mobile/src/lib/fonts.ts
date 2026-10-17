import AsyncStorage from '@react-native-async-storage/async-storage';
import {
  Poppins_400Regular,
  Poppins_500Medium,
  Poppins_600SemiBold,
  Poppins_700Bold,
  Poppins_800ExtraBold,
} from '@expo-google-fonts/poppins';
import { useSyncExternalStore } from 'react';
import type { TextStyle } from 'react-native';

export type FontChoice = 'poppins' | 'system';

export const poppinsFonts = { Poppins_400Regular, Poppins_500Medium, Poppins_600SemiBold, Poppins_700Bold, Poppins_800ExtraBold };

// Android picks a font file by exact name, so every weight maps to its own family.
const poppinsByWeight = {
  regular: 'Poppins_400Regular',
  medium: 'Poppins_500Medium',
  semibold: 'Poppins_600SemiBold',
  bold: 'Poppins_700Bold',
  black: 'Poppins_800ExtraBold',
} as const;

type Weight = keyof typeof poppinsByWeight;

const KEY = 'mb.font';
let choice: FontChoice = 'poppins';
const listeners = new Set<() => void>();

AsyncStorage.getItem(KEY)
  .then((v) => {
    if (v === 'system' || v === 'poppins') setFont(v, false);
  })
  .catch(() => {});

export function setFont(next: FontChoice, persist = true) {
  if (next === choice) return;
  choice = next;
  listeners.forEach((l) => l());
  if (persist) AsyncStorage.setItem(KEY, next).catch(() => {});
}

function subscribe(l: () => void) {
  listeners.add(l);
  return () => {
    listeners.delete(l);
  };
}

export function useFontChoice() {
  return useSyncExternalStore(subscribe, () => choice);
}

function weightOf(className: string): Weight {
  if (/\bfont-(black|extrabold)\b/.test(className)) return 'black';
  if (/\bfont-bold\b/.test(className)) return 'bold';
  if (/\bfont-semibold\b/.test(className)) return 'semibold';
  if (/\bfont-medium\b/.test(className)) return 'medium';
  return 'regular';
}

// Style to apply on top of the className; empty for the system font.
export function fontStyle(font: FontChoice, className: string): TextStyle | undefined {
  if (font === 'system') return undefined;
  return { fontFamily: poppinsByWeight[weightOf(className)], fontWeight: 'normal' };
}
