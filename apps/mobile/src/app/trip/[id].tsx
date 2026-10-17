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
import { colors } from '@/lib/theme';

function Feedback({ trip }: { trip: Trip }) {
  const qc = useQueryClient();
  const [thumbs, setThumbs] = useState<1 | -1 | null>(null);
  const [safe, setSafe] = useState(true);
  const [note, setNote] = useState('');
  const send = useMutation({
    mutationFn: () => post(`/v1/trips/${trip.id}/feedback`, { thumbs, felt_safe: safe, note }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: travelKeys.tripById(trip.id) });
      toast('Thanks. Your feedback stays private.', 'success');
    },
  });
  if (trip.feedback_given) return <Text className="text-center text-muted">Thanks for your feedback.</Text>;
  return (
    <View className="gap-3 rounded-2xl border border-line bg-surface p-4">
      <Text variant="label">How was it? (private)</Text>
      <View className="flex-row gap-3">
        <Button title="Good" kind={thumbs === 1 ? 'primary' : 'secondary'} className="flex-1" onPress={() => setThumbs(1)} />
        <Button title="Not great" kind={thumbs === -1 ? 'primary' : 'secondary'} className="flex-1" onPress={() => setThumbs(-1)} />
      </View>
      <View className="flex-row gap-3">
        <Button title="I felt safe" kind={safe ? 'primary' : 'secondary'} className="flex-1" onPress={() => setSafe(true)} />
        <Button title="I didn't feel safe" kind={!safe ? 'danger' : 'secondary'} className="flex-1" onPress={() => setSafe(false)} />
      </View>
      {!safe ? <Text variant="caption">Our safety team will look into this. The other person won’t know it was you.</Text> : null}
      <Input value={note} onChangeText={(t) => setNote(t.slice(0, 300))} placeholder="Anything to add? (optional)" multiline />
      <Button title="Send" disabled={thumbs === null} loading={send.isPending} onPress={() => send.mutate()} />
    </View>
  );
}

export default function TripScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const qc = useQueryClient();
  const trip = useTrip(id);
  const persona = useMyPersona().data;
  const [passing, setPassing] = useState(false);
  const t = trip.data;
  const refresh = () => {
    qc.invalidateQueries({ queryKey: travelKeys.tripById(id) });
    qc.invalidateQueries({ queryKey: travelKeys.trip });
  };

  const checkin = useMutation({ mutationFn: () => post(`/v1/trips/${id}/checkin`), onSuccess: refresh });
  const share = useMutation({
    mutationFn: () => post<{ url: string }>(`/v1/trips/${id}/share`),
    onSuccess: (r) => {
      refresh();
      Share.share({ message: `I'm on a Metro Buddy trip. Live station updates: ${r.url}` });
    },
  });
  // One tap: no confirmation dialog stands between someone in trouble and help.
  const sos = useMutation({
    mutationFn: () => post<{ contacts_notified: number }>('/v1/sos', { trip_id: id }),
    onMutate: () => Haptics.notificationAsync(Haptics.NotificationFeedbackType.Warning).catch(() => {}),
    onSuccess: (r) =>
      Alert.alert(
        'SOS sent',
        r.contacts_notified
          ? `We texted ${r.contacts_notified} trusted contact${r.contacts_notified > 1 ? 's' : ''} your live trip link and alerted our safety team.`
          : 'Our safety team has been alerted. Add trusted contacts so they get a text next time.',
        [
          { text: 'Call 112', style: 'destructive', onPress: () => Linking.openURL('tel:112') },
          { text: 'OK' },
        ],
      ),
  });
  const progress = useMutation({ mutationFn: (sid: string) => post(`/v1/trips/${id}/progress`, { station_id: sid }), onSuccess: () => toast('Updated', 'success') });

  if (!t) return null;
  const me = t.participants.find((p) => p.is_me);
  const active = t.status === 'planned' || t.status === 'met';

  return (
    <SafeAreaView edges={['top', 'bottom']} className="flex-1 bg-bg">
      <ScreenHeader title="Your trip" />
      <ScrollView contentContainerClassName="gap-4 px-4 pb-10">
        {active ? (
          <Pressable
            onPress={() => sos.mutate()}
            disabled={sos.isPending}
            className="flex-row items-center justify-center gap-3 rounded-2xl bg-danger py-5 active:opacity-80"
            accessibilityRole="button"
            accessibilityLabel="SOS. Alerts your trusted contacts and our safety team"
          >
            <Siren size={26} color="#fff" />
            <Text className="text-xl font-black text-white">{sos.isPending ? 'Sending…' : 'SOS'}</Text>
          </Pressable>
        ) : null}

        <View className="gap-3 rounded-2xl border border-mint/40 bg-mint/10 p-4">
          <View className="flex-row items-center gap-2">
            <MapPin size={18} color={colors.mint} />
            <Text variant="label" className="text-mint">
              Meet inside the station
            </Text>
          </View>
          <Text variant="heading">{t.meet_station.name}</Text>
          <Text>{t.meet_platform}</Text>
          <Text>{t.meet_coach}</Text>
          <Text variant="caption">{new Date(t.meet_at).toLocaleString([], { weekday: 'short', hour: 'numeric', minute: '2-digit' })}</Text>
        </View>

        <View className="gap-2">
          {t.participants.map((p) => (
            <View key={p.persona.id} className="flex-row items-center gap-3 rounded-xl bg-surface p-3">
              <Avatar seed={p.persona.avatar_seed} size={36} />
              <Text className="flex-1">{p.is_me ? `${p.persona.alias} (you)` : p.persona.alias}</Text>
              {p.checked_in ? <CheckCircle2 size={18} color={colors.mint} /> : <Text variant="caption">{p.no_show ? 'no-show' : 'not here yet'}</Text>}
            </View>
          ))}
        </View>

        {active && !me?.checked_in ? <Button title="I'm at the meet point" loading={checkin.isPending} onPress={() => checkin.mutate()} /> : null}
        <View className="flex-row gap-2">
          {t.conversation_id ? (
            <Button title="Chat" kind="secondary" className="flex-1" onPress={() => router.push(`/chat/${t.conversation_id}`)} />
          ) : null}
          {active ? <Button title={t.shared ? 'Share again' : 'Share trip'} kind="secondary" className="flex-1" loading={share.isPending} onPress={() => share.mutate()} /> : null}
        </View>
        {active ? <Button title="Update: I just passed a station" kind="ghost" onPress={() => setPassing(true)} /> : null}
        <View className="flex-row items-center gap-2 px-1">
          <Phone size={14} color={colors.muted} />
          <Text variant="caption" onPress={() => Linking.openURL('tel:112')}>
            Emergency 112 · DMRC helpline 155370
          </Text>
        </View>
        {!active || me?.checked_in ? <Feedback trip={t} /> : null}
        {t.match_id && (t.status === 'met' || t.status === 'completed') ? <StartCrew matchId={t.match_id} /> : null}
      </ScrollView>
      {persona ? (
        <StationPicker
          cityId={persona.city_id}
          visible={passing}
          title="Which station did you just pass?"
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
