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
import { useT } from '@/i18n';

const KINDS = [
  { key: 'topic', label: 'bowls.create.kindTopic' },
  { key: 'office_hub', label: 'bowls.create.kindOffice' },
  { key: 'college', label: 'bowls.create.kindCollege' },
] as const;

export default function NewBowl() {
  const { t } = useT();
  const qc = useQueryClient();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [rules, setRules] = useState(() => t('bowls.create.defaultRules'));
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
      <ScreenHeader title={t('bowls.create.title')} />
      <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} className="flex-1">
        <ScrollView contentContainerClassName="gap-5 p-4" keyboardShouldPersistTaps="handled">
          <Input label={t('bowls.create.name')} value={name} onChangeText={(v) => setName(v.slice(0, 60))} placeholder={t('bowls.create.namePlaceholder')} />
          <View className="gap-2">
            <Text variant="label">{t('bowls.create.kind')}</Text>
            <View className="flex-row gap-2">
              {KINDS.map((k) => (
                <Chip key={k.key} label={t(k.label)} selected={kind === k.key} onPress={() => setKind(k.key)} />
              ))}
            </View>
          </View>
          <Input label={t('bowls.create.purpose')} value={description} onChangeText={(v) => setDescription(v.slice(0, 300))} multiline className="min-h-20" textAlignVertical="top" />
          <Input label={t('bowls.create.rules')} value={rules} onChangeText={(v) => setRules(v.slice(0, 2000))} multiline className="min-h-32" textAlignVertical="top" />
          <View className="flex-row gap-2">
            <Chip label={t('bowls.create.public')} selected={!membersOnly} onPress={() => setMembersOnly(false)} />
            <Chip label={t('bowls.create.membersOnly')} selected={membersOnly} onPress={() => setMembersOnly(true)} />
          </View>
          <Text variant="caption">{t('bowls.create.ownerNote')}</Text>
          <Button title={t('bowls.create.submit')} disabled={name.trim().length < 3} loading={create.isPending} onPress={() => create.mutate()} />
        </ScrollView>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
