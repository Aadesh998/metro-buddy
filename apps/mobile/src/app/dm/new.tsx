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
import { useT } from '@/i18n';

export default function NewDM() {
  const { t } = useT();
  const p = useLocalSearchParams<{ persona: string; alias: string; seed: string; context?: string; context_id?: string }>();
  const [intro, setIntro] = useState('');
  const [error, setError] = useState<string>();
  const send = useMutation({
    mutationFn: () => post('/v1/dm-requests', { to_persona_id: p.persona, intro: intro.trim(), context_type: p.context ?? 'profile', context_id: p.context_id }),
    onSuccess: () => {
      toast(t('chat.dm.sent', { alias: p.alias }), 'success');
      router.back();
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : t('chat.rooms.failed')),
  });
  return (
    <Screen className="gap-5">
      <ScreenHeader title={t('chat.dm.title')} />
      <View className="items-center gap-2">
        <Avatar seed={p.seed} size={64} />
        <Text variant="heading">{p.alias}</Text>
        <Text variant="caption" className="text-center">
          {t('chat.dm.hint')}
        </Text>
      </View>
      <Input value={intro} onChangeText={(v) => setIntro(v.slice(0, 200))} placeholder={t('chat.dm.placeholder')} multiline error={error} />
      <Text variant="caption">{t('chat.dm.counter', { n: intro.length })}</Text>
      <Button title={t('chat.dm.send')} disabled={intro.trim().length < 2} loading={send.isPending} onPress={() => send.mutate()} />
    </Screen>
  );
}
