import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import { Users } from 'lucide-react-native';
import { useState } from 'react';
import { Modal, Pressable, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { get, post } from '@/api/client';
import type { Crew, CrewInvite } from '@/api/types';
import { Avatar } from '@/components/Avatar';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';
import { useT } from '@/i18n';
import { colors } from '@/lib/theme';

export const crewKeys = { mine: ['crews'] as const, invites: ['crew-invites'] as const };

export function useCrews() {
  return useQuery({ queryKey: crewKeys.mine, queryFn: () => get<{ items: Crew[] }>('/v1/crews').then((r) => r.items), retry: false, meta: { silent: true } });
}

function useInvites() {
  return useQuery({ queryKey: crewKeys.invites, queryFn: () => get<{ items: CrewInvite[] }>('/v1/crew-invites').then((r) => r.items), retry: false, meta: { silent: true } });
}

function CrewRow({ c }: { c: Crew }) {
  const { t } = useT();
  const day = t('common.weekdaysShort').split(',');
  return (
    <Pressable onPress={() => c.conversation_id && router.push(`/chat/${c.conversation_id}`)} className="gap-2 rounded-2xl border border-line bg-surface p-4">
      <View className="flex-row items-center gap-2">
        <View className="h-3 w-3 rounded-full" style={{ backgroundColor: c.line.color }} />
        <Text variant="label" className="flex-1">
          {c.name}
        </Text>
        <Text variant="caption">
          {c.depart_at} · {c.weekdays.map((d) => day[d - 1]).join('')}
        </Text>
      </View>
      <View className="flex-row items-center">
        {c.members.map((m, i) => (
          <View key={m.id} style={{ marginLeft: i ? -8 : 0 }}>
            <Avatar seed={m.avatar_seed} size={28} />
          </View>
        ))}
        <Text variant="caption" className="ml-2">
          {c.members.length}/6{c.pending_invites ? ` · ${t('travel.crews.invited', { n: c.pending_invites })}` : ''}
        </Text>
      </View>
    </Pressable>
  );
}

export function CrewsSection() {
  const { t, tp } = useT();
  const qc = useQueryClient();
  const crews = useCrews();
  const invites = useInvites();
  const respond = useMutation({
    mutationFn: ({ id, accept }: { id: string; accept: boolean }) => post(`/v1/crew-invites/${id}/${accept ? 'accept' : 'decline'}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: crewKeys.mine });
      qc.invalidateQueries({ queryKey: crewKeys.invites });
      qc.invalidateQueries({ queryKey: ['conversations'] });
    },
    onError: (e) => toast(e.message),
  });
  const list = crews.data ?? [];
  const inv = invites.data ?? [];
  if (list.length === 0 && inv.length === 0) return null;
  return (
    <View className="gap-3">
      <Text variant="heading" className="mt-2">
        {t('travel.crews.title')}
      </Text>
      {inv.map((i) => (
        <View key={i.id} className="gap-3 rounded-2xl border border-accent/40 bg-brand/5 p-4">
          <Text>
            {t('travel.crews.invitedYou')
              .split(/(\{\w+\})/)
              .map((part, k) =>
                part === '{from}' ? (
                  <Text key={k} className="font-semibold">
                    {i.from.alias}
                  </Text>
                ) : part === '{crew}' ? (
                  <Text key={k} className="font-semibold">
                    {i.crew.name}
                  </Text>
                ) : (
                  part
                ),
              )}
          </Text>
          <Text variant="caption">
            {i.crew.line.name} · {i.crew.depart_at} · {tp('travel.crews.inCrew', i.crew.members.length)}
          </Text>
          <View className="flex-row gap-2">
            <Button title={t('common.join')} className="flex-1" onPress={() => respond.mutate({ id: i.id, accept: true })} />
            <Button title={t('travel.crews.noThanks')} kind="secondary" className="flex-1" onPress={() => respond.mutate({ id: i.id, accept: false })} />
          </View>
        </View>
      ))}
      {list.map((c) => (
        <CrewRow key={c.id} c={c} />
      ))}
    </View>
  );
}

// Offered after a trip: turn a good match into a standing crew.
export function StartCrew({ matchId }: { matchId: string }) {
  const { t } = useT();
  const qc = useQueryClient();
  const insets = useSafeAreaInsets();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState('');
  const create = useMutation({
    mutationFn: () => post<Crew>('/v1/crews', { match_id: matchId, name: name.trim() }),
    onSuccess: (c) => {
      setOpen(false);
      qc.invalidateQueries({ queryKey: crewKeys.mine });
      qc.invalidateQueries({ queryKey: ['conversations'] });
      toast(t('travel.crews.created'), 'success');
      if (c.conversation_id) router.push(`/chat/${c.conversation_id}`);
    },
    onError: (e) => toast(e.message),
  });
  return (
    <>
      <Pressable onPress={() => setOpen(true)} className="flex-row items-center gap-3 rounded-2xl border border-dashed border-line p-4">
        <Users size={20} color={colors.brand} />
        <View className="flex-1">
          <Text variant="label">{t('travel.crews.rideTogether')}</Text>
          <Text variant="caption">{t('travel.crews.rideBody')}</Text>
        </View>
      </Pressable>
      <Modal visible={open} transparent animationType="slide" onRequestClose={() => setOpen(false)}>
        <Pressable className="flex-1 bg-black/60" onPress={() => setOpen(false)} />
        <View className="gap-4 rounded-t-3xl bg-surface p-5" style={{ paddingBottom: insets.bottom + 20 }}>
          <Text variant="heading">{t('travel.crews.nameCrew')}</Text>
          <Input value={name} onChangeText={(v) => setName(v.slice(0, 40))} placeholder={t('travel.crews.namePlaceholder')} autoFocus />
          <Text variant="caption">{t('travel.crews.nameHint')}</Text>
          <Button title={t('travel.crews.create')} disabled={name.trim().length < 3} loading={create.isPending} onPress={() => create.mutate()} />
        </View>
      </Modal>
    </>
  );
}
