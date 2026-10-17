import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Check, MicOff, UserX } from 'lucide-react-native';
import { Pressable, View } from 'react-native';

import { post } from '@/api/client';
import type { CallIn, Episode } from '@/api/types';
import { Avatar } from '@/components/Avatar';
import { Button } from '@/components/ui/Button';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';
import { useT } from '@/i18n';
import { confirm } from '@/lib/confirm';
import { colors } from '@/lib/theme';
import { studioKeys, useQuestions, useQueue } from './hooks';

function useEpisodeAction(ep: Episode) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ path, body }: { path: string; body?: unknown }) => post(`/v1/episodes/${ep.id}/${path}`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: studioKeys.episode(ep.id) });
      qc.invalidateQueries({ queryKey: studioKeys.queue(ep.id) });
    },
  });
}

function QueueRow({ c, ep, onAir }: { c: CallIn; ep: Episode; onAir: boolean }) {
  const { t } = useT();
  const act = useEpisodeAction(ep);
  return (
    <View className="gap-2 rounded-xl bg-raised p-3">
      <View className="flex-row items-center gap-2">
        <Avatar seed={c.persona.avatar_seed} size={24} />
        <Text variant="label" className="flex-1">
          {c.persona.alias}
        </Text>
        <Pressable
          hitSlop={10}
          accessibilityLabel={t('studio.hostControls.removeLabel', { alias: c.persona.alias })}
          onPress={() =>
            confirm(t('studio.hostControls.removeTitle'), t('studio.hostControls.removeBody', { alias: c.persona.alias }), t('common.remove'), () =>
              act.mutate({ path: 'remove', body: { persona_id: c.persona.id } }),
            )
          }
        >
          <UserX size={18} color={colors.muted} />
        </Pressable>
      </View>
      <Text className="text-sm">“{c.topic}”</Text>
      <Button
        title={onAir ? t('studio.hostControls.swap') : t('studio.hostControls.bring')}
        loading={act.isPending}
        onPress={() => act.mutate({ path: 'stage', body: { call_in_id: c.id } }, { onError: (e) => toast(e.message) })}
      />
    </View>
  );
}

export function OnAirControls({ ep }: { ep: Episode }) {
  const { t } = useT();
  const act = useEpisodeAction(ep);
  if (!ep.on_air) return null;
  return (
    <View className="flex-row gap-2">
      <Button title={t('studio.hostControls.extend')} kind="secondary" className="flex-1" onPress={() => act.mutate({ path: 'extend', body: { minutes: 2 } }, { onError: (e) => toast(e.message) })} />
      <Button title={t('studio.hostControls.endCall')} kind="secondary" className="flex-1" onPress={() => act.mutate({ path: 'unstage' })} />
      <Pressable
        onPress={() => act.mutate({ path: 'mute' })}
        accessibilityLabel={t('studio.hostControls.cutMic')}
        className="h-12 w-12 items-center justify-center rounded-xl bg-danger"
      >
        <MicOff size={20} color="#fff" />
      </Pressable>
    </View>
  );
}

export function HostPanel({ ep }: { ep: Episode }) {
  const { t } = useT();
  const live = ep.status === 'live';
  const queue = useQueue(ep.id, live);
  const questions = useQuestions(ep.id, live);
  const qc = useQueryClient();
  const answered = useMutation({
    mutationFn: (id: string) => post(`/v1/questions/${id}/answered`),
    onSuccess: () => qc.invalidateQueries({ queryKey: studioKeys.questions(ep.id) }),
  });
  const waiting = (queue.data ?? []).filter((c) => c.status === 'queued');
  const open = (questions.data ?? []).filter((q) => !q.answered);
  return (
    <View className="gap-5">
      {ep.auto_muted ? (
        <View className="rounded-xl border border-danger/50 bg-danger/10 p-3">
          <Text className="text-sm text-danger">{t('studio.hostControls.autoMuted')}</Text>
        </View>
      ) : null}
      <View className="gap-2">
        <Text variant="heading">{t('studio.hostControls.queue', { n: waiting.length })}</Text>
        {waiting.length === 0 ? <Text className="text-muted">{t('studio.hostControls.noWaiting')}</Text> : null}
        {waiting.map((c) => (
          <QueueRow key={c.id} c={c} ep={ep} onAir={!!ep.on_air} />
        ))}
      </View>
      <View className="gap-2">
        <Text variant="heading">{t('studio.hostControls.questions', { n: open.length })}</Text>
        {open.length === 0 ? <Text className="text-muted">{t('studio.hostControls.noQuestions')}</Text> : null}
        {open.map((q) => (
          <View key={q.id} className="flex-row items-start gap-3 rounded-xl bg-raised p-3">
            <View className="flex-1 gap-1">
              <Text variant="caption">{q.persona.alias}</Text>
              <Text className="text-sm">{q.text}</Text>
            </View>
            <Pressable onPress={() => answered.mutate(q.id)} hitSlop={10} accessibilityLabel={t('studio.hostControls.markAnswered')}>
              <Check size={20} color={colors.mint} />
            </Pressable>
          </View>
        ))}
      </View>
    </View>
  );
}
