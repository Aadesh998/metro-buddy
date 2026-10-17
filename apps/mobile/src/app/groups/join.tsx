import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import { useState } from 'react';

import { ApiError, post } from '@/api/client';
import type { Conversation } from '@/api/types';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Screen } from '@/components/ui/Screen';
import { chatKeys } from '@/features/chat/hooks';
import { useT } from '@/i18n';

export default function JoinGroup() {
  const { t } = useT();
  const qc = useQueryClient();
  const [code, setCode] = useState('');
  const [error, setError] = useState<string>();
  const join = useMutation({
    mutationFn: () => post<Conversation>('/v1/groups/join', { code }),
    onSuccess: (c) => {
      qc.invalidateQueries({ queryKey: chatKeys.conversations });
      router.replace(`/chat/${c.id}`);
    },
    onError: (e) => setError(e instanceof ApiError && e.status === 404 ? t('chat.groups.noGroup') : (e as Error).message),
  });
  return (
    <Screen className="gap-5">
      <ScreenHeader title={t('chat.groups.joinTitle')} />
      <Input
        label={t('chat.groups.inviteCode')}
        value={code}
        onChangeText={(v) => setCode(v.toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 8))}
        placeholder="ABCD2345"
        autoCapitalize="characters"
        autoFocus
        error={error}
        className="text-center text-2xl tracking-[6px]"
      />
      <Button title={t('common.join')} disabled={code.length !== 8} loading={join.isPending} onPress={() => join.mutate()} />
    </Screen>
  );
}
