import { useMutation, useQueryClient } from '@tanstack/react-query';
import * as Haptics from 'expo-haptics';
import { router, useLocalSearchParams } from 'expo-router';
import { CheckCircle2, MapPin, Phone, Siren } from 'lucide-react-native';
import { useState } from 'react';
import { Alert, Linking, Pressable, ScrollView, Share, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { post } from '@/api/client';
import type { Trip } from '@/api/types';
import { Avatar } from '@/components/Avatar';
import { ScreenHeader } from '@/components/ScreenHeader';
import { StationPicker } from '@/components/StationPicker';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';
import { useMyPersona } from '@/features/me/hooks';
import { StartCrew } from '@/features/travel/Crews';
import { travelKeys, useTrip } from '@/features/travel/hooks';
import { dateLocale, useT } from '@/i18n';
import { colors } from '@/lib/theme';

function Feedback({ trip }: { trip: Trip }) {
  const { t } = useT();
  const qc = useQueryClient();
  const [thumbs, setThumbs] = useState<1 | -1 | null>(null);
  const [safe, setSafe] = useState(true);
  const [note, setNote] = useState('');
  const send = useMutation({
    mutationFn: () => post(`/v1/trips/${trip.id}/feedback`, { thumbs, felt_safe: safe, note }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: travelKeys.tripById(trip.id) });
      toast(t('travel.trip.feedbackToast'), 'success');
    },
  });
  if (trip.feedback_given) return <Text className="text-center text-muted">{t('travel.trip.feedbackThanks')}</Text>;
  return (
    <View className="gap-3 rounded-2xl border border-line bg-surface p-4">
      <Text variant="label">{t('travel.trip.howWasIt')}</Text>
      <View className="flex-row gap-3">
        <Button title={t('travel.trip.good')} kind={thumbs === 1 ? 'primary' : 'secondary'} className="flex-1" onPress={() => setThumbs(1)} />
        <Button title={t('travel.trip.notGreat')} kind={thumbs === -1 ? 'primary' : 'secondary'} className="flex-1" onPress={() => setThumbs(-1)} />
      </View>
      <View className="flex-row gap-3">
        <Button title={t('travel.trip.feltSafe')} kind={safe ? 'primary' : 'secondary'} className="flex-1" onPress={() => setSafe(true)} />
        <Button title={t('travel.trip.notSafe')} kind={!safe ? 'danger' : 'secondary'} className="flex-1" onPress={() => setSafe(false)} />
      </View>
      {!safe ? <Text variant="caption">{t('travel.trip.safetyNote')}</Text> : null}
      <Input value={note} onChangeText={(v) => setNote(v.slice(0, 300))} placeholder={t('travel.trip.notePlaceholder')} multiline />
      <Button title={t('common.send')} disabled={thumbs === null} loading={send.isPending} onPress={() => send.mutate()} />
    </View>
  );
}

