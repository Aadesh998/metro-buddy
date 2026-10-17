import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Trash2 } from 'lucide-react-native';
import { useState } from 'react';
import { Pressable, View } from 'react-native';

import { ApiError, del, get, post } from '@/api/client';
import type { TrustedContact } from '@/api/types';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { colors } from '@/lib/theme';

export default function Contacts() {
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ['contacts'], queryFn: () => get<{ items: TrustedContact[] }>('/v1/me/trusted-contacts').then((r) => r.items) });
  const [name, setName] = useState('');
  const [phone, setPhone] = useState('');
  const [error, setError] = useState<string>();
  const add = useMutation({
    mutationFn: () => post<{ items: TrustedContact[] }>('/v1/me/trusted-contacts', { name: name.trim(), phone }),
    onSuccess: (r) => {
      qc.setQueryData(['contacts'], r.items);
      setName('');
      setPhone('');
      setError(undefined);
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : 'Failed'),
  });
  const remove = useMutation({ mutationFn: (id: string) => del(`/v1/me/trusted-contacts/${id}`), onSuccess: () => qc.invalidateQueries({ queryKey: ['contacts'] }) });
  const list = q.data ?? [];
  return (
    <Screen scroll className="gap-4">
      <ScreenHeader title="Trusted contacts" />
      <Text className="text-muted">When you press SOS, these people get a text with a live link showing which station you last passed. They never see your chats.</Text>
      {list.map((c) => (
        <View key={c.id} className="flex-row items-center justify-between rounded-xl border border-line bg-surface p-4">
          <View>
            <Text variant="label">{c.name}</Text>
            <Text variant="caption">{c.phone_masked}</Text>
          </View>
          <Pressable onPress={() => remove.mutate(c.id)} hitSlop={10} accessibilityLabel={`Remove ${c.name}`}>
            <Trash2 size={18} color={colors.muted} />
          </Pressable>
        </View>
      ))}
      {list.length < 5 ? (
        <View className="gap-3 rounded-2xl border border-line p-4">
          <Input label="Name" value={name} onChangeText={(t) => setName(t.slice(0, 60))} placeholder="Didi" />
          <Input label="Mobile" value={phone} onChangeText={(t) => setPhone(t.replace(/[^\d+ ]/g, '').slice(0, 16))} keyboardType="phone-pad" placeholder="98111 22233" error={error} />
          <Button title="Add contact" disabled={!name.trim() || phone.replace(/\D/g, '').length < 10} loading={add.isPending} onPress={() => add.mutate()} />
        </View>
      ) : null}
    </Screen>
  );
}
