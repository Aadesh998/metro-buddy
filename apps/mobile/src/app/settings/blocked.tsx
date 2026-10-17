import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ShieldBan } from 'lucide-react-native';
import { View } from 'react-native';

import { del, get } from '@/api/client';
import type { PersonaRef } from '@/api/types';
import { EmptyState } from '@/components/EmptyState';
import { PersonaRow } from '@/components/PersonaBadge';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { colors } from '@/lib/theme';

type Rel = { persona: PersonaRef; created_at: string };

function Section({ kind }: { kind: 'blocks' | 'mutes' }) {
  const qc = useQueryClient();
  const q = useQuery({ queryKey: [kind], queryFn: () => get<{ items: Rel[] }>(`/v1/${kind}`).then((r) => r.items) });
  const undo = useMutation({
    mutationFn: (id: string) => del(`/v1/${kind}/${id}`),
    onSuccess: () => qc.invalidateQueries(),
  });
  return (
    <View className="gap-2">
      <Text variant="heading">{kind === 'blocks' ? 'Blocked' : 'Muted'}</Text>
      {(q.data ?? []).length === 0 ? (
        <Text variant="caption">Nobody here.</Text>
      ) : (
        q.data!.map((r) => (
          <View key={r.persona.id} className="flex-row items-center justify-between rounded-xl border border-line bg-surface p-3">
            <PersonaRow p={r.persona} />
            <Button title={kind === 'blocks' ? 'Unblock' : 'Unmute'} kind="secondary" className="h-9" onPress={() => undo.mutate(r.persona.id)} />
          </View>
        ))
      )}
    </View>
  );
}

export default function BlockedScreen() {
  return (
    <Screen scroll className="gap-6">
      <ScreenHeader title="Blocked & muted" />
      <Section kind="blocks" />
      <Section kind="mutes" />
      <EmptyState icon={<ShieldBan color={colors.faint} size={28} />} title="" body="Blocked people can’t see you or contact you anywhere. Muted people just disappear from your feeds." />
    </Screen>
  );
}
