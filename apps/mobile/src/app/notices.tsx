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
import { dateLocale, useT, type Key } from '@/i18n';

type Notice = { action_id: string; action: string; target_type: string; until: string | null; created_at: string; appeal_status: string | null; can_appeal: boolean };

const LABEL: Record<string, Key> = {
  remove_content: 'safety.notices.remove_content',
  warn: 'safety.notices.warn',
  mute_24h: 'safety.notices.mute_24h',
  suspend: 'safety.notices.suspend',
  ban: 'safety.notices.ban',
  revoke_host: 'safety.notices.revoke_host',
};

function NoticeCard({ n }: { n: Notice }) {
  const qc = useQueryClient();
  const { t } = useT();
  const [text, setText] = useState('');
  const [open, setOpen] = useState(false);
  const appeal = useMutation({
    mutationFn: () => post('/v1/appeals', { action_id: n.action_id, text }),
    onSuccess: () => {
      toast(t('safety.notices.appealSent'), 'success');
      qc.invalidateQueries({ queryKey: ['notices'] });
    },
    onError: (e) => toast(e instanceof ApiError ? e.message : t('safety.notices.failed')),
  });
  return (
    <View className="gap-2 rounded-2xl border border-line bg-surface p-4">
      <Text variant="label">{LABEL[n.action] ? t(LABEL[n.action]) : n.action}</Text>
      <Text variant="caption">
        {n.target_type} · {new Date(n.created_at).toLocaleDateString(dateLocale())}
        {n.until ? t('safety.notices.until', { date: new Date(n.until).toLocaleString(dateLocale()) }) : ''}
      </Text>
      {n.appeal_status ? <Text className="text-sm text-muted">{t('safety.notices.appealStatus', { status: n.appeal_status })}</Text> : null}
      {n.can_appeal && !open ? <Button title={t('safety.notices.appeal')} kind="secondary" className="h-10 self-start px-4" onPress={() => setOpen(true)} /> : null}
      {open ? (
        <>
          <Input value={text} onChangeText={(s) => setText(s.slice(0, 1000))} multiline placeholder={t('safety.notices.appealPlaceholder')} />
          <Button title={t('safety.notices.sendAppeal')} disabled={text.trim().length < 10} loading={appeal.isPending} onPress={() => appeal.mutate()} />
        </>
      ) : null}
    </View>
  );
}

export default function NoticesScreen() {
  const { t } = useT();
  const q = useQuery({ queryKey: ['notices'], queryFn: () => get<{ items: Notice[] }>('/v1/me/moderation').then((r) => r.items) });
  return (
    <Screen scroll className="gap-3">
      <ScreenHeader title={t('safety.notices.title')} />
      {(q.data ?? []).length === 0 && !q.isPending ? <EmptyState title={t('safety.notices.emptyTitle')} body={t('safety.notices.emptyBody')} /> : null}
      {(q.data ?? []).map((n) => (
        <NoticeCard key={n.action_id} n={n} />
      ))}
    </Screen>
  );
}
