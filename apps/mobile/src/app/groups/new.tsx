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

export default function NewGroup() {
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
      <ScreenHeader title="New group" />
      <Input label="Group name" value={title} onChangeText={(t) => setTitle(t.slice(0, 60))} placeholder="Yellow Line 9:10 regulars" autoFocus />
      <Text variant="caption">You’ll get an invite code to share. Groups hold up to 50 people.</Text>
      <Button title="Create group" disabled={title.trim().length < 2} loading={create.isPending} onPress={() => create.mutate()} />
    </Screen>
  );
}
