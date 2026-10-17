import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import { useState } from 'react';

import { post } from '@/api/client';
import type { Conversation } from '@/api/types';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { chatKeys } from '@/features/chat/hooks';
import { useT } from '@/i18n';

export default function NewGroup() {
  const { t } = useT();
  const qc = useQueryClient();
  const [title, setTitle] = useState('');
  const create = useMutation({
    mutationFn: () => post<Conversation>('/v1/groups', { title: title.trim() }),
    onSuccess: (c) => {
      qc.invalidateQueries({ queryKey: chatKeys.conversations });
      router.replace(`/chat/${c.id}/info`);
    },
  });
  return (
    <Screen className="gap-5">
      <ScreenHeader title={t('chat.groups.newTitle')} />
      <Input label={t('chat.groups.name')} value={title} onChangeText={(v) => setTitle(v.slice(0, 60))} placeholder={t('chat.groups.namePlaceholder')} autoFocus />
      <Text variant="caption">{t('chat.groups.newHint')}</Text>
      <Button title={t('chat.groups.create')} disabled={title.trim().length < 2} loading={create.isPending} onPress={() => create.mutate()} />
    </Screen>
  );
}
