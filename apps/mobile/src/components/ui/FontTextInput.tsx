import { forwardRef } from 'react';
import { TextInput, type TextInputInstance, type TextInputProps } from 'react-native';

import { fontStyle, useFontChoice } from '@/lib/fonts';

// TextInput with the app font applied, for inputs that don't use the Input field component.
export const FontTextInput = forwardRef<TextInputInstance, TextInputProps & { className?: string }>(function FontTextInput(
  { className = '', style, ...rest },
  ref,
) {
  const font = useFontChoice();
  return <TextInput ref={ref} className={className} style={[fontStyle(font, className), style]} {...rest} />;
});
