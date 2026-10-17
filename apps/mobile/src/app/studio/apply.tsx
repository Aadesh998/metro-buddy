import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import { useState } from 'react';
import { ScrollView } from 'react-native';
import { KeyboardAvoidingView } from 'react-native-keyboard-controller';
import { SafeAreaView } from 'react-native-safe-area-context';

import { post } from '@/api/client';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';
import { studioKeys } from '@/features/studio/hooks';
import { useT } from '@/i18n';

export default function ApplyToHost() {
  const { t } = useT();
  const qc = useQueryClient();
  const [tagline, setTagline] = useState('');
  const [pitch, setPitch] = useState('');
  const send = useMutation({
    mutationFn: () => post('/v1/studio/host-application', { tagline: tagline.trim(), pitch: pitch.trim() }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: studioKeys.application });
      toast(t('studio.apply.sent'), 'success');
      router.back();
    },
  });
  return (
    <SafeAreaView className="flex-1 bg-bg">
      <ScreenHeader title={t('studio.apply.title')} />
      <KeyboardAvoidingView behavior="padding" className="flex-1">
        <ScrollView contentContainerClassName="gap-5 p-4" keyboardShouldPersistTaps="handled">
          <Text className="text-muted">{t('studio.apply.intro')}</Text>
          <Input label={t('studio.apply.tagline')} value={tagline} onChangeText={(v) => setTagline(v.slice(0, 80))} placeholder={t('studio.apply.taglinePlaceholder')} />
          <Input
            label={t('studio.apply.pitch')}
            value={pitch}
            onChangeText={(v) => setPitch(v.slice(0, 500))}
            placeholder={t('studio.apply.pitchPlaceholder')}
            multiline
            className="min-h-32"
            textAlignVertical="top"
          />
          <Text variant="caption">{t('studio.apply.warning')}</Text>
          <Button title={t('studio.apply.send')} disabled={tagline.trim().length < 5 || pitch.trim().length < 20} loading={send.isPending} onPress={() => send.mutate()} />
        </ScrollView>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
