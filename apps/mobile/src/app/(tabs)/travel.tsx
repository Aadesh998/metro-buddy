import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import { ArrowRight, MapPin, Plus, Settings2, ShieldCheck, TrainFront } from 'lucide-react-native';
import { useState } from 'react';
import { Pressable, RefreshControl, ScrollView, Switch, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { del, patch, post } from '@/api/client';
import type { CommutePlan, Station } from '@/api/types';
import { EmptyState } from '@/components/EmptyState';
import { StationPicker } from '@/components/StationPicker';
import { TabHeader } from '@/components/TabHeader';
import { Button } from '@/components/ui/Button';
import { Chip } from '@/components/ui/Chip';
import { Text } from '@/components/ui/Text';
import { useMyPersona } from '@/features/me/hooks';
import { useStations } from '@/features/metro/hooks';
import { needsConsent, travelKeys, useActiveIntent, useActiveTrip, useMatches, usePlans } from '@/features/travel/hooks';
import { CrewsSection } from '@/features/travel/Crews';
import { MatchCard } from '@/features/travel/MatchCard';
import { dateLocale, useT } from '@/i18n';
import { useNow } from '@/lib/clock';
import { colors } from '@/lib/theme';

function ConsentCard() {
  const { t } = useT();
  const qc = useQueryClient();
  const enable = useMutation({
    mutationFn: () => post('/v1/me/consents', { policy_version: '2026-10', items: [{ purpose: 'travel_matching', granted: true }] }),
    onSuccess: () => qc.invalidateQueries(),
  });
  return (
    <EmptyState
      icon={<TrainFront color={colors.brand} size={36} />}
      title={t('travel.consent.title')}
      body={t('travel.consent.body')}
      action={<Button title={t('travel.consent.action')} loading={enable.isPending} onPress={() => enable.mutate()} />}
    />
  );
}

function Countdown({ until }: { until: string }) {
  const now = useNow(1000);
  const s = Math.max(0, Math.round((Date.parse(until) - now) / 1000));
  return <Text className="font-mono text-2xl font-bold text-brand">{`${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`}</Text>;
}

function TravelNow() {
  const { t } = useT();
  const qc = useQueryClient();
  const persona = useMyPersona().data;
  const stations = useStations(persona?.city_id);
  const intent = useActiveIntent();
  const [from, setFrom] = useState<Station>();
  const [to, setTo] = useState<Station>();
  const [inMin, setInMin] = useState(0);
  const [mood, setMood] = useState<'chatty' | 'quiet' | 'either'>('either');
  const [picking, setPicking] = useState<'from' | 'to' | null>(null);
  const home = stations.data?.find((s) => s.id === persona?.home_station_id);
  const work = stations.data?.find((s) => s.id === persona?.work_station_id);
  const f = from ?? home;
  const dest = to ?? work;

  const start = useMutation({
    mutationFn: () => post('/v1/travel-intents', { from_station_id: f!.id, to_station_id: dest!.id, depart_in_min: inMin, mood }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: travelKeys.intent });
      qc.invalidateQueries({ queryKey: travelKeys.matches });
    },
  });
  const stop = useMutation({
    mutationFn: (id: string) => del(`/v1/travel-intents/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: travelKeys.intent });
      qc.invalidateQueries({ queryKey: travelKeys.matches });
    },
  });

  if (intent.data && intent.data.status === 'active') {
    const i = intent.data;
    return (
      <View className="gap-3 rounded-2xl border border-accent/40 bg-brand/5 p-4">
        <View className="flex-row items-center justify-between">
          <Text variant="label" className="text-brand">
            {i.from_plan ? t('travel.now.commuteLive') : t('travel.now.isOn')}
          </Text>
          <Countdown until={i.expires_at} />
        </View>
        <View className="flex-row items-center gap-2">
          <Text className="flex-1" numberOfLines={1}>
            {i.from.name}
          </Text>
          <ArrowRight size={14} color={colors.muted} />
          <Text className="flex-1 text-right" numberOfLines={1}>
            {i.to.name}
          </Text>
        </View>
        <Text variant="caption">{t('travel.now.privacy')}</Text>
        <Button title={t('travel.now.stop')} kind="secondary" loading={stop.isPending} onPress={() => stop.mutate(i.id)} />
      </View>
    );
  }
  return (
    <View className="gap-3 rounded-2xl border border-line bg-surface p-4">
      <Text variant="heading">{t('travel.now.title')}</Text>
      {(['from', 'to'] as const).map((k) => {
        const st = k === 'from' ? f : dest;
        return (
          <Pressable key={k} onPress={() => setPicking(k)} className="flex-row items-center gap-3 rounded-xl bg-raised p-3">
            <MapPin size={16} color={k === 'from' ? colors.brand : colors.sky} />
            <Text variant="caption" className="w-10">
              {k === 'from' ? t('travel.now.from') : t('travel.now.to')}
            </Text>
            <Text className={`flex-1 ${st ? 'text-ink' : 'text-faint'}`}>{st?.name ?? t('travel.chooseStation')}</Text>
          </Pressable>
        );
      })}
      <View className="flex-row flex-wrap gap-2">
        {[0, 10, 20, 30].map((m) => (
          <Chip key={m} label={m ? t('travel.now.inMin', { n: m }) : t('travel.now.leavingNow')} selected={inMin === m} onPress={() => setInMin(m)} />
        ))}
      </View>
      <View className="flex-row flex-wrap gap-2">
        {(['either', 'chatty', 'quiet'] as const).map((m) => (
          <Chip key={m} label={m === 'either' ? t('travel.now.either') : m === 'chatty' ? t('travel.now.chatty') : t('travel.now.quiet')} selected={mood === m} onPress={() => setMood(m)} />
        ))}
      </View>
      <Button title={t('travel.now.find')} disabled={!f || !dest || f.id === dest.id} loading={start.isPending} onPress={() => start.mutate()} />
      {persona ? (
        <StationPicker
          cityId={persona.city_id}
          visible={picking !== null}
          title={picking === 'from' ? t('travel.gettingOn') : t('travel.gettingOff')}
          onClose={() => setPicking(null)}
          onPick={(s) => {
            if (picking === 'from') setFrom(s);
            else setTo(s);
            setPicking(null);
          }}
        />
      ) : null}
    </View>
  );
}

function PlanRow({ p }: { p: CommutePlan }) {
  const { t, tp } = useT();
  const qc = useQueryClient();
  const toggle = useMutation({ mutationFn: (active: boolean) => patch(`/v1/commute-plans/${p.id}`, { active }), onSuccess: () => qc.invalidateQueries({ queryKey: travelKeys.plans }) });
  return (
    <Pressable onPress={() => router.push({ pathname: '/travel/plan', params: { id: p.id } })} className="flex-row items-center gap-3 rounded-2xl border border-line bg-surface p-4">
      <View className="flex-1 gap-1">
        <Text numberOfLines={1}>
          {p.from.name} → {p.to.name}
        </Text>
        <Text variant="caption">
          {p.depart_from}–{p.depart_to} · {t('common.weekdaysShort').split(',').filter((_, i) => p.weekdays.includes(i + 1)).join(' ') || '—'} · {tp('travel.tab.stops', p.stops)}
        </Text>
      </View>
      <Switch value={p.active} onValueChange={(v) => toggle.mutate(v)} trackColor={{ true: colors.brand, false: colors.line }} thumbColor={colors.surface} />
    </Pressable>
  );
}

export default function TravelTab() {
  const { t } = useT();
  const plans = usePlans();
  const matches = useMatches();
  const trip = useActiveTrip();

  if (needsConsent(plans.error)) {
    return (
      <SafeAreaView edges={['top']} className="flex-1 bg-bg">
        <TabHeader title={t('common.tabs.travel')} />
        <ConsentCard />
      </SafeAreaView>
    );
  }
  const live = (matches.data ?? []).filter((m) => m.status !== 'declined');
  return (
    <SafeAreaView edges={['top']} className="flex-1 bg-bg">
      <TabHeader
        title={t('common.tabs.travel')}
        right={
          <Pressable onPress={() => router.push('/travel/preferences')} className="h-10 w-10 items-center justify-center rounded-full bg-raised" accessibilityLabel={t('travel.tab.preferences')}>
            <Settings2 size={18} color={colors.ink} />
          </Pressable>
        }
      />
      <ScrollView
        contentContainerClassName="gap-4 px-4 pb-10"
        refreshControl={<RefreshControl refreshing={matches.isRefetching} onRefresh={() => void post('/v1/matches/refresh').catch(() => {}).then(() => matches.refetch())} tintColor={colors.brand} />}
      >
        {trip.data ? (
          <Pressable onPress={() => router.push(`/trip/${trip.data!.id}`)} className="gap-2 rounded-2xl border border-mint/50 bg-mint/10 p-4">
            <View className="flex-row items-center gap-2">
              <ShieldCheck size={18} color={colors.mint} />
              <Text variant="label" className="text-mint">
                {t('travel.tab.upcomingTrip')}
              </Text>
            </View>
            <Text>
              {trip.data.meet_station.name} · {new Date(trip.data.meet_at).toLocaleTimeString(dateLocale(), { hour: 'numeric', minute: '2-digit' })}
            </Text>
            <Text variant="caption">{t('travel.tab.tripHint')}</Text>
          </Pressable>
        ) : null}
        <TravelNow />
        <Text variant="heading" className="mt-2">
          {t('travel.tab.matches')}
        </Text>
        {live.length === 0 ? (
          <Text className="text-muted">{t('travel.tab.noMatches')}</Text>
        ) : (
          live.map((m) => <MatchCard key={m.id} m={m} />)
        )}
        <CrewsSection />
        <View className="mt-2 flex-row items-center justify-between">
          <Text variant="heading">{t('travel.tab.myCommutes')}</Text>
          <Pressable onPress={() => router.push('/travel/plan')} hitSlop={10} accessibilityLabel={t('travel.tab.addCommute')}>
            <Plus size={22} color={colors.brand} />
          </Pressable>
        </View>
        {(plans.data ?? []).map((p) => (
          <PlanRow key={p.id} p={p} />
        ))}
        {plans.data?.length === 0 ? <Text className="text-muted">{t('travel.tab.noCommutes')}</Text> : null}
      </ScrollView>
    </SafeAreaView>
  );
}
