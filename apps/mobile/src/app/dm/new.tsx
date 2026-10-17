import { useMutation } from '@tanstack/react-query';
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { View } from 'react-native';

import { ApiError, post } from '@/api/client';
import { Avatar } from '@/components/Avatar';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';

export default function NewDM() {
  const p = useLocalSearchParams<{ persona: string; alias: string; seed: string; context?: string; context_id?: string }>();
  const [intro, setIntro] = useState('');
  const [error, setError] = useState<string>();
  const send = useMutation({
    mutationFn: () => post('/v1/dm-requests', { to_persona_id: p.persona, intro: intro.trim(), context_type: p.context ?? 'profile', context_id: p.context_id }),
    onSuccess: () => {
      toast(`Request sent to ${p.alias}`, 'success');
      router.back();
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : 'Failed'),
  });
  return (
    <Screen className="gap-5">
      <ScreenHeader title="Message request" />
      <View className="items-center gap-2">
        <Avatar seed={p.seed} size={64} />
        <Text variant="heading">{p.alias}</Text>
        <Text variant="caption" className="text-center">
          They’ll see your intro and can accept or ignore. You can’t message them until they accept.
        </Text>
      </View>
      <Input value={intro} onChangeText={(t) => setIntro(t.slice(0, 200))} placeholder="Hey! Saw your post about the Blue Line…" multiline error={error} />
      <Text variant="caption">{intro.length}/200 · contact details are hidden automatically</Text>
      <Button title="Send request" disabled={intro.trim().length < 2} loading={send.isPending} onPress={() => send.mutate()} />
    </Screen>
  );
}
