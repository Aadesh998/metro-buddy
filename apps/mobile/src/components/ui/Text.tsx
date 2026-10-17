import { Text as RNText, type TextProps } from 'react-native';

import { cn } from '@/lib/cn';
import { fontStyle, headlineStyle, useFontChoice } from '@/lib/fonts';

type Variant = 'title' | 'heading' | 'body' | 'label' | 'caption' | 'eyebrow';

const variants: Record<Variant, string> = {
  // Brand type scale: serif headlines at 110% leading, sans body at 140% with slight tracking,
  // and small bold caps for section labels.
  title: 'text-3xl font-semibold leading-[33px] text-ink',
  heading: 'text-lg font-semibold leading-[23px] text-ink',
  body: 'text-base leading-[22px] tracking-[0.16px] text-ink',
  label: 'text-sm font-medium leading-[20px] tracking-[0.14px] text-ink',
  caption: 'text-xs leading-[17px] tracking-[0.12px] text-muted',
  eyebrow: 'text-xs font-bold uppercase leading-[16px] tracking-[1.2px] text-muted',
};

export function Text({ variant = 'body', className = '', style, ...rest }: TextProps & { variant?: Variant; className?: string }) {
  const font = useFontChoice();
  const cls = cn(variants[variant], className);
  const face = variant === 'title' || variant === 'heading' ? headlineStyle(font) : fontStyle(font, cls);
  return <RNText className={cls} style={[face, style]} {...rest} />;
}
