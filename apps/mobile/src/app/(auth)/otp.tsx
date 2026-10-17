import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { Pressable, TextInput, View, type TextInputInstance } from 'react-native';

import { ApiError } from '@/api/client';
import { Button } from '@/components/ui/Button';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { useAuth } from '@/lib/auth';

const LEN = 6;
const RESEND_AFTER = 30;

export default function OtpScreen() {
  const { phone } = useLocalSearchParams<{ phone: string }>();
  const { verifyOtp, requestOtp } = useAuth();
  const [code, setCode] = useState('');
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);
  const [wait, setWait] = useState(RESEND_AFTER);
  const input = useRef<TextInputInstance>(null);

  useEffect(() => {
    if (wait <= 0) return;
    const t = setTimeout(() => setWait((w) => w - 1), 1000);
    return () => clearTimeout(t);
  }, [wait]);

  async function verify(value: string) {
    setBusy(true);
    setError(undefined);
    try {
      await verifyOtp(phone, value);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Something went wrong');
      setCode('');
      if (e instanceof ApiError && (e.code === 'otp_locked' || e.code === 'otp_expired')) setWait(0);
    } finally {
      setBusy(false);
    }
  }

  async function resend() {
    setError(undefined);
    try {
      await requestOtp(phone);
      setWait(RESEND_AFTER);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Something went wrong');
    }
  }

  return (
    <Screen>
      <View className="flex-1 justify-center gap-8">
        <View className="gap-2">
          <Text variant="title">Enter the code</Text>
          <Text className="text-muted">
            Sent to +91 {phone?.slice(0, 5)} {phone?.slice(5)}.{' '}
            <Text className="text-accent" onPress={() => router.back()}>
              Change
            </Text>
          </Text>
        </View>

        <Pressable onPress={() => input.current?.focus()} className="flex-row justify-between">
          {Array.from({ length: LEN }).map((_, i) => (
            <View
              key={i}
              className={`h-14 w-12 items-center justify-center rounded-xl border bg-surface ${
                error ? 'border-danger' : i === code.length ? 'border-accent' : 'border-line'
              }`}
            >
              <Text className="text-2xl font-semibold">{code[i] ?? ''}</Text>
            </View>
          ))}
        </Pressable>
        <TextInput
          ref={input}
          value={code}
          onChangeText={(t) => {
            const v = t.replace(/\D/g, '').slice(0, LEN);
            setCode(v);
            if (v.length === LEN) verify(v);
          }}
          keyboardType="number-pad"
          autoComplete="one-time-code"
          textContentType="oneTimeCode"
          autoFocus
          maxLength={LEN}
          className="absolute h-px w-px opacity-0"
          accessibilityLabel="One-time code"
        />
        {error ? <Text className="text-sm text-danger">{error}</Text> : null}

        <Button title="Verify" onPress={() => verify(code)} disabled={code.length !== LEN} loading={busy} />
        <Pressable disabled={wait > 0} onPress={resend} className="items-center">
          <Text className={wait > 0 ? 'text-muted' : 'text-accent'}>
            {wait > 0 ? `Resend code in ${wait}s` : 'Resend code'}
          </Text>
        </Pressable>
      </View>
    </Screen>
  );
}
