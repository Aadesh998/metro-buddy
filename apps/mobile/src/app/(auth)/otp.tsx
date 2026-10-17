import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { Pressable, View, type TextInputInstance } from 'react-native';

import { FontTextInput } from '@/components/ui/FontTextInput';
import { ApiError } from '@/api/client';
import { Button } from '@/components/ui/Button';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';
import { useAuth } from '@/lib/auth';

const LEN = 6;
const RESEND_AFTER = 30;

export default function OtpScreen() {
  const { phone } = useLocalSearchParams<{ phone: string }>();
  const { verifyOtp, requestOtp } = useAuth();
  const { t } = useT();
  const [code, setCode] = useState('');
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);
  const [wait, setWait] = useState(RESEND_AFTER);
  const input = useRef<TextInputInstance>(null);

  useEffect(() => {
    if (wait <= 0) return;
    const id = setTimeout(() => setWait((w) => w - 1), 1000);
    return () => clearTimeout(id);
  }, [wait]);

  async function verify(value: string) {
    setBusy(true);
    setError(undefined);
    try {
      await verifyOtp(phone, value);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t('common.somethingWrong'));
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
      setError(e instanceof ApiError ? e.message : t('common.somethingWrong'));
    }
  }

  return (
    <Screen>
      <View className="flex-1 justify-center gap-8">
        <View className="gap-2">
          <Text variant="title">{t('auth.otp.title')}</Text>
          <Text className="text-muted">
            {t('auth.otp.sentTo', { number: `${phone?.slice(0, 5)} ${phone?.slice(5)}` })}{' '}
            <Text className="text-accent" onPress={() => router.back()}>
              {t('auth.otp.change')}
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
        <FontTextInput
          ref={input}
          value={code}
          onChangeText={(s) => {
            const v = s.replace(/\D/g, '').slice(0, LEN);
            setCode(v);
            if (v.length === LEN) verify(v);
          }}
          keyboardType="number-pad"
          autoComplete="one-time-code"
          textContentType="oneTimeCode"
          autoFocus
          maxLength={LEN}
          className="absolute h-px w-px opacity-0"
          accessibilityLabel={t('auth.otp.a11y')}
        />
        {error ? <Text className="text-sm text-danger">{error}</Text> : null}

        <Button title={t('auth.otp.verify')} onPress={() => verify(code)} disabled={code.length !== LEN} loading={busy} />
        <Pressable disabled={wait > 0} onPress={resend} className="items-center">
          <Text className={wait > 0 ? 'text-muted' : 'text-accent'}>
            {wait > 0 ? t('auth.otp.resendIn', { n: wait }) : t('auth.otp.resend')}
          </Text>
        </Pressable>
      </View>
    </Screen>
  );
}
