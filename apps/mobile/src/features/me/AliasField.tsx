import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { View } from 'react-native';

import { ApiError, post } from '@/api/client';
import { Input } from '@/components/ui/Input';
import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';

export type AliasStatus = 'empty' | 'checking' | 'available' | 'taken' | 'invalid';

function useDebounced<T>(value: T, ms: number) {
  const [v, setV] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setV(value), ms);
    return () => clearTimeout(id);
  }, [value, ms]);
  return v;
}

// Username input that checks, as you type, whether the name is allowed and free in your city.
export function useAliasCheck(alias: string, cityId?: string, current?: string) {
  const name = useDebounced(alias, 400);
  const skip = !name || name === current;
  const q = useQuery({
    queryKey: ['alias-check', cityId, name],
    queryFn: () => post<{ available: boolean }>('/v1/me/persona/alias-check', { city_id: cityId, alias: name }),
    enabled: !skip && name.length >= 3,
    retry: false,
    staleTime: 30_000,
  });
  let status: AliasStatus = 'checking';
  let message = '';
  if (!alias) status = 'empty';
  else if (alias === current) status = 'available';
  else if (alias.length < 3) {
    status = 'invalid';
    message = '';
  } else if (name !== alias || q.isFetching) status = 'checking';
  else if (q.error) {
    status = 'invalid';
    message = q.error instanceof ApiError ? q.error.message : '';
  } else if (q.data) status = q.data.available ? 'available' : 'taken';
  return { status, message };
}

export function AliasField({
  value,
  onChange,
  cityId,
  current,
  label,
}: {
  value: string;
  onChange: (v: string) => void;
  cityId?: string;
  current?: string;
  label?: string;
}) {
  const { t } = useT();
  const { status, message } = useAliasCheck(value, cityId, current);
  const hint =
    status === 'available'
      ? { text: t('onboarding.persona.aliasAvailable'), cls: 'text-mint' }
      : status === 'taken'
        ? { text: t('onboarding.persona.aliasTaken'), cls: 'text-danger' }
        : status === 'invalid'
          ? { text: message || t('onboarding.persona.aliasRule'), cls: 'text-danger' }
          : status === 'checking'
            ? { text: t('onboarding.persona.aliasChecking'), cls: 'text-muted' }
            : { text: t('onboarding.persona.aliasRule'), cls: 'text-muted' };
  return (
    <View className="gap-1.5">
      <Input
        label={label ?? t('onboarding.persona.aliasLabel')}
        value={value}
        onChangeText={(v) => onChange(v.replace(/\s{2,}/g, ' ').slice(0, 20))}
        placeholder={t('onboarding.persona.aliasPlaceholder')}
        autoCapitalize="words"
        autoCorrect={false}
        maxLength={20}
      />
      <Text className={`text-xs ${hint.cls}`}>{hint.text}</Text>
    </View>
  );
}
