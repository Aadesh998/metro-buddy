import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { Alert, Pressable, ScrollView, View } from 'react-native';

import { ApiError, del, patch, post } from '@/api/client';
import type { CommutePlan, Station } from '@/api/types';
import { ScreenHeader } from '@/components/ScreenHeader';
import { StationPicker } from '@/components/StationPicker';
import { Button } from '@/components/ui/Button';
import { Chip } from '@/components/ui/Chip';
import { Input } from '@/components/ui/Input';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { useMyPersona } from '@/features/me/hooks';
import { useStations } from '@/features/metro/hooks';
import { travelKeys, usePlans } from '@/features/travel/hooks';
import { useT } from '@/i18n';

const TIME = /^([01]\d|2[0-3]):[0-5]\d$/;

function Form({ plan, cityId, stations }: { plan?: CommutePlan; cityId: string; stations: Station[] }) {
  const { t } = useT();
  const qc = useQueryClient();
  const persona = useMyPersona().data;
  const [from, setFrom] = useState<Station | undefined>(stations.find((s) => s.id === (plan?.from.id ?? persona?.home_station_id)));
  const [to, setTo] = useState<Station | undefined>(stations.find((s) => s.id === (plan?.to.id ?? persona?.work_station_id)));
  const [start, setStart] = useState(plan?.depart_from ?? '08:30');
  const [end, setEnd] = useState(plan?.depart_to ?? '09:15');
  const [days, setDays] = useState<number[]>(plan?.weekdays ?? [1, 2, 3, 4, 5]);
  const [picking, setPicking] = useState<'from' | 'to' | null>(null);
  const [error, setError] = useState<string>();

  const save = useMutation({
    mutationFn: () =>
      plan
        ? patch(`/v1/commute-plans/${plan.id}`, { depart_from: start, depart_to: end, weekdays: days })
        : post('/v1/commute-plans', { from_station_id: from!.id, to_station_id: to!.id, depart_from: start, depart_to: end, weekdays: days }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: travelKeys.plans });
      router.back();
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : t('travel.plan.failed')),
  });

  const valid = from && to && from.id !== to.id && TIME.test(start) && TIME.test(end) && start < end && days.length > 0;
  return (
    <ScrollView contentContainerClassName="gap-5 px-5 pb-10" keyboardShouldPersistTaps="handled">
      {(['from', 'to'] as const).map((k) => (
        <Pressable key={k} disabled={!!plan} onPress={() => setPicking(k)} className="rounded-xl border border-line bg-surface p-4">
          <Text variant="caption">{k === 'from' ? t('travel.gettingOn') : t('travel.gettingOff')}</Text>
          <Text>{(k === 'from' ? from : to)?.name ?? t('travel.chooseStation')}</Text>
        </Pressable>
      ))}
      <View className="flex-row gap-3">
        <View className="flex-1">
          <Input label={t('travel.plan.leaveBetween')} value={start} onChangeText={setStart} placeholder="08:30" keyboardType="numbers-and-punctuation" maxLength={5} />
        </View>
        <View className="flex-1">
          <Input label={t('travel.plan.and')} value={end} onChangeText={setEnd} placeholder="09:15" keyboardType="numbers-and-punctuation" maxLength={5} />
        </View>
      </View>
      <View className="flex-row flex-wrap gap-2">
        {t('travel.plan.days').split(',').map((d, i) => (
          <Chip key={d} label={d} selected={days.includes(i + 1)} onPress={() => setDays(days.includes(i + 1) ? days.filter((x) => x !== i + 1) : [...days, i + 1])} />
        ))}
      </View>
      <Text variant="caption">{t('travel.plan.hint')}</Text>
      {error ? <Text className="text-danger">{error}</Text> : null}
      <Button title={t('travel.plan.save')} disabled={!valid} loading={save.isPending} onPress={() => save.mutate()} />
      {plan ? (
        <Button
          title={t('travel.plan.delete')}
          kind="ghost"
          onPress={() =>
            Alert.alert(t('travel.plan.deleteTitle'), undefined, [
              { text: t('common.cancel'), style: 'cancel' },
              {
                text: t('common.delete'),
                style: 'destructive',
                onPress: () =>
                  del(`/v1/commute-plans/${plan.id}`).then(() => {
                    qc.invalidateQueries({ queryKey: travelKeys.plans });
                    router.back();
                  }),
              },
            ])
          }
        />
      ) : null}
      <StationPicker
        cityId={cityId}
        visible={picking !== null}
        title={picking === 'from' ? t('travel.gettingOn') : t('travel.gettingOff')}
        onClose={() => setPicking(null)}
        onPick={(s) => {
          if (picking === 'from') setFrom(s);
          else setTo(s);
          setPicking(null);
        }}
      />
    </ScrollView>
  );
}

export default function PlanScreen() {
  const { t } = useT();
  const { id } = useLocalSearchParams<{ id?: string }>();
  const persona = useMyPersona().data;
  const stations = useStations(persona?.city_id);
  const plans = usePlans();
  const plan = plans.data?.find((p) => p.id === id);
  return (
    <Screen edges={['top', 'bottom']} className="px-0">
      <ScreenHeader title={plan ? t('travel.plan.editTitle') : t('travel.plan.newTitle')} />
      {persona && stations.data && (!id || plan) ? <Form plan={plan} cityId={persona.city_id} stations={stations.data} /> : null}
    </Screen>
  );
}
