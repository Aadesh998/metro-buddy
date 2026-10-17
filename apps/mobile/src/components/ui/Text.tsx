import { Text as RNText, type TextProps } from 'react-native';

import { cn } from '@/lib/cn';
import { fontStyle, useFontChoice } from '@/lib/fonts';

type Variant = 'title' | 'heading' | 'body' | 'label' | 'caption';

const variants: Record<Variant, string> = {
  title: 'text-3xl font-bold text-ink',
  heading: 'text-lg font-semibold text-ink',
  body: 'text-base text-ink',
  label: 'text-sm font-medium text-ink',
  caption: 'text-xs text-muted',
};

export function Text({ variant = 'body', className = '', style, ...rest }: TextProps & { variant?: Variant; className?: string }) {
  const font = useFontChoice();
  const cls = cn(variants[variant], className);
  return <RNText className={cls} style={[fontStyle(font, cls), style]} {...rest} />;
}