export default function TripScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { t, tp } = useT();
  const qc = useQueryClient();
  const trip = useTrip(id);
  const persona = useMyPersona().data;
  const [passing, setPassing] = useState(false);
  const trp = trip.data;
  const refresh = () => {
    qc.invalidateQueries({ queryKey: travelKeys.tripById(id) });
    qc.invalidateQueries({ queryKey: travelKeys.trip });
  };

  const checkin = useMutation({ mutationFn: () => post(`/v1/trips/${id}/checkin`), onSuccess: refresh });
  const share = useMutation({
    mutationFn: () => post<{ url: string }>(`/v1/trips/${id}/share`),
    onSuccess: (r) => {
      refresh();
      Share.share({ message: t('travel.trip.shareMessage', { url: r.url }) });
    },
  });
  // One tap: no confirmation dialog stands between someone in trouble and help.
  const sos = useMutation({
    mutationFn: () => post<{ contacts_notified: number }>('/v1/sos', { trip_id: id }),
    onMutate: () => Haptics.notificationAsync(Haptics.NotificationFeedbackType.Warning).catch(() => {}),
    onSuccess: (r) =>
      Alert.alert(
        t('travel.trip.sosSent'),
        r.contacts_notified ? tp('travel.trip.sosContacts', r.contacts_notified) : t('travel.trip.sosNoContacts'),
        [
          { text: t('travel.trip.call112'), style: 'destructive', onPress: () => Linking.openURL('tel:112') },
          { text: t('common.ok') },
        ],
      ),
  });
  const progress = useMutation({ mutationFn: (sid: string) => post(`/v1/trips/${id}/progress`, { station_id: sid }), onSuccess: () => toast(t('travel.trip.updated'), 'success') });

  if (!trp) return null;
  const me = trp.participants.find((p) => p.is_me);
  const active = trp.status === 'planned' || trp.status === 'met';

  return (
    <SafeAreaView edges={['top', 'bottom']} className="flex-1 bg-bg">
      <ScreenHeader title={t('travel.trip.title')} />
      <ScrollView contentContainerClassName="gap-4 px-4 pb-10">
        {active ? (
          <Pressable
            onPress={() => sos.mutate()}
            disabled={sos.isPending}
            className="flex-row items-center justify-center gap-3 rounded-2xl bg-danger py-5 active:opacity-80"
            accessibilityRole="button"
            accessibilityLabel={t('travel.trip.sosLabel')}
          >
            <Siren size={26} color="#fff" />
            <Text className="text-xl font-black text-white">{sos.isPending ? t('travel.trip.sending') : 'SOS'}</Text>
          </Pressable>
        ) : null}

        <View className="gap-3 rounded-2xl border border-mint/40 bg-mint/10 p-4">
          <View className="flex-row items-center gap-2">
            <MapPin size={18} color={colors.mint} />
            <Text variant="label" className="text-mint">
              {t('travel.trip.meetInside')}
            </Text>
          </View>
          <Text variant="heading">{trp.meet_station.name}</Text>
          <Text>{trp.meet_platform}</Text>
          <Text>{trp.meet_coach}</Text>
          <Text variant="caption">{new Date(trp.meet_at).toLocaleString(dateLocale(), { weekday: 'short', hour: 'numeric', minute: '2-digit' })}</Text>
        </View>

        <View className="gap-2">
          {trp.participants.map((p) => (
            <View key={p.persona.id} className="flex-row items-center gap-3 rounded-xl bg-surface p-3">
              <Avatar seed={p.persona.avatar_seed} size={36} />
              <Text className="flex-1">{p.is_me ? t('travel.trip.you', { alias: p.persona.alias }) : p.persona.alias}</Text>
              {p.checked_in ? <CheckCircle2 size={18} color={colors.mint} /> : <Text variant="caption">{p.no_show ? t('travel.trip.noShow') : t('travel.trip.notHere')}</Text>}
            </View>
          ))}
        </View>

        {active && !me?.checked_in ? <Button title={t('travel.trip.atMeetPoint')} loading={checkin.isPending} onPress={() => checkin.mutate()} /> : null}
        <View className="flex-row gap-2">
          {trp.conversation_id ? (
            <Button title={t('travel.trip.chat')} kind="secondary" className="flex-1" onPress={() => router.push(`/chat/${trp.conversation_id}`)} />
          ) : null}
          {active ? <Button title={trp.shared ? t('travel.trip.shareAgain') : t('travel.trip.shareTrip')} kind="secondary" className="flex-1" loading={share.isPending} onPress={() => share.mutate()} /> : null}
        </View>
        {active ? <Button title={t('travel.trip.passedStation')} kind="ghost" onPress={() => setPassing(true)} /> : null}
        <View className="flex-row items-center gap-2 px-1">
          <Phone size={14} color={colors.muted} />
          <Text variant="caption" onPress={() => Linking.openURL('tel:112')}>
            {t('travel.trip.emergency')}
          </Text>
        </View>
        {!active || me?.checked_in ? <Feedback trip={trp} /> : null}
        {trp.match_id && (trp.status === 'met' || trp.status === 'completed') ? <StartCrew matchId={trp.match_id} /> : null}
      </ScrollView>
      {persona ? (
        <StationPicker
          cityId={persona.city_id}
          visible={passing}
          title={t('travel.trip.whichStation')}
          onClose={() => setPassing(false)}
          onPick={(s) => {
            setPassing(false);
            progress.mutate(s.id);
          }}
        />
      ) : null}
    </SafeAreaView>
  );
}
