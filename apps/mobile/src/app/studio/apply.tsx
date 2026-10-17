import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import { useState } from 'react';
import { KeyboardAvoidingView, Platform, ScrollView } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { post } from '@/api/client';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';
import { studioKeys } from '@/features/studio/hooks';

export default function ApplyToHost() {
  const qc = useQueryClient();
  const [tagline, setTagline] = useState('');
  const [pitch, setPitch] = useState('');
  const send = useMutation({
    mutationFn: () => post('/v1/studio/host-application', { tagline: tagline.trim(), pitch: pitch.trim() }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: studioKeys.application });
      toast('Application sent. We’ll be in touch.', 'success');
      router.back();
    },
  });
  return (
    <SafeAreaView className="flex-1 bg-bg">
      <ScreenHeader title="Host a show" />
      <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} className="flex-1">
        <ScrollView contentContainerClassName="gap-5 p-4" keyboardShouldPersistTaps="handled">
          <Text className="text-muted">
            Hosts run live audio shows where listeners call in anonymously. You’ll stay anonymous too. Our team reviews every application to keep rooms
            safe.
          </Text>
          <Input label="Your show in a line" value={tagline} onChangeText={(t) => setTagline(t.slice(0, 80))} placeholder="Late-night metro confessions" />
          <Input
            label="Why you’d be a good host"
            value={pitch}
            onChangeText={(t) => setPitch(t.slice(0, 500))}
            placeholder="What would your show be about? How would you keep it kind?"
            multiline
            className="min-h-32"
            textAlignVertical="top"
          />
          <Text variant="caption">Don’t share your real name, phone or workplace. Contact details are masked.</Text>
          <Button title="Send application" disabled={tagline.trim().length < 5 || pitch.trim().length < 20} loading={send.isPending} onPress={() => send.mutate()} />
        </ScrollView>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
