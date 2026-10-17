import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { del, get, patch } from '@/api/client';
import type { PersonaRef } from '@/api/types';
import { PersonaRow } from '@/components/PersonaBadge';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';
import { bowlKeys, useBowl } from '@/features/bowls/hooks';
import { useMyPersona } from '@/features/me/hooks';
import { confirm } from '@/lib/confirm';
import { ago } from '@/lib/time';

type Mod = { persona: PersonaRef; role: 'owner' | 'moderator' };
type LogRow = { verb: string; actor: PersonaRef; meta: Record<string, unknown>; created_at: string };

const VERBS: Record<string, string> = {
  update: 'edited the bowl',
  pin: 'pinned a post',
  unpin: 'unpinned a post',
  add_mod: 'added a moderator',
  remove_mod: 'removed a moderator',
  mute: 'muted a member',
  unmute: 'unmuted a member',
  post_remove: 'removed a post',
  post_restore: 'restored a post',
  post_lock: 'closed comments',
  post_unlock: 'reopened comments',
  comment_remove: 'removed a comment',
  comment_restore: 'restored a comment',
};

function Rules({ id, initial }: { id: string; initial: string }) {
  const qc = useQueryClient();
  const [rules, setRules] = useState(initial);
  const save = useMutation({
    mutationFn: () => patch(`/v1/bowls/${id}`, { rules_md: rules }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: bowlKeys.bowl(id) });
      toast('Rules saved', 'success');
    },
  });
  return (
    <View className="gap-2">
      <Input label="Rules" value={rules} onChangeText={(t) => setRules(t.slice(0, 2000))} multiline className="min-h-28" textAlignVertical="top" />
      <Button title="Save rules" kind="secondary" disabled={rules === initial} loading={save.isPending} onPress={() => save.mutate()} />
    </View>
  );
}

export default function BowlModTools() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const qc = useQueryClient();
  const bowl = useBowl(id);
  const me = useMyPersona().data;
  const mods = useQuery({ queryKey: ['bowl-mods', id], queryFn: () => get<{ items: Mod[] }>(`/v1/bowls/${id}/moderators`).then((r) => r.items) });
  const log = useQuery({ queryKey: ['bowl-modlog', id], queryFn: () => get<{ items: LogRow[] }>(`/v1/bowls/${id}/modlog`).then((r) => r.items) });
  const removeMod = useMutation({
    mutationFn: (persona: string) => del(`/v1/bowls/${id}/moderators/${persona}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['bowl-mods', id] }),
  });
  const owner = bowl.data?.role === 'owner';
  return (
    <SafeAreaView className="flex-1 bg-bg">
      <ScreenHeader title="Mod tools" />
      <ScrollView contentContainerClassName="gap-6 p-4 pb-10">
        {bowl.data ? <Rules key={bowl.data.rules_md} id={id} initial={bowl.data.rules_md ?? ''} /> : null}
        <View className="gap-3">
          <Text variant="heading">Moderators</Text>
          {(mods.data ?? []).map((m) => (
            <View key={m.persona.id} className="flex-row items-center justify-between">
              <PersonaRow p={m.persona} subtitle={m.role === 'owner' ? 'Owner' : 'Moderator'} />
              {m.role === 'moderator' && (owner || m.persona.id === me?.id) ? (
                <Pressable
                  hitSlop={8}
                  onPress={() =>
                    confirm(m.persona.id === me?.id ? 'Step down?' : 'Remove moderator?', `${m.persona.alias} will become a regular member.`, 'Confirm', () =>
                      removeMod.mutate(m.persona.id),
                    )
                  }
                >
                  <Text className="text-sm text-danger">{m.persona.id === me?.id ? 'Step down' : 'Remove'}</Text>
                </Pressable>
              ) : null}
            </View>
          ))}
          {owner ? <Text variant="caption">To add a moderator, tap the shield on one of their posts in the bowl.</Text> : null}
        </View>
        <View className="gap-3">
          <Text variant="heading">Mod log</Text>
          {(log.data ?? []).length === 0 ? <Text className="text-muted">Nothing yet.</Text> : null}
          {(log.data ?? []).map((r, i) => (
            <View key={i} className="flex-row items-baseline gap-2">
              <Text className="flex-1 text-sm">
                <Text className="text-sm font-semibold">{r.actor.alias}</Text> {VERBS[r.verb] ?? r.verb}
                {typeof r.meta.reason === 'string' && r.meta.reason ? ` · “${r.meta.reason}”` : ''}
              </Text>
              <Text variant="caption">{ago(r.created_at)}</Text>
            </View>
          ))}
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}
