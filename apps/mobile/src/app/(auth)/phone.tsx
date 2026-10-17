import { router } from 'expo-router';
import { useState } from 'react';
import { Linking, Pressable, TextInput, View } from 'react-native';

import { ApiError } from '@/api/client';
import { Button } from '@/components/ui/Button';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { useAuth } from '@/lib/auth';
import { API_URL } from '@/lib/config';
import { colors } from '@/lib/theme';

export default function PhoneScreen() {
  const { requestOtp } = useAuth();
  const [digits, setDigits] = useState('');
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);
  const valid = /^[6-9]\d{9}$/.test(digits);

  async function submit() {
    setError(undefined);
    setBusy(true);
    try {
      await requestOtp(digits);
      router.push({ pathname: '/otp', params: { phone: digits } });
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Something went wrong');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen>
      <View className="flex-1 justify-center gap-8">
        <View className="gap-3">
          <View className="h-12 w-12 items-center justify-center rounded-2xl bg-accent">
            <Text className="text-xl font-black text-accent-ink">M</Text>
          </View>
          <Text variant="title">Metro Buddy</Text>
          <Text className="text-muted">
            Anonymous company for your daily commute. Vent, chat, and find a travel buddy on your line.
          </Text>
        </View>

        <View className="gap-2">
          <Text variant="label">Your mobile number</Text>
          <View className={`h-14 flex-row items-center rounded-xl border bg-surface px-4 ${error ? 'border-danger' : 'border-line'}`}>
            <Text className="mr-3 text-base text-muted">+91</Text>
            <TextInput
              value={digits}
              onChangeText={(t) => setDigits(t.replace(/\D/g, '').slice(0, 10))}
              keyboardType="phone-pad"
              autoComplete="tel-national"
              textContentType="telephoneNumber"
              placeholder="98765 43210"
              placeholderTextColor={colors.faint}
              className="flex-1 text-lg tracking-wider text-ink"
              accessibilityLabel="Mobile number"
              autoFocus
            />
          </View>
          {error ? <Text className="text-sm text-danger">{error}</Text> : null}
          <Text variant="caption">We only use your number to log you in. Nobody on Metro Buddy ever sees it.</Text>
        </View>

        <Button title="Send code" onPress={submit} disabled={!valid} loading={busy} />
      </View>
      <Pressable className="items-center pb-4" onPress={() => Linking.openURL(`${API_URL}/terms`)}>
        <Text variant="caption" className="text-center">
          18+ only. By continuing you agree to the <Text className="text-xs text-accent">Terms</Text> and{' '}
          <Text className="text-xs text-accent">Privacy Policy</Text>.
        </Text>
      </Pressable>
    </Screen>
  );
}
