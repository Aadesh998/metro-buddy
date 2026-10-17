import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import { useState } from 'react';
import { KeyboardAvoidingView, Platform, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { post } from '@/api/client';
import type { Bowl } from '@/api/types';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Chip } from '@/components/ui/Chip';
import { Input } from '@/components/ui/Input';
import { Text } from '@/components/ui/Text';
import { bowlKeys } from '@/features/bowls/hooks';

const KINDS = [
  { key: 'topic', label: 'Topic' },
  { key: 'office_hub', label: 'Office' },
  { key: 'college', label: 'College' },
] as const;

export default function NewBowl() {
  const qc = useQueryClient();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [rules, setRules] = useState('1. Be kind.\n2. No real names, numbers or handles.\n3. No hate or harassment.');
  const [kind, setKind] = useState<(typeof KINDS)[number]['key']>('topic');
  const [membersOnly, setMembersOnly] = useState(false);
  const create = useMutation({
    mutationFn: () =>
      post<Bowl>('/v1/bowls', { name: name.trim(), description: description.trim(), rules_md: rules.trim(), kind, visibility: membersOnly ? 'members_only' : 'public' }),
    onSuccess: (b) => {
      qc.invalidateQueries({ queryKey: bowlKeys.all });
      router.replace(`/bowl/${b.id}`);
    },
  });
  return (
    <SafeAreaView className="flex-1 bg-bg">
      <ScreenHeader title="New bowl" />
      <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} className="flex-1">
        <ScrollView contentContainerClassName="gap-5 p-4" keyboardShouldPersistTaps="handled">
          <Input label="Name" value={name} onChangeText={(t) => setName(t.slice(0, 60))} placeholder="Night Shift Nurses" />
          <View className="gap-2">
            <Text variant="label">Kind</Text>
            <View className="flex-row gap-2">
              {KINDS.map((k) => (
                <Chip key={k.key} label={k.label} selected={kind === k.key} onPress={() => setKind(k.key)} />
              ))}
            </View>
          </View>
          <Input label="What it’s for" value={description} onChangeText={(t) => setDescription(t.slice(0, 300))} multiline className="min-h-20" textAlignVertical="top" />
          <Input label="Rules" value={rules} onChangeText={(t) => setRules(t.slice(0, 2000))} multiline className="min-h-32" textAlignVertical="top" />
          <View className="flex-row gap-2">
            <Chip label="Anyone can read" selected={!membersOnly} onPress={() => setMembersOnly(false)} />
            <Chip label="Members only" selected={membersOnly} onPress={() => setMembersOnly(true)} />
          </View>
          <Text variant="caption">You’ll be the owner and can add up to 10 moderators. Platform rules still apply in every bowl.</Text>
          <Button title="Create bowl" disabled={name.trim().length < 3} loading={create.isPending} onPress={() => create.mutate()} />
        </ScrollView>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
