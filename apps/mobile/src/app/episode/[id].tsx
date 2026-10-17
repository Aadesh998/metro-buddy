import { useMutation, useQueryClient } from '@tanstack/react-query';
import * as Haptics from 'expo-haptics';
import { router, useLocalSearchParams } from 'expo-router';
import { Headphones, Mic, MicOff, MoreHorizontal, PhoneCall, Send } from 'lucide-react-native';
import { useCallback, useEffect, useState } from 'react';
import { KeyboardAvoidingView, Modal, Platform, Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView, useSafeAreaInsets } from 'react-native-safe-area-context';

import { del, post } from '@/api/client';
import type { Episode, PersonaRef } from '@/api/types';
import { Avatar } from '@/components/Avatar';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';
import { useMyPersona } from '@/features/me/hooks';
import { useSafetySheet } from '@/features/safety/SafetySheet';
import { whenLabel } from '@/features/studio/EpisodeCard';
import { HostPanel, OnAirControls } from '@/features/studio/HostControls';
import { studioKeys, useEpisode, useEpisodeEvents } from '@/features/studio/hooks';
import { useLiveRoom } from '@/features/studio/useLiveRoom';
import { useNow } from '@/lib/clock';
import { confirm } from '@/lib/confirm';
import { colors } from '@/lib/theme';

const REACTIONS: Record<string, string> = { heart: '❤️', hug: '🤗', clap: '👏', laugh: '😂', fire: '🔥', same: '🙋' };

function Timer({ until }: { until: string }) {
  const now = useNow(1000);
  const s = Math.max(0, Math.round((Date.parse(until) - now) / 1000));
  return (
    <Text className={`font-mono text-sm font-bold ${s < 60 ? 'text-danger' : 'text-accent'}`} accessibilityLabel={`${Math.ceil(s / 60)} minutes left`}>
      {`${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`}
    </Text>
  );
}

function Speaker({ p, label, speaking }: { p: PersonaRef; label: string; speaking: boolean }) {
  return (
    <View className="items-center gap-1.5">
      <View className={`rounded-full p-1 ${speaking ? 'border-2 border-accent' : 'border-2 border-transparent'}`}>
        <Avatar seed={p.avatar_seed} size={64} />
      </View>
      <Text variant="label" numberOfLines={1} className="max-w-28">
        {p.alias}
      </Text>
      <Text variant="caption">{label}</Text>
    </View>
  );
}

function CallInSheet({ ep, visible, onClose }: { ep: Episode; visible: boolean; onClose: () => void }) {
  const qc = useQueryClient();
  const insets = useSafeAreaInsets();
  const [topic, setTopic] = useState('');
  const send = useMutation({
    mutationFn: () => post(`/v1/episodes/${ep.id}/call-in`, { topic: topic.trim() }),
    onSuccess: () => {
      setTopic('');
      onClose();
      qc.invalidateQueries({ queryKey: studioKeys.episode(ep.id) });
      toast('You’re in the queue. The host will bring you on air.', 'success');
    },
    onError: (e) => toast(e.message),
  });
  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={onClose}>
      <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} className="flex-1 justify-end bg-black/60">
        <Pressable className="flex-1" onPress={onClose} accessibilityLabel="Close" />
        <View className="gap-4 rounded-t-3xl bg-surface p-5" style={{ paddingBottom: insets.bottom + 20 }}>
          <Text variant="heading">Call in to {ep.show_title}</Text>
          <Text className="text-sm text-muted">Tell the host what you want to talk about. Only the host sees this. You’ll be on air for up to 8 minutes.</Text>
          <Input value={topic} onChangeText={(t) => setTopic(t.slice(0, 80))} placeholder="My manager takes credit for my work" autoFocus />
          <Text variant="caption">{80 - topic.length} characters left · phone numbers and names are masked</Text>
          <Button title="Join the queue" disabled={topic.trim().length < 3} loading={send.isPending} onPress={() => send.mutate()} />
        </View>
      </KeyboardAvoidingView>
    </Modal>
  );
}

function AskBox({ ep }: { ep: Episode }) {
  const [text, setText] = useState('');
  const ask = useMutation({
    mutationFn: () => post(`/v1/episodes/${ep.id}/questions`, { text: text.trim() }),
    onSuccess: () => {
      setText('');
      toast('Question sent to the host', 'success');
    },
    onError: (e) => toast(e.message),
  });
  return (
    <View className="flex-row items-end gap-2">
      <View className="flex-1">
        <Input value={text} onChangeText={(t) => setText(t.slice(0, 200))} placeholder="Ask the host something" />
      </View>
      <Pressable
        onPress={() => ask.mutate()}
        disabled={text.trim().length < 3 || ask.isPending}
        accessibilityLabel="Send question"
        className={`h-12 w-12 items-center justify-center rounded-xl ${text.trim().length < 3 ? 'bg-raised' : 'bg-accent'}`}
      >
        <Send size={18} color={text.trim().length < 3 ? colors.faint : colors.accentInk} />
      </Pressable>
    </View>
  );
}

