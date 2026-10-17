import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Alert, Pressable, View } from 'react-native';

import { ApiError, patch, post, put } from '@/api/client';
import type { OwnPersona, Station } from '@/api/types';
import { Avatar } from '@/components/Avatar';
import { ScreenHeader } from '@/components/ScreenHeader';
import { LineDots, StationPicker } from '@/components/StationPicker';
import { MIN_VIBES, VibePicker } from '@/components/VibePicker';
import { Button } from '@/components/ui/Button';
import { Chip } from '@/components/ui/Chip';
import { Input } from '@/components/ui/Input';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';
import { meKeys, useMyPersona } from '@/features/me/hooks';
import { useStations } from '@/features/metro/hooks';
import { INTENTS } from '@/lib/intents';

export default function EditProfile() {
  const { data: p } = useMyPersona();
  const stations = useStations(p?.city_id);
  if (!p || !stations.data) return null;
  return <Form p={p} stations={stations.data} />;
}

function Form({ p, stations }: { p: OwnPersona; stations: Station[] }) {
  const qc = useQueryClient();
  const [bio, setBio] = useState(p.bio);
  const [intents, setIntents] = useState<string[]>(p.intents);
  const [vibes, setVibes] = useState<string[]>(p.vibes.map((v) => v.slug));
  const [home, setHome] = useState<Station | undefined>(stations.find((s) => s.id === p.home_station_id));
  const [work, setWork] = useState<Station | undefined>(stations.find((s) => s.id === p.work_station_id));
  const [picking, setPicking] = useState<'home' | 'work' | null>(null);

  const save = useMutation({
    mutationFn: async () => {
      await patch('/v1/me/persona', { bio, intents, home_station_id: home?.id, work_station_id: work?.id });
      await put('/v1/me/vibes', { slugs: vibes });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: meKeys.me });
      toast('Profile saved', 'success');
    },
  });

  const reroll = useMutation({
    mutationFn: async () => {
      const r = await post<{ suggestions: { alias: string }[] }>('/v1/me/persona/suggestions', { city_id: p.city_id });
      return r.suggestions[0].alias;
    },
    onSuccess: (alias) =>
      Alert.alert(`Become ${alias}?`, 'You can change your alias once every 30 days.', [
        { text: 'Cancel', style: 'cancel' },
        {
          text: 'Change',
          onPress: () =>
            put('/v1/me/persona/alias', { alias })
              .then(() => qc.invalidateQueries({ queryKey: meKeys.persona }))
              .catch((e) => toast(e instanceof ApiError ? e.message : 'Could not change alias')),
        },
      ]),
  });

  const canChangeAlias = new Date(p.alias_change_available_at) <= new Date();

  return (
    <Screen scroll className="gap-6">
      <ScreenHeader title="Edit profile" />
      <View className="items-center gap-2">
        <Avatar seed={p.avatar_seed} size={72} />
        <Text variant="heading">{p.alias}</Text>
        {canChangeAlias ? (
          <Button title="Get a new alias" kind="ghost" loading={reroll.isPending} onPress={() => reroll.mutate()} />
        ) : (
          <Text variant="caption">New alias available {new Date(p.alias_change_available_at).toLocaleDateString()}</Text>
        )}
      </View>

      <Input label="Bio" value={bio} onChangeText={(t) => setBio(t.slice(0, 160))} multiline />

      <View className="gap-2">
        <Text variant="label">Here for</Text>
        <View className="flex-row flex-wrap gap-2">
          {INTENTS.map((it) => (
            <Chip
              key={it.key}
              label={it.label}
              selected={intents.includes(it.key)}
              onPress={() => setIntents((xs) => (xs.includes(it.key) ? xs.filter((x) => x !== it.key) : [...xs, it.key]))}
            />
          ))}
        </View>
      </View>

      <View className="gap-2">
        <Text variant="label">Stations (private)</Text>
        {(['home', 'work'] as const).map((k) => {
          const s = k === 'home' ? home : work;
          return (
            <Pressable key={k} onPress={() => setPicking(k)} className="flex-row items-center justify-between rounded-xl border border-line bg-surface p-4">
              <View>
                <Text variant="caption">{k === 'home' ? 'Near home' : 'Near work / college'}</Text>
                <Text>{s?.name ?? 'Not set'}</Text>
              </View>
              {s ? <LineDots station={s} /> : null}
            </Pressable>
          );
        })}
      </View>

      <View className="gap-2">
        <Text variant="label">Vibes ({vibes.length})</Text>
        <VibePicker selected={vibes} onChange={setVibes} />
      </View>

      <Button title="Save" disabled={vibes.length < MIN_VIBES || intents.length === 0 || !home} loading={save.isPending} onPress={() => save.mutate()} />
      <StationPicker
        cityId={p.city_id}
        visible={picking !== null}
        title={picking === 'home' ? 'Station near home' : 'Station near work'}
        onClose={() => setPicking(null)}
        onPick={(s) => {
          if (picking === 'home') setHome(s);
          else setWork(s);
          setPicking(null);
        }}
      />
    </Screen>
  );
}
