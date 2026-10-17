import { Eye, EyeOff } from 'lucide-react-native';
import { useState } from 'react';
import { ActivityIndicator, Linking, Pressable, View } from 'react-native';

import { ApiError } from '@/api/client';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { BrandMark } from '@/components/BrandMark';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { googleAvailable } from '@/features/auth/google';
import { useT } from '@/i18n';
import { useAuth } from '@/lib/auth';
import { API_URL } from '@/lib/config';
import { colors } from '@/lib/theme';

const emailRe = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const strong = (pw: string) => pw.length >= 8 && /[A-Za-z]/.test(pw) && /\d/.test(pw);

function GoogleMark() {
  return <Text className="text-lg font-bold text-[#4285F4]">G</Text>;
}

export default function SignInScreen() {
  const { signUpWithEmail, logInWithEmail, signInWithGoogle } = useAuth();
  const { t } = useT();
  const [mode, setMode] = useState<'login' | 'signup'>('signup');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [show, setShow] = useState(false);
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState<'email' | 'google' | null>(null);

  const signup = mode === 'signup';
  const mismatch = signup && confirm.length > 0 && confirm !== password;
  const valid = emailRe.test(email.trim()) && (signup ? strong(password) && confirm === password : password.length > 0);

  async function run(kind: 'email' | 'google', fn: () => Promise<unknown>) {
    setError(undefined);
    setBusy(kind);
    try {
      await fn();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t('common.somethingWrong'));
    } finally {
      setBusy(null);
    }
  }

  return (
    <Screen scroll>
      <View className="flex-1 justify-center gap-7 py-8">
        <View className="gap-3">
          <BrandMark size={56} />
          <Text variant="title">Metro Buddy</Text>
          <Text className="text-muted">{t('auth.tagline')}</Text>
        </View>

        {googleAvailable ? (
          <>
            <Pressable
              onPress={() => run('google', signInWithGoogle)}
              disabled={busy !== null}
              accessibilityRole="button"
              className="h-14 flex-row items-center justify-center gap-3 rounded-full border border-line bg-white active:opacity-80"
            >
              {busy === 'google' ? <ActivityIndicator color="#1f1f1f" /> : <GoogleMark />}
              <Text className="text-base font-semibold text-[#1f1f1f]">{t('auth.google')}</Text>
            </Pressable>
            <View className="flex-row items-center gap-3">
              <View className="h-px flex-1 bg-line" />
              <Text variant="caption">{t('auth.or')}</Text>
              <View className="h-px flex-1 bg-line" />
            </View>
          </>
        ) : null}

        <View className="gap-4">
          <Input
            label={t('auth.email')}
            value={email}
            onChangeText={(v) => setEmail(v.slice(0, 254))}
            placeholder={t('auth.emailPlaceholder')}
            keyboardType="email-address"
            autoCapitalize="none"
            autoCorrect={false}
            autoComplete="email"
            textContentType="emailAddress"
          />
          <View>
            <Input
              label={t('auth.password')}
              value={password}
              onChangeText={(v) => setPassword(v.slice(0, 128))}
              placeholder={t('auth.passwordPlaceholder')}
              secureTextEntry={!show}
              autoCapitalize="none"
              autoComplete={signup ? 'new-password' : 'current-password'}
              textContentType={signup ? 'newPassword' : 'password'}
              className="pr-12"
            />
            <Pressable
              onPress={() => setShow(!show)}
              hitSlop={10}
              accessibilityLabel={show ? t('auth.hidePassword') : t('auth.showPassword')}
              className="absolute bottom-3.5 right-4"
            >
              {show ? <EyeOff size={20} color={colors.muted} /> : <Eye size={20} color={colors.muted} />}
            </Pressable>
          </View>
          {signup ? (
            <>
              <Input
                label={t('auth.confirmPassword')}
                value={confirm}
                onChangeText={(v) => setConfirm(v.slice(0, 128))}
                secureTextEntry={!show}
                autoCapitalize="none"
                autoComplete="new-password"
                error={mismatch ? t('auth.mismatch') : undefined}
              />
              <Text variant="caption">{t('auth.passwordRule')}</Text>
            </>
          ) : null}
          {error ? <Text className="text-sm text-danger">{error}</Text> : null}
          <Button
            title={signup ? t('auth.createAccount') : t('auth.logIn')}
            disabled={!valid || busy !== null}
            loading={busy === 'email'}
            onPress={() => run('email', () => (signup ? signUpWithEmail(email.trim(), password) : logInWithEmail(email.trim(), password)))}
          />
          <Pressable
            onPress={() => {
              setMode(signup ? 'login' : 'signup');
              setError(undefined);
              setConfirm('');
            }}
            className="items-center py-1"
            hitSlop={8}
          >
            <Text className="text-sm text-muted">
              {signup ? t('auth.haveAccount') : t('auth.noAccount')}{' '}
              <Text className="text-sm font-semibold text-brand">{signup ? t('auth.switchToLogIn') : t('auth.switchToSignUp')}</Text>
            </Text>
          </Pressable>
          <Text variant="caption" className="text-center">
            {t('auth.privateNote')}
          </Text>
        </View>
      </View>
      <Pressable className="items-center pb-4" onPress={() => Linking.openURL(`${API_URL}/terms`)}>
        <Text variant="caption" className="text-center">
          {t('auth.termsBefore')}
          <Text className="text-xs text-brand">{t('auth.terms')}</Text>
          {t('auth.termsMiddle')}
          <Text className="text-xs text-brand">{t('auth.privacy')}</Text>
          {t('auth.termsAfter')}
        </Text>
      </Pressable>
    </Screen>
  );
}
