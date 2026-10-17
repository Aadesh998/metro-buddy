import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Alert, Pressable, View } from 'react-native';

import { ApiError, patch, put } from '@/api/client';
import type { OwnPersona, Station } from '@/api/types';
import { Avatar } from '@/components/Avatar';
import { AvatarPicker } from '@/components/AvatarPicker';
import { AliasField, useAliasCheck } from '@/features/me/AliasField';
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
import { dateLocale, useT } from '@/i18n';
import { intents as intentOptions } from '@/lib/intents';

export default function EditProfile() {
  const { data: p } = useMyPersona();
  const stations = useStations(p?.city_id);
  if (!p || !stations.data) return null;
  return <Form p={p} stations={stations.data} />;
}

function Form({ p, stations }: { p: OwnPersona; stations: Station[] }) {
  const qc = useQueryClient();
  const { t } = useT();
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
      toast(t('settings.profile.saved'), 'success');
    },
  });

  const [newAlias, setNewAlias] = useState('');
  const [editingAlias, setEditingAlias] = useState(false);
  const aliasCheck = useAliasCheck(newAlias.trim(), p.city_id, p.alias);
  const rename = useMutation({
    mutationFn: () => put('/v1/me/persona/alias', { alias: newAlias.trim() }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: meKeys.persona });
      setEditingAlias(false);
      toast(t('settings.profile.aliasChanged'), 'success');
    },
    onError: (e) => toast(e instanceof ApiError ? e.message : t('settings.profile.aliasFailed')),
  });
  const confirmRename = () =>
    Alert.alert(t('settings.profile.becomeTitle', { alias: newAlias.trim() }), t('settings.profile.becomeBody'), [
      { text: t('common.cancel'), style: 'cancel' },
      { text: t('settings.profile.change'), onPress: () => rename.mutate() },
    ]);

  const [pickAvatar, setPickAvatar] = useState(false);
  const avatar = useMutation({
    mutationFn: (seed: string) => patch('/v1/me/persona', { avatar_seed: seed }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: meKeys.persona });
      setPickAvatar(false);
      toast(t('settings.profile.avatarSaved'), 'success');
    },
  });

  const canChangeAlias = new Date(p.alias_change_available_at) <= new Date();

  return (
    <Screen scroll className="gap-6">
      <ScreenHeader title={t('settings.profile.title')} />
      <View className="items-center gap-2">
        <Pressable onPress={() => setPickAvatar((v) => !v)} accessibilityRole="button" accessibilityLabel={t('settings.profile.avatar')}>
          <Avatar seed={p.avatar_seed} size={72} />
        </Pressable>
        <Pressable onPress={() => setPickAvatar((v) => !v)} hitSlop={8}>
          <Text className="text-sm font-semibold text-brand">{t('settings.profile.avatar')}</Text>
        </Pressable>
        <Text variant="heading">{p.alias}</Text>
        {canChangeAlias ? (
          !editingAlias ? (
            <Button
              title={t('settings.profile.newAlias')}
              kind="ghost"
              onPress={() => {
                setNewAlias(p.alias);
                setEditingAlias(true);
              }}
            />
          ) : null
        ) : (
          <Text variant="caption">{t('settings.profile.aliasAvailable', { date: new Date(p.alias_change_available_at).toLocaleDateString(dateLocale()) })}</Text>
        )}
      </View>

      {editingAlias ? (
        <View className="gap-3">
          <AliasField value={newAlias} onChange={setNewAlias} cityId={p.city_id} current={p.alias} />
          <View className="flex-row gap-2">
            <Button title={t('common.cancel')} kind="secondary" className="flex-1" onPress={() => setEditingAlias(false)} />
            <Button
              title={t('settings.profile.change')}
              className="flex-1"
              disabled={aliasCheck.status !== 'available' || newAlias.trim() === p.alias}
              loading={rename.isPending}
              onPress={confirmRename}
            />
          </View>
        </View>
      ) : null}

      {pickAvatar ? <AvatarPicker value={p.avatar_seed} onChange={(seed) => avatar.mutate(seed)} size={48} /> : null}

      <Input label={t('settings.profile.bio')} value={bio} onChangeText={(s) => setBio(s.slice(0, 160))} multiline />

      <View className="gap-2">
        <Text variant="label">{t('settings.profile.hereFor')}</Text>
        <View className="flex-row flex-wrap gap-2">
          {intentOptions().map((it) => (
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
        <Text variant="label">{t('settings.profile.stations')}</Text>
        {(['home', 'work'] as const).map((k) => {
          const s = k === 'home' ? home : work;
          return (
            <Pressable key={k} onPress={() => setPicking(k)} className="flex-row items-center justify-between rounded-xl border border-line bg-surface p-4">
              <View>
                <Text variant="caption">{k === 'home' ? t('settings.profile.nearHome') : t('settings.profile.nearWork')}</Text>
                <Text>{s?.name ?? t('settings.profile.notSet')}</Text>
              </View>
              {s ? <LineDots station={s} /> : null}
            </Pressable>
          );
        })}
      </View>

      <View className="gap-2">
        <Text variant="label">{t('settings.profile.vibes', { count: vibes.length })}</Text>
        <VibePicker selected={vibes} onChange={setVibes} />
      </View>

      <Button title={t('common.save')} disabled={vibes.length < MIN_VIBES || intents.length === 0 || !home} loading={save.isPending} onPress={() => save.mutate()} />
      <StationPicker
        cityId={p.city_id}
        visible={picking !== null}
        title={picking === 'home' ? t('onboarding.homeMetro.nearHome') : t('onboarding.homeMetro.nearWork')}
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
