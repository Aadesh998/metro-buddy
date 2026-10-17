import { Image } from 'expo-image';

import { useScheme } from '@/lib/theme';

const ink = require('../../assets/brand/mark-ink.png');
const paper = require('../../assets/brand/mark-paper.png');

// The logo mark, in ink on light screens and paper on dark ones.
export function BrandMark({ size = 48 }: { size?: number }) {
  const scheme = useScheme();
  return <Image source={scheme === 'dark' ? paper : ink} style={{ width: size, height: size }} contentFit="contain" accessibilityLabel="Metro Buddy" />;
}
