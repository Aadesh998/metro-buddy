import AsyncStorage from '@react-native-async-storage/async-storage';
import {
  Inter_400Regular,
  Inter_500Medium,
  Inter_600SemiBold,
  Inter_700Bold,
  Inter_800ExtraBold,
} from '@expo-google-fonts/inter';
import { Lora_400Regular, Lora_500Medium, Lora_600SemiBold, Lora_700Bold } from '@expo-google-fonts/lora';
import {
  Nunito_400Regular,
  Nunito_500Medium,
  Nunito_600SemiBold,
  Nunito_700Bold,
  Nunito_800ExtraBold,
} from '@expo-google-fonts/nunito';
import {
  Poppins_400Regular,
  Poppins_500Medium,
  Poppins_600SemiBold,
  Poppins_700Bold,
  Poppins_800ExtraBold,
} from '@expo-google-fonts/poppins';
import { useSyncExternalStore } from 'react';
import type { TextStyle } from 'react-native';

export const FONT_CHOICES = ['poppins', 'inter', 'nunito', 'lora', 'system'] as const;
export type FontChoice = (typeof FONT_CHOICES)[number];
type BundledFont = Exclude<FontChoice, 'system'>;

export const appFonts = {
  Poppins_400Regular,
  Poppins_500Medium,
  Poppins_600SemiBold,
  Poppins_700Bold,
  Poppins_800ExtraBold,
  Inter_400Regular,
  Inter_500Medium,
  Inter_600SemiBold,
  Inter_700Bold,
  Inter_800ExtraBold,
  Nunito_400Regular,
  Nunito_500Medium,
  Nunito_600SemiBold,
  Nunito_700Bold,
  Nunito_800ExtraBold,
  Lora_400Regular,
  Lora_500Medium,
  Lora_600SemiBold,
  Lora_700Bold,
};

type Weight = 'regular' | 'medium' | 'semibold' | 'bold' | 'black';

// Android picks a font file by exact name, so every weight maps to its own family.
const families: Record<BundledFont, Record<Weight, string>> = {
  poppins: {
    regular: 'Poppins_400Regular',
    medium: 'Poppins_500Medium',
    semibold: 'Poppins_600SemiBold',
    bold: 'Poppins_700Bold',
    black: 'Poppins_800ExtraBold',
  },
  inter: {
    regular: 'Inter_400Regular',
    medium: 'Inter_500Medium',
    semibold: 'Inter_600SemiBold',
    bold: 'Inter_700Bold',
    black: 'Inter_800ExtraBold',
  },
  nunito: {
    regular: 'Nunito_400Regular',
    medium: 'Nunito_500Medium',
    semibold: 'Nunito_600SemiBold',
    bold: 'Nunito_700Bold',
    black: 'Nunito_800ExtraBold',
  },
  // Lora has no extra-bold cut; its bold stands in.
  lora: {
    regular: 'Lora_400Regular',
    medium: 'Lora_500Medium',
    semibold: 'Lora_600SemiBold',
    bold: 'Lora_700Bold',
    black: 'Lora_700Bold',
  },
};

const KEY = 'mb.font';
let choice: FontChoice = 'poppins';
const listeners = new Set<() => void>();

AsyncStorage.getItem(KEY)
  .then((v) => {
    if (v && (FONT_CHOICES as readonly string[]).includes(v)) setFont(v as FontChoice, false);
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

export function fontFamily(font: FontChoice, weight: Weight = 'regular'): string | undefined {
  return font === 'system' ? undefined : families[font][weight];
}

// Headlines are always set in the serif (brand), whatever the body font; the phone font opts out.
export function headlineStyle(font: FontChoice): TextStyle | undefined {
  if (font === 'system') return undefined;
  return { fontFamily: families.lora.semibold, fontWeight: 'normal' };
}

// Style to apply on top of the className; empty for the system font.
export function fontStyle(font: FontChoice, className: string): TextStyle | undefined {
  if (font === 'system') return undefined;
  return { fontFamily: families[font][weightOf(className)], fontWeight: 'normal' };
}
