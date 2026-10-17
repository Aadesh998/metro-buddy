import { useMutation, useQueryClient } from '@tanstack/react-query';
import * as Clipboard from 'expo-clipboard';
import { router, useLocalSearchParams } from 'expo-router';
import { Alert, Pressable, Share, Switch, View } from 'react-native';

import { del, patch, post, put } from '@/api/client';
import type { PersonaRef } from '@/api/types';
import { PersonaRow } from '@/components/PersonaBadge';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Chip } from '@/components/ui/Chip';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';
import { chatKeys, useConversation } from '@/features/chat/hooks';
import { RevealPanel } from '@/features/chat/RevealPanel';
import { useMyPersona } from '@/features/me/hooks';
import { useT } from '@/i18n';
import { useNow } from '@/lib/clock';
import { colors } from '@/lib/theme';

export default function ChatInfo() {
  const { t, tp } = useT();
  const { id } = useLocalSearchParams<{ id: string }>();
  const qc = useQueryClient();
  const me = useMyPersona().data?.id;
  const { data: c } = useConversation(id);
  const now = useNow();
  const refresh = () => qc.invalidateQueries({ queryKey: chatKeys.conversation(id) });

  const mute = useMutation({
    mutationFn: (on: boolean) => patch(`/v1/conversations/${id}`, on ? { muted_until: new Date(Date.now() + 365 * 864e5).toISOString() } : { unmute: true }),
    onSuccess: () => {
      refresh();
      qc.invalidateQueries({ queryKey: chatKeys.conversations });
    },
  });
  const rotate = useMutation({ mutationFn: () => post(`/v1/conversations/${id}/invite`), onSuccess: refresh });
  const timer = useMutation({ mutationFn: (seconds: number) => put(`/v1/conversations/${id}/disappearing`, { seconds }), onSuccess: refresh, onError: (e) => toast(e.message) });

  if (!c) return null;
  const admin = c.my_role !== 'member';
  const isGroupish = c.kind === 'group' || c.kind === 'crew' || c.kind === 'discussion';
  const muted = !!c.muted_until && Date.parse(c.muted_until) > now;

  function manage(p: PersonaRef) {
    if (!admin || p.id === me) return;
    const actions: { text: string; style?: 'destructive' | 'cancel'; onPress?: () => void }[] = [];
    if (c!.my_role === 'owner') {
      actions.push({ text: t('chat.info.makeAdmin'), onPress: () => patch(`/v1/conversations/${id}/members/${p.id}`, { role: 'admin' }).then(refresh) });
      actions.push({ text: t('chat.info.makeMember'), onPress: () => patch(`/v1/conversations/${id}/members/${p.id}`, { role: 'member' }).then(refresh) });
    }
    actions.push({
      text: t('chat.info.removeFromGroup'),
      style: 'destructive',
      onPress: () =>
        del(`/v1/conversations/${id}/members/${p.id}`)
          .then(refresh)
          .catch((e) => toast(e.message)),
    });
    actions.push({ text: t('common.cancel'), style: 'cancel' });
    Alert.alert(p.alias, undefined, actions);
  }

  function leave() {
    Alert.alert(t('chat.info.leaveTitle'), t('chat.info.leaveBody'), [
      { text: t('common.cancel'), style: 'cancel' },
      {
        text: t('common.leave'),
        style: 'destructive',
        onPress: async () => {
          await post(`/v1/conversations/${id}/leave`);
          qc.invalidateQueries({ queryKey: chatKeys.conversations });
          router.dismissAll();
          router.replace('/chats');
        },
      },
    ]);
  }

  return (
    <Screen scroll className="gap-5">
      <ScreenHeader title={c.title ?? t('chat.chatInfo')} />
      {c.topic ? <Text className="text-muted">{c.topic}</Text> : null}

      {admin && c.invite_code ? (
        <View className="gap-3 rounded-2xl border border-line bg-surface p-4">
          <Text variant="label">{t('chat.info.inviteCode')}</Text>
          <Text className="text-2xl font-bold tracking-[6px] text-accent">{c.invite_code}</Text>
          <View className="flex-row gap-2">
            <Button title={t('common.copy')} kind="secondary" className="h-10 flex-1" onPress={() => Clipboard.setStringAsync(c.invite_code!).then(() => toast(t('common.copied'), 'success'))} />
            <Button title={t('common.share')} kind="secondary" className="h-10 flex-1" onPress={() => Share.share({ message: t('chat.info.shareMessage', { code: c.invite_code! }) })} />
            <Button title={t('chat.info.newCode')} kind="ghost" className="h-10" loading={rotate.isPending} onPress={() => rotate.mutate()} />
          </View>
          <Text variant="caption">{t('chat.info.inviteHint')}</Text>
        </View>
      ) : null}

      {c.kind === 'dm' || c.kind === 'travel' ? (
        <RevealPanel conversationId={id} otherAlias={c.members.find((m) => m.id !== me)?.alias ?? t('chat.info.they')} />
      ) : null}

      <View className="flex-row items-center justify-between rounded-2xl border border-line bg-surface p-4">
        <Text>{t('chat.info.muteNotifications')}</Text>
        <Switch value={muted} onValueChange={(v) => mute.mutate(v)} trackColor={{ true: colors.accent, false: colors.line }} thumbColor={colors.ink} />
      </View>

      {c.kind !== 'discussion' && (c.kind !== 'group' || admin) ? (
        <View className="gap-3 rounded-2xl border border-line bg-surface p-4">
          <Text>{t('chat.info.disappearing')}</Text>
          <View className="flex-row flex-wrap gap-2">
            {[
              [0, t('chat.info.off')],
              [3600, t('chat.info.oneHour')],
              [86400, t('chat.info.oneDay')],
              [604800, t('chat.info.sevenDays')],
            ].map(([sec, label]) => (
              <Chip key={sec} label={String(label)} selected={(c.disappear_after_sec ?? 0) === sec} onPress={() => timer.mutate(Number(sec))} />
            ))}
          </View>
          <Text variant="caption">{t('chat.info.disappearingHint')}</Text>
        </View>
      ) : null}

      <View className="gap-2">
        <Text variant="label">{tp('chat.members', c.member_count)}</Text>
        {c.members.map((m) => (
          <Pressable key={m.id} onPress={() => (admin && isGroupish ? manage(m) : m.id !== me && router.push(`/u/${m.id}`))} className="rounded-xl bg-surface p-3 active:bg-raised">
            <PersonaRow p={m} subtitle={m.id === me ? t('common.you') : undefined} />
          </Pressable>
        ))}
      </View>

      <Button title={c.kind === 'dm' || c.kind === 'travel' ? t('chat.info.leaveChat') : t('chat.info.leaveGroup')} kind="danger" onPress={leave} />
    </Screen>
  );
}