type Floater = { key: number; emoji: string; left: number };

function Reactions({ ep, floaters }: { ep: Episode; floaters: Floater[] }) {
  const react = useMutation({ mutationFn: (r: string) => post(`/v1/episodes/${ep.id}/reactions`, { reaction: r }) });
  return (
    <View>
      <View className="pointer-events-none absolute -top-16 left-0 right-0 h-16">
        {floaters.map((f) => (
          <Text key={f.key} className="absolute text-2xl" style={{ left: `${f.left}%` }}>
            {f.emoji}
          </Text>
        ))}
      </View>
      <View className="flex-row justify-between">
        {Object.entries(REACTIONS).map(([k, e]) => (
          <Pressable
            key={k}
            onPress={() => {
              Haptics.selectionAsync().catch(() => {});
              react.mutate(k);
            }}
            accessibilityLabel={`React ${k}`}
            className="h-11 w-11 items-center justify-center rounded-full bg-raised"
          >
            <Text className="text-xl">{e}</Text>
          </Pressable>
        ))}
      </View>
    </View>
  );
}

export default function EpisodeScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const qc = useQueryClient();
  const episode = useEpisode(id);
  const safety = useSafetySheet();
  const { state, join, leave, setMic } = useLiveRoom(id);
  const [calling, setCalling] = useState(false);
  const [floaters, setFloaters] = useState<Floater[]>([]);
  const now = useNow(60_000);
  const ep = episode.data;
  const me = useMyPersona().data;

  const onReaction = useCallback((r: string) => {
    const f = { key: Math.random(), emoji: REACTIONS[r] ?? '❤️', left: 5 + Math.random() * 85 };
    setFloaters((cur) => [...cur.slice(-12), f]);
    setTimeout(() => setFloaters((cur) => cur.filter((x) => x.key !== f.key)), 1800);
  }, []);
  useEpisodeEvents(id, onReaction);

  const ended = ep?.status === 'ended' || ep?.status === 'cancelled';
  useEffect(() => {
    if (ended && state.status !== 'idle') leave(false);
  }, [ended, state.status, leave]);

  const refresh = () => qc.invalidateQueries({ queryKey: studioKeys.episode(id) });
  const action = useMutation({
    mutationFn: (path: string) => post(`/v1/episodes/${id}/${path}`),
    onSuccess: refresh,
    onError: (e) => toast(e.message),
  });
  const withdraw = useMutation({ mutationFn: () => del(`/v1/episodes/${id}/call-in`), onSuccess: refresh });

  if (!ep) return <SafeAreaView className="flex-1 bg-bg" />;
  const role = ep.my_role ?? state.role;
  const isHost = ep.host.id === me?.id || role === 'co_host';
  const connected = state.status === 'connected' || state.status === 'reconnecting';
  const meOnAir = ep.on_air && ep.my_call_in?.id === ep.on_air.id;
  const scheduledMs = Date.parse(ep.scheduled_at);

  const confirmEnd = () => confirm('End the episode?', 'Everyone will be disconnected. Nothing is recorded.', 'End', () => action.mutate('end'));

  return (
    <SafeAreaView className="flex-1 bg-bg">
      <ScreenHeader
        title={ep.show_title}
        right={
          !isHost ? (
            <Pressable onPress={() => safety.open({ type: 'episode', id: ep.id, personaId: ep.host.id, alias: ep.host.alias })} hitSlop={12} accessibilityLabel="Safety options">
              <MoreHorizontal color={colors.ink} size={22} />
            </Pressable>
          ) : null
        }
      />
      <ScrollView contentContainerClassName="gap-6 p-4 pb-10" keyboardShouldPersistTaps="handled">
        <View className="gap-1">
          <Text variant="title">{ep.title}</Text>
          <View className="flex-row items-center gap-2">
            {ep.status === 'live' ? (
              <View className="rounded-full bg-danger px-2 py-0.5">
                <Text className="text-xs font-bold text-white">LIVE</Text>
              </View>
            ) : null}
            <Text variant="caption">
              {ep.status === 'live' ? `${ep.listeners} listening` : ended ? `Ended · peak ${ep.peak_listeners} listening` : whenLabel(ep.scheduled_at, now)}
            </Text>
          </View>
        </View>

        {ep.status === 'scheduled' && !isHost ? (
          <View className="gap-3">
            <Text className="text-muted">This episode hasn’t started yet. Follow the show to get a nudge 10 minutes before.</Text>
            <Button title="See the show" kind="secondary" onPress={() => router.push(`/show/${ep.show_id}`)} />
          </View>
        ) : null}

        {ep.status === 'live' ? (
          <View className="flex-row justify-around rounded-3xl border border-line bg-surface py-6">
            <Speaker p={ep.host} label="Host" speaking={state.speaking.has(ep.host.id)} />
            {ep.on_air ? (
              <View className="items-center gap-1">
                <Speaker p={ep.on_air.persona} label="On air" speaking={state.speaking.has(ep.on_air.persona.id)} />
                {ep.on_air.ends_at ? <Timer until={ep.on_air.ends_at} /> : null}
              </View>
            ) : (
              <View className="items-center justify-center gap-1">
                <View className="h-[72px] w-[72px] items-center justify-center rounded-full border-2 border-dashed border-line">
                  <PhoneCall size={22} color={colors.faint} />
                </View>
                <Text variant="caption">Open line</Text>
              </View>
            )}
          </View>
        ) : null}

        {ep.status === 'live' && !connected ? (
          <View className="gap-2">
            <Button
              title={state.status === 'connecting' ? 'Connecting…' : isHost ? 'Open the studio' : 'Listen in'}
              loading={state.status === 'connecting'}
              onPress={join}
            />
            {state.status === 'error' ? <Text className="text-center text-sm text-danger">{state.error}</Text> : null}
            <Text variant="caption" className="text-center">
              Shows are never recorded. Listeners are anonymous.
            </Text>
          </View>
        ) : null}

        {state.status === 'reconnecting' ? <Text className="text-center text-sm text-accent">Reconnecting…</Text> : null}

        {connected && ep.status === 'live' ? (
          <View className="gap-4">
            {meOnAir ? (
              <View className="gap-3 rounded-2xl border border-accent/50 bg-accent/10 p-4">
                <Text variant="heading">You’re on air</Text>
                <Text className="text-sm text-muted">Everyone can hear you. Don’t share your name, workplace or anything that identifies you.</Text>
                <View className="flex-row gap-2">
                  <Button
                    title={state.micOn ? 'Mute' : 'Unmute'}
                    kind="secondary"
                    className="flex-1"
                    onPress={() => setMic(!state.micOn)}
                  />
                  <Button title="Hang up" kind="danger" className="flex-1" onPress={() => withdraw.mutate()} />
                </View>
              </View>
            ) : null}
            {isHost ? (
              <View className="gap-3">
                <View className="flex-row gap-2">
                  <Pressable
                    onPress={() => setMic(!state.micOn)}
                    accessibilityLabel={state.micOn ? 'Mute your mic' : 'Unmute your mic'}
                    className={`h-12 flex-1 flex-row items-center justify-center gap-2 rounded-xl ${state.micOn ? 'bg-raised' : 'bg-danger'}`}
                  >
                    {state.micOn ? <Mic size={18} color={colors.ink} /> : <MicOff size={18} color="#fff" />}
                    <Text className={state.micOn ? 'text-ink' : 'text-white'}>{state.micOn ? 'Mic on' : 'Mic muted'}</Text>
                  </Pressable>
                  <Button title="End episode" kind="danger" className="flex-1" onPress={confirmEnd} />
                </View>
                <OnAirControls ep={ep} />
                <HostPanel ep={ep} />
              </View>
            ) : (
              <View className="gap-4">
                {ep.on_air && !meOnAir ? (
                  <Pressable
                    onPress={() => safety.open({ type: 'call_in', id: ep.on_air!.id, personaId: ep.on_air!.persona.id, alias: ep.on_air!.persona.alias })}
                    className="self-center"
                  >
                    <Text className="text-xs text-muted underline">Report the caller</Text>
                  </Pressable>
                ) : null}
                {!ep.my_call_in ? (
                  <Button title="Call in" onPress={() => setCalling(true)} />
                ) : ep.my_call_in.status === 'queued' ? (
                  <View className="gap-2 rounded-2xl border border-line bg-surface p-4">
                    <Text variant="label">You’re in the queue</Text>
                    <Text className="text-sm text-muted">“{ep.my_call_in.topic}”</Text>
                    <Button title="Leave the queue" kind="secondary" onPress={() => del(`/v1/episodes/${id}/call-in`).catch(() => {}).finally(refresh)} />
                  </View>
                ) : null}
                <Reactions ep={ep} floaters={floaters} />
                <AskBox ep={ep} />
              </View>
            )}
            <Button
              title="Leave"
              kind="ghost"
              onPress={() => {
                leave(true);
                if (isHost) return;
                router.back();
              }}
            />
          </View>
        ) : null}

        {ep.status === 'scheduled' && isHost ? (
          <View className="gap-2">
            <Button title="Go live" disabled={scheduledMs - now > 15 * 60_000} loading={action.isPending} onPress={() => action.mutate('start', { onSuccess: () => join() })} />
            {scheduledMs - now > 15 * 60_000 ? <Text variant="caption" className="text-center">You can go live 15 minutes before the scheduled time.</Text> : null}
          </View>
        ) : null}

        {ended ? (
          <View className="items-center gap-2 py-6">
            <Headphones size={32} color={colors.muted} />
            <Text className="text-muted">This episode has ended. Nothing was recorded.</Text>
          </View>
        ) : null}
      </ScrollView>
      <CallInSheet ep={ep} visible={calling} onClose={() => setCalling(false)} />
    </SafeAreaView>
  );
}
