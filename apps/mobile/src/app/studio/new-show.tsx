import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import { useState } from 'react';
import { KeyboardAvoidingView, Platform, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { post } from '@/api/client';
import type { Show } from '@/api/types';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Chip } from '@/components/ui/Chip';
import { Input } from '@/components/ui/Input';
import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';

const TAGS = ['work', 'relationships', 'family', 'stress', 'night', 'exams', 'loneliness', 'city life', 'money', 'funny'];

export default function NewShow() {
  const { t } = useT();
  const qc = useQueryClient();
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [schedule, setSchedule] = useState('');
  const [tags, setTags] = useState<string[]>([]);
  const create = useMutation({
    mutationFn: () => post<Show>('/v1/shows', { title: title.trim(), description: description.trim(), schedule: schedule.trim(), topic_tags: tags }),
    onSuccess: (s) => {
      qc.invalidateQueries({ queryKey: ['shows'] });
      router.replace(`/show/${s.id}`);
    },
  });
  return (
    <SafeAreaView className="flex-1 bg-bg">
      <ScreenHeader title={t('studio.newShow')} />
      <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} className="flex-1">
        <ScrollView contentContainerClassName="gap-5 p-4" keyboardShouldPersistTaps="handled">
          <Input label={t('studio.create.titleLabel')} value={title} onChangeText={(v) => setTitle(v.slice(0, 60))} placeholder={t('studio.create.titlePlaceholder')} />
          <Input
            label={t('studio.create.about')}
            value={description}
            onChangeText={(v) => setDescription(v.slice(0, 300))}
            multiline
            className="min-h-24"
            textAlignVertical="top"
          />
          <Input label={t('studio.create.schedule')} value={schedule} onChangeText={(v) => setSchedule(v.slice(0, 120))} placeholder={t('studio.create.schedulePlaceholder')} />
          <View className="gap-2">
            <Text variant="label">{t('studio.create.topics')}</Text>
            <View className="flex-row flex-wrap gap-2">
              {TAGS.map((tag) => (
                <Chip
                  key={tag}
                  label={tag}
                  selected={tags.includes(tag)}
                  onPress={() => setTags((cur) => (cur.includes(tag) ? cur.filter((x) => x !== tag) : cur.length < 5 ? [...cur, tag] : cur))}
                />
              ))}
            </View>
          </View>
          <Button title={t('studio.create.create')} disabled={title.trim().length < 3} loading={create.isPending} onPress={() => create.mutate()} />
        </ScrollView>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
