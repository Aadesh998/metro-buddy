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

const TAGS = ['work', 'relationships', 'family', 'stress', 'night', 'exams', 'loneliness', 'city life', 'money', 'funny'];

export default function NewShow() {
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
      <ScreenHeader title="New show" />
      <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} className="flex-1">
        <ScrollView contentContainerClassName="gap-5 p-4" keyboardShouldPersistTaps="handled">
          <Input label="Title" value={title} onChangeText={(t) => setTitle(t.slice(0, 60))} placeholder="Last Train Home" />
          <Input
            label="What it’s about"
            value={description}
            onChangeText={(t) => setDescription(t.slice(0, 300))}
            multiline
            className="min-h-24"
            textAlignVertical="top"
          />
          <Input label="When you usually go live (optional)" value={schedule} onChangeText={(t) => setSchedule(t.slice(0, 120))} placeholder="Fridays, 10 PM" />
          <View className="gap-2">
            <Text variant="label">Topics (up to 5)</Text>
            <View className="flex-row flex-wrap gap-2">
              {TAGS.map((t) => (
                <Chip
                  key={t}
                  label={t}
                  selected={tags.includes(t)}
                  onPress={() => setTags((cur) => (cur.includes(t) ? cur.filter((x) => x !== t) : cur.length < 5 ? [...cur, t] : cur))}
                />
              ))}
            </View>
          </View>
          <Button title="Create show" disabled={title.trim().length < 3} loading={create.isPending} onPress={() => create.mutate()} />
        </ScrollView>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
