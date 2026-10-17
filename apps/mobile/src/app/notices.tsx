import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { View } from 'react-native';

import { ApiError, get, post } from '@/api/client';
import { EmptyState } from '@/components/EmptyState';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';

type Notice = { action_id: string; action: string; target_type: string; until: string | null; created_at: string; appeal_status: string | null; can_appeal: boolean };

const LABEL: Record<string, string> = {
  remove_content: 'Your content was removed',
  warn: 'You received a warning',
  mute_24h: 'You were muted for 24 hours',
  suspend: 'Your account was suspended',
  ban: 'Your account was banned',
  revoke_host: 'Your host access was removed',
};

function NoticeCard({ n }: { n: Notice }) {
  const qc = useQueryClient();
  const [text, setText] = useState('');
  const [open, setOpen] = useState(false);
  const appeal = useMutation({
    mutationFn: () => post('/v1/appeals', { action_id: n.action_id, text }),
    onSuccess: () => {
      toast('Appeal sent. A different person on our team will review it.', 'success');
      qc.invalidateQueries({ queryKey: ['notices'] });
    },
    onError: (e) => toast(e instanceof ApiError ? e.message : 'Failed'),
  });
  return (
    <View className="gap-2 rounded-2xl border border-line bg-surface p-4">
      <Text variant="label">{LABEL[n.action] ?? n.action}</Text>
      <Text variant="caption">
        {n.target_type} · {new Date(n.created_at).toLocaleDateString()}
        {n.until ? ` · until ${new Date(n.until).toLocaleString()}` : ''}
      </Text>
      {n.appeal_status ? <Text className="text-sm text-muted">Appeal: {n.appeal_status}</Text> : null}
      {n.can_appeal && !open ? <Button title="Appeal this decision" kind="secondary" className="h-10 self-start px-4" onPress={() => setOpen(true)} /> : null}
      {open ? (
        <>
          <Input value={text} onChangeText={(t) => setText(t.slice(0, 1000))} multiline placeholder="Tell us why you think this was a mistake" />
          <Button title="Send appeal" disabled={text.trim().length < 10} loading={appeal.isPending} onPress={() => appeal.mutate()} />
        </>
      ) : null}
    </View>
  );
}

export default function NoticesScreen() {
  const q = useQuery({ queryKey: ['notices'], queryFn: () => get<{ items: Notice[] }>('/v1/me/moderation').then((r) => r.items) });
  return (
    <Screen scroll className="gap-3">
      <ScreenHeader title="Account notices" />
      {(q.data ?? []).length === 0 && !q.isPending ? <EmptyState title="No notices" body="You’re in good standing." /> : null}
      {(q.data ?? []).map((n) => (
        <NoticeCard key={n.action_id} n={n} />
      ))}
    </Screen>
  );
}
