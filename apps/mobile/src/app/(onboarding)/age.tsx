import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useRef, useState } from 'react';
import { TextInput, View, type TextInputInstance } from 'react-native';

import { ApiError, put } from '@/api/client';
import { StepHeader } from '@/components/StepHeader';
import { Button } from '@/components/ui/Button';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { meKeys } from '@/features/me/hooks';
import { useT } from '@/i18n';
import { colors } from '@/lib/theme';

function isAdult(d: Date) {
  const cutoff = new Date();
  cutoff.setFullYear(cutoff.getFullYear() - 18);
  return d <= cutoff;
}

export default function AgeScreen() {
  const qc = useQueryClient();
  const { t } = useT();
  const [dd, setDd] = useState('');
  const [mm, setMm] = useState('');
  const [yyyy, setYyyy] = useState('');
  const [error, setError] = useState<string>();
  const mRef = useRef<TextInputInstance>(null);
  const yRef = useRef<TextInputInstance>(null);

  const save = useMutation({
    mutationFn: (birth_date: string) => put('/v1/me/age', { birth_date }),
    onSuccess: () => qc.invalidateQueries({ queryKey: meKeys.me }),
    onError: (e) => setError(e instanceof ApiError ? e.message : t('common.somethingWrong')),
  });

  function submit() {
    setError(undefined);
    const d = new Date(Number(yyyy), Number(mm) - 1, Number(dd));
    if (d.getDate() !== Number(dd) || d.getMonth() !== Number(mm) - 1 || yyyy.length !== 4) {
      setError(t('onboarding.age.badDate'));
      return;
    }
    if (!isAdult(d)) {
      setError(t('onboarding.age.under18'));
      return;
    }
    save.mutate(`${yyyy}-${mm.padStart(2, '0')}-${dd.padStart(2, '0')}`);
  }

  const field = 'h-14 rounded-xl border border-line bg-surface text-center text-lg text-ink';
  return (
    <Screen>
      <StepHeader step="age" title={t('onboarding.age.title')} subtitle={t('onboarding.age.subtitle')} />
      <View className="flex-row gap-3">
        <TextInput
          value={dd}
          onChangeText={(s) => {
            const v = s.replace(/\D/g, '').slice(0, 2);
            setDd(v);
            if (v.length === 2) mRef.current?.focus();
          }}
          placeholder={t('onboarding.age.dd')}
          placeholderTextColor={colors.faint}
          keyboardType="number-pad"
          className={`${field} w-20`}
          accessibilityLabel={t('onboarding.age.day')}
          autoFocus
        />
        <TextInput
          ref={mRef}
          value={mm}
          onChangeText={(s) => {
            const v = s.replace(/\D/g, '').slice(0, 2);
            setMm(v);
            if (v.length === 2) yRef.current?.focus();
          }}
          placeholder={t('onboarding.age.mm')}
          placeholderTextColor={colors.faint}
          keyboardType="number-pad"
          className={`${field} w-20`}
          accessibilityLabel={t('onboarding.age.month')}
        />
        <TextInput
          ref={yRef}
          value={yyyy}
          onChangeText={(s) => setYyyy(s.replace(/\D/g, '').slice(0, 4))}
          placeholder={t('onboarding.age.yyyy')}
          placeholderTextColor={colors.faint}
          keyboardType="number-pad"
          className={`${field} flex-1`}
          accessibilityLabel={t('onboarding.age.year')}
        />
      </View>
      {error ? <Text className="mt-3 text-sm text-danger">{error}</Text> : null}
      <View className="flex-1" />
      <Button title={t('common.continue')} onPress={submit} loading={save.isPending} disabled={!dd || !mm || yyyy.length !== 4} className="mb-4" />
    </Screen>
  );
}
