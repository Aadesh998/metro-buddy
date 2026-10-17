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
import { useNow } from '@/lib/clock';
import { colors } from '@/lib/theme';

const DAYS = ['M', 'T', 'W', 'T', 'F', 'S', 'S'];

function ConsentCard() {
  const qc = useQueryClient();
  const enable = useMutation({
    mutationFn: () => post('/v1/me/consents', { policy_version: '2026-10', items: [{ purpose: 'travel_matching', granted: true }] }),
    onSuccess: () => qc.invalidateQueries(),
  });
  return (
    <EmptyState
      icon={<TrainFront color={colors.accent} size={36} />}
      title="Find a travel buddy"
      body="Turn on Travel Buddy matching to use your commute stations and times for suggestions. Only the stretch you share is ever shown to anyone."
      action={<Button title="Turn on Travel Buddy" loading={enable.isPending} onPress={() => enable.mutate()} />}
    />
  );
}

function Countdown({ until }: { until: string }) {
  const now = useNow(1000);
  const s = Math.max(0, Math.round((Date.parse(until) - now) / 1000));
  return <Text className="font-mono text-2xl font-bold text-accent">{`${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`}</Text>;
}

function TravelNow() {
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
  const t = to ?? work;

  const start = useMutation({
    mutationFn: () => post('/v1/travel-intents', { from_station_id: f!.id, to_station_id: t!.id, depart_in_min: inMin, mood }),
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
      <View className="gap-3 rounded-2xl border border-accent/40 bg-accent/5 p-4">
        <View className="flex-row items-center justify-between">
          <Text variant="label" className="text-accent">
            {i.from_plan ? 'Your commute is live' : 'Travel Now is on'}
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
        <Text variant="caption">Only people on the same stretch, at about the same time, will see you. Your stations stay private.</Text>
        <Button title="Stop" kind="secondary" loading={stop.isPending} onPress={() => stop.mutate(i.id)} />
      </View>
    );
  }
  return (
    <View className="gap-3 rounded-2xl border border-line bg-surface p-4">
      <Text variant="heading">Travel Now</Text>
      {(['from', 'to'] as const).map((k) => {
        const st = k === 'from' ? f : t;
        return (
          <Pressable key={k} onPress={() => setPicking(k)} className="flex-row items-center gap-3 rounded-xl bg-raised p-3">
            <MapPin size={16} color={k === 'from' ? colors.accent : colors.sky} />
            <Text variant="caption" className="w-10">
              {k}
            </Text>
            <Text className={`flex-1 ${st ? 'text-ink' : 'text-faint'}`}>{st?.name ?? 'Choose station'}</Text>
          </Pressable>
        );
      })}
      <View className="flex-row flex-wrap gap-2">
        {[0, 10, 20, 30].map((m) => (
          <Chip key={m} label={m ? `in ${m} min` : 'Leaving now'} selected={inMin === m} onPress={() => setInMin(m)} />
        ))}
      </View>
      <View className="flex-row flex-wrap gap-2">
        {(['either', 'chatty', 'quiet'] as const).map((m) => (
          <Chip key={m} label={m === 'either' ? 'Either is fine' : m === 'chatty' ? 'Feeling chatty' : 'Quiet company'} selected={mood === m} onPress={() => setMood(m)} />
        ))}
      </View>
      <Button title="Find a buddy for 45 min" disabled={!f || !t || f.id === t.id} loading={start.isPending} onPress={() => start.mutate()} />
      {persona ? (
        <StationPicker
          cityId={persona.city_id}
          visible={picking !== null}
          title={picking === 'from' ? 'Getting on at' : 'Getting off at'}
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
  const qc = useQueryClient();
  const toggle = useMutation({ mutationFn: (active: boolean) => patch(`/v1/commute-plans/${p.id}`, { active }), onSuccess: () => qc.invalidateQueries({ queryKey: travelKeys.plans }) });
  return (
    <Pressable onPress={() => router.push({ pathname: '/travel/plan', params: { id: p.id } })} className="flex-row items-center gap-3 rounded-2xl border border-line bg-surface p-4">
      <View className="flex-1 gap-1">
        <Text numberOfLines={1}>
          {p.from.name} → {p.to.name}
        </Text>
        <Text variant="caption">
          {p.depart_from}–{p.depart_to} · {DAYS.filter((_, i) => p.weekdays.includes(i + 1)).join(' ') || '—'} · {p.stops} stops
        </Text>
      </View>
      <Switch value={p.active} onValueChange={(v) => toggle.mutate(v)} trackColor={{ true: colors.accent, false: colors.line }} thumbColor={colors.ink} />
    </Pressable>
  );
}

export default function TravelTab() {
  const plans = usePlans();
  const matches = useMatches();
  const trip = useActiveTrip();

  if (needsConsent(plans.error)) {
    return (
      <SafeAreaView edges={['top']} className="flex-1 bg-bg">
        <TabHeader title="Travel" />
        <ConsentCard />
      </SafeAreaView>
    );
  }
  const live = (matches.data ?? []).filter((m) => m.status !== 'declined');
  return (
    <SafeAreaView edges={['top']} className="flex-1 bg-bg">
      <TabHeader
        title="Travel"
        right={
          <Pressable onPress={() => router.push('/travel/preferences')} className="h-10 w-10 items-center justify-center rounded-full bg-raised" accessibilityLabel="Matching preferences">
            <Settings2 size={18} color={colors.ink} />
          </Pressable>
        }
      />
      <ScrollView
        contentContainerClassName="gap-4 px-4 pb-10"
        refreshControl={<RefreshControl refreshing={matches.isRefetching} onRefresh={() => void post('/v1/matches/refresh').catch(() => {}).then(() => matches.refetch())} tintColor={colors.accent} />}
      >
        {trip.data ? (
          <Pressable onPress={() => router.push(`/trip/${trip.data!.id}`)} className="gap-2 rounded-2xl border border-mint/50 bg-mint/10 p-4">
            <View className="flex-row items-center gap-2">
              <ShieldCheck size={18} color={colors.mint} />
              <Text variant="label" className="text-mint">
                Upcoming trip
              </Text>
            </View>
            <Text>
              {trip.data.meet_station.name} · {new Date(trip.data.meet_at).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })}
            </Text>
            <Text variant="caption">Tap for meet point, check-in, share trip and SOS</Text>
          </Pressable>
        ) : null}
        <TravelNow />
        <Text variant="heading" className="mt-2">
          Matches
        </Text>
        {live.length === 0 ? (
          <Text className="text-muted">No matches yet. Start Travel Now or save your commute and we’ll look for people on your route.</Text>
        ) : (
          live.map((m) => <MatchCard key={m.id} m={m} />)
        )}
        <CrewsSection />
        <View className="mt-2 flex-row items-center justify-between">
          <Text variant="heading">My commutes</Text>
          <Pressable onPress={() => router.push('/travel/plan')} hitSlop={10} accessibilityLabel="Add commute">
            <Plus size={22} color={colors.accent} />
          </Pressable>
        </View>
        {(plans.data ?? []).map((p) => (
          <PlanRow key={p.id} p={p} />
        ))}
        {plans.data?.length === 0 ? <Text className="text-muted">Save your daily commute and it goes live automatically before you leave.</Text> : null}
      </ScrollView>
    </SafeAreaView>
  );
}
