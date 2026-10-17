import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { Animated, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { Text } from './Text';

type Kind = 'error' | 'info' | 'success';
type ToastFn = (message: string, kind?: Kind) => void;

const Ctx = createContext<ToastFn>(() => {});

let external: ToastFn = () => {};

// For code outside React (query/mutation caches).
export function toast(message: string, kind: Kind = 'error') {
  external(message, kind);
}

export function useToast() {
  return useContext(Ctx);
}

const tone: Record<Kind, string> = { error: 'border-danger', info: 'border-line', success: 'border-mint' };

export function ToastProvider({ children }: { children: ReactNode }) {
  const insets = useSafeAreaInsets();
  const [msg, setMsg] = useState<{ text: string; kind: Kind } | null>(null);
  const [opacity] = useState(() => new Animated.Value(0));
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const show = useCallback<ToastFn>(
    (text, kind = 'error') => {
      setMsg({ text, kind });
      if (timer.current) clearTimeout(timer.current);
      Animated.timing(opacity, { toValue: 1, duration: 150, useNativeDriver: true }).start();
      timer.current = setTimeout(() => {
        Animated.timing(opacity, { toValue: 0, duration: 200, useNativeDriver: true }).start(() => setMsg(null));
      }, 3200);
    },
    [opacity],
  );
  useEffect(() => {
    external = show;
  }, [show]);

  return (
    <Ctx.Provider value={show}>
      {children}
      {msg ? (
        <Animated.View
          pointerEvents="none"
          style={{ opacity, position: 'absolute', left: 16, right: 16, bottom: insets.bottom + 72 }}
        >
          <View className={`rounded-xl border bg-raised px-4 py-3 ${tone[msg.kind]}`}>
            <Text accessibilityLiveRegion="polite">{msg.text}</Text>
          </View>
        </Animated.View>
      ) : null}
    </Ctx.Provider>
  );
}
