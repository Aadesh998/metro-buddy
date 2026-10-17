import { cssInterop } from 'nativewind';
import { KeyboardAvoidingView, KeyboardStickyView } from 'react-native-keyboard-controller';

// Third-party views ignore `className` unless NativeWind is told to map it to `style`.
// Without this, `flex-1` on KeyboardAvoidingView is dropped and lists grow past the screen.
cssInterop(KeyboardAvoidingView, { className: 'style' });
cssInterop(KeyboardStickyView, { className: 'style' });
