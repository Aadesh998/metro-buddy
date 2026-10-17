import { useMutation, useQueryClient } from '@tanstack/react-query';
import { MapPin, Briefcase } from 'lucide-react-native';
import { useState } from 'react';
import { Pressable, View } from 'react-native';

import { patch } from '@/api/client';
import type { Station } from '@/api/types';
import { LineDots, StationPicker } from '@/components/StationPicker';
import { StepHeader } from '@/components/StepHeader';
import { Button } from '@/components/ui/Button';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { meKeys, useMyPersona } from '@/features/me/hooks';
import { colors } from '@/lib/theme';

function StationField({ label, icon, station, onPress }: { label: string; icon: React.ReactNode; station?: Station; onPress: () => void }) {
  return (
    <Pressable onPress={onPress} className="flex-row items-center gap-4 rounded-2xl border border-line bg-surface p-4 active:bg-raised">
      <View className="h-10 w-10 items-center justify-center rounded-full bg-raised">{icon}</View>
      <View className="flex-1 gap-1">
        <Text variant="caption">{label}</Text>
        <Text className={station ? 'text-ink' : 'text-faint'}>{station?.name ?? 'Choose a station'}</Text>
      </View>
      {station ? <LineDots station={station} /> : null}
    </Pressable>
  );
}

export default function HomeMetroScreen() {
  const qc = useQueryClient();
  const { data: persona } = useMyPersona();
  const [home, setHome] = useState<Station>();
  const [work, setWork] = useState<Station>();
  const [picking, setPicking] = useState<'home' | 'work' | null>(null);

  const save = useMutation({
    mutationFn: () => patch('/v1/me/persona', { home_station_id: home?.id, work_station_id: work?.id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: meKeys.me }),
  });

  if (!persona) return null;
  return (
    <Screen>
      <StepHeader
        step="home-metro"
        title="Your everyday metro"
        subtitle="We use these to suggest bowls and travel buddies. Nobody ever sees your home or work station."
      />
      <View className="gap-3">
        <StationField label="Station near home" icon={<MapPin color={colors.accent} size={20} />} station={home} onPress={() => setPicking('home')} />
        <StationField label="Station near work or college (optional)" icon={<Briefcase color={colors.sky} size={20} />} station={work} onPress={() => setPicking('work')} />
      </View>
      <View className="flex-1" />
      <Button title="Continue" disabled={!home} loading={save.isPending} onPress={() => save.mutate()} className="mb-4" />
      <StationPicker
        cityId={persona.city_id}
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
