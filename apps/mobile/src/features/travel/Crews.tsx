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
import { colors } from '@/lib/theme';

const DAY = ['M', 'T', 'W', 'T', 'F', 'S', 'S'];
export const crewKeys = { mine: ['crews'] as const, invites: ['crew-invites'] as const };

export function useCrews() {
  return useQuery({ queryKey: crewKeys.mine, queryFn: () => get<{ items: Crew[] }>('/v1/crews').then((r) => r.items), retry: false, meta: { silent: true } });
}

function useInvites() {
  return useQuery({ queryKey: crewKeys.invites, queryFn: () => get<{ items: CrewInvite[] }>('/v1/crew-invites').then((r) => r.items), retry: false, meta: { silent: true } });
}

function CrewRow({ c }: { c: Crew }) {
  return (
    <Pressable onPress={() => c.conversation_id && router.push(`/chat/${c.conversation_id}`)} className="gap-2 rounded-2xl border border-line bg-surface p-4">
      <View className="flex-row items-center gap-2">
        <View className="h-3 w-3 rounded-full" style={{ backgroundColor: c.line.color }} />
        <Text variant="label" className="flex-1">
          {c.name}
        </Text>
        <Text variant="caption">
          {c.depart_at} · {c.weekdays.map((d) => DAY[d - 1]).join('')}
        </Text>
      </View>
      <View className="flex-row items-center">
        {c.members.map((m, i) => (
          <View key={m.id} style={{ marginLeft: i ? -8 : 0 }}>
            <Avatar seed={m.avatar_seed} size={28} />
          </View>
        ))}
        <Text variant="caption" className="ml-2">
          {c.members.length}/6{c.pending_invites ? ` · ${c.pending_invites} invited` : ''}
        </Text>
      </View>
    </Pressable>
  );
}

export function CrewsSection() {
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
        Crews
      </Text>
      {inv.map((i) => (
        <View key={i.id} className="gap-3 rounded-2xl border border-accent/40 bg-accent/5 p-4">
          <Text>
            <Text className="font-semibold">{i.from.alias}</Text> invited you to <Text className="font-semibold">{i.crew.name}</Text>
          </Text>
          <Text variant="caption">
            {i.crew.line.name} · {i.crew.depart_at} · {i.crew.members.length} in the crew
          </Text>
          <View className="flex-row gap-2">
            <Button title="Join" className="flex-1" onPress={() => respond.mutate({ id: i.id, accept: true })} />
            <Button title="No thanks" kind="secondary" className="flex-1" onPress={() => respond.mutate({ id: i.id, accept: false })} />
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
      toast('Crew created. We sent them an invite.', 'success');
      if (c.conversation_id) router.push(`/chat/${c.conversation_id}`);
    },
    onError: (e) => toast(e.message),
  });
  return (
    <>
      <Pressable onPress={() => setOpen(true)} className="flex-row items-center gap-3 rounded-2xl border border-dashed border-line p-4">
        <Users size={20} color={colors.accent} />
        <View className="flex-1">
          <Text variant="label">Ride together regularly?</Text>
          <Text variant="caption">Start a commute crew with a shared plan and group chat (up to 6).</Text>
        </View>
      </Pressable>
      <Modal visible={open} transparent animationType="slide" onRequestClose={() => setOpen(false)}>
        <Pressable className="flex-1 bg-black/60" onPress={() => setOpen(false)} />
        <View className="gap-4 rounded-t-3xl bg-surface p-5" style={{ paddingBottom: insets.bottom + 20 }}>
          <Text variant="heading">Name your crew</Text>
          <Input value={name} onChangeText={(t) => setName(t.slice(0, 40))} placeholder="Yellow Line 9:10" autoFocus />
          <Text variant="caption">They’ll get an invite and can say no. You can invite others you’ve matched with later.</Text>
          <Button title="Create crew" disabled={name.trim().length < 3} loading={create.isPending} onPress={() => create.mutate()} />
        </View>
      </Modal>
    </>
  );
}
