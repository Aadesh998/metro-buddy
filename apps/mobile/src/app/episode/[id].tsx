import { useMutation, useQueryClient } from '@tanstack/react-query';
import * as Haptics from 'expo-haptics';
import { router, useLocalSearchParams } from 'expo-router';
import { Headphones, Mic, MicOff, MoreHorizontal, PhoneCall, Send } from 'lucide-react-native';
import { useCallback, useEffect, useState } from 'react';
import { KeyboardAvoidingView, Modal, Platform, Pressable, ScrollView, Switch, View } from 'react-native';
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
import { voiceMaskSupported } from '@/features/studio/voice';
import { useT } from '@/i18n';
import { useNow } from '@/lib/clock';
import { confirm } from '@/lib/confirm';
import { colors } from '@/lib/theme';

const REACTIONS: Record<string, string> = { heart: '❤️', hug: '🤗', clap: '👏', laugh: '😂', fire: '🔥', same: '🙋' };

function Timer({ until }: { until: string }) {
  const { tp } = useT();
  const now = useNow(1000);
  const s = Math.max(0, Math.round((Date.parse(until) - now) / 1000));
  return (
    <Text className={`font-mono text-sm font-bold ${s < 60 ? 'text-danger' : 'text-accent'}`} accessibilityLabel={tp('studio.episode.minutesLeft', Math.ceil(s / 60))}>
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

function CallInSheet({ ep, visible, onClose, onMasked }: { ep: Episode; visible: boolean; onClose: () => void; onMasked: (on: boolean) => void }) {
  const { t } = useT();
  const qc = useQueryClient();
  const insets = useSafeAreaInsets();
  const [topic, setTopic] = useState('');
  const [mask, setMask] = useState(false);
  const send = useMutation({
    mutationFn: () => post(`/v1/episodes/${ep.id}/call-in`, { topic: topic.trim(), voice_masked: mask }),
    onSuccess: () => {
      onMasked(mask);
      setTopic('');
      onClose();
      qc.invalidateQueries({ queryKey: studioKeys.episode(ep.id) });
      toast(t('studio.episode.queuedToast'), 'success');
    },
    onError: (e) => toast(e.message),
  });
  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={onClose}>
      <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} className="flex-1 justify-end bg-black/60">
        <Pressable className="flex-1" onPress={onClose} accessibilityLabel={t('common.close')} />
        <View className="gap-4 rounded-t-3xl bg-surface p-5" style={{ paddingBottom: insets.bottom + 20 }}>
          <Text variant="heading">{t('studio.episode.callInTitle', { show: ep.show_title })}</Text>
          <Text className="text-sm text-muted">{t('studio.episode.callInBody')}</Text>
          <Input value={topic} onChangeText={(v) => setTopic(v.slice(0, 80))} placeholder={t('studio.episode.callInPlaceholder')} autoFocus />
          <Text variant="caption">{t('studio.episode.charsLeft', { n: 80 - topic.length })}</Text>
          {voiceMaskSupported ? (
            <View className="flex-row items-center gap-3">
              <View className="flex-1">
                <Text variant="label">{t('studio.episode.maskVoice')}</Text>
                <Text variant="caption">{t('studio.episode.maskVoiceHint')}</Text>
              </View>
              <Switch value={mask} onValueChange={setMask} trackColor={{ true: colors.accent, false: colors.line }} thumbColor={colors.ink} />
            </View>
          ) : null}
          <Button title={t('studio.episode.joinQueue')} disabled={topic.trim().length < 3} loading={send.isPending} onPress={() => send.mutate()} />
        </View>
      </KeyboardAvoidingView>
    </Modal>
  );
}

function AskBox({ ep }: { ep: Episode }) {
  const { t } = useT();
  const [text, setText] = useState('');
  const ask = useMutation({
    mutationFn: () => post(`/v1/episodes/${ep.id}/questions`, { text: text.trim() }),
    onSuccess: () => {
      setText('');
      toast(t('studio.episode.questionSent'), 'success');
    },
    onError: (e) => toast(e.message),
  });
  return (
    <View className="flex-row items-end gap-2">
      <View className="flex-1">
        <Input value={text} onChangeText={(v) => setText(v.slice(0, 200))} placeholder={t('studio.episode.askPlaceholder')} />
      </View>
      <Pressable
        onPress={() => ask.mutate()}
        disabled={text.trim().length < 3 || ask.isPending}
        accessibilityLabel={t('studio.episode.sendQuestion')}
        className={`h-12 w-12 items-center justify-center rounded-xl ${text.trim().length < 3 ? 'bg-raised' : 'bg-accent'}`}
      >
        <Send size={18} color={text.trim().length < 3 ? colors.faint : colors.accentInk} />
      </Pressable>
    </View>
  );
}

type Floater = { key: number; emoji: string; left: number };

function Reactions({ ep, floaters }: { ep: Episode; floaters: Floater[] }) {
  const { t } = useT();
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
            accessibilityLabel={t('studio.episode.react', { emoji: e })}
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
  const { t, tp } = useT();
  const qc = useQueryClient();
  const episode = useEpisode(id);
  const safety = useSafetySheet();
  const { state, join, leave, setMic, setMasked } = useLiveRoom(id);
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
  const maskedOnServer = !!ep?.my_call_in?.voice_masked;
  useEffect(() => {
    if (maskedOnServer) setMasked(true);
  }, [maskedOnServer, setMasked]);

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

  const confirmEnd = () => confirm(t('studio.episode.endTitle'), t('studio.episode.endBody'), t('studio.episode.end'), () => action.mutate('end'));

  return (
    <SafeAreaView className="flex-1 bg-bg">
      <ScreenHeader
        title={ep.show_title}
        right={
          !isHost ? (
            <Pressable onPress={() => safety.open({ type: 'episode', id: ep.id, personaId: ep.host.id, alias: ep.host.alias })} hitSlop={12} accessibilityLabel={t('common.safetyOptions')}>
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
              {ep.status === 'live' ? tp('studio.listening', ep.listeners) : ended ? tp('studio.episode.endedPeak', ep.peak_listeners) : whenLabel(ep.scheduled_at, now)}
            </Text>
          </View>
        </View>

        {ep.status === 'scheduled' && !isHost ? (
          <View className="gap-3">
            <Text className="text-muted">{t('studio.episode.notStarted')}</Text>
            <Button title={t('studio.episode.seeShow')} kind="secondary" onPress={() => router.push(`/show/${ep.show_id}`)} />
          </View>
        ) : null}

        {ep.status === 'live' ? (
          <View className="flex-row justify-around rounded-3xl border border-line bg-surface py-6">
            <Speaker p={ep.host} label={t('studio.host')} speaking={state.speaking.has(ep.host.id)} />
            {ep.on_air ? (
              <View className="items-center gap-1">
                <Speaker p={ep.on_air.persona} label={t('studio.episode.onAir')} speaking={state.speaking.has(ep.on_air.persona.id)} />
                {ep.on_air.ends_at ? <Timer until={ep.on_air.ends_at} /> : null}
              </View>
            ) : (
              <View className="items-center justify-center gap-1">
                <View className="h-[72px] w-[72px] items-center justify-center rounded-full border-2 border-dashed border-line">
                  <PhoneCall size={22} color={colors.faint} />
                </View>
                <Text variant="caption">{t('studio.episode.openLine')}</Text>
              </View>
            )}
          </View>
        ) : null}

        {ep.status === 'live' && !connected ? (
          <View className="gap-2">
            <Button
              title={state.status === 'connecting' ? t('studio.episode.connecting') : isHost ? t('studio.episode.openStudio') : t('studio.episode.listenIn')}
              loading={state.status === 'connecting'}
              onPress={join}
            />
            {state.status === 'error' ? <Text className="text-center text-sm text-danger">{state.error}</Text> : null}
            <Text variant="caption" className="text-center">
              {t('studio.episode.neverRecorded')}
            </Text>
          </View>
        ) : null}

        {state.status === 'reconnecting' ? <Text className="text-center text-sm text-accent">{t('studio.episode.reconnecting')}</Text> : null}

        {connected && ep.status === 'live' ? (
          <View className="gap-4">
            {meOnAir ? (
              <View className="gap-3 rounded-2xl border border-accent/50 bg-accent/10 p-4">
                <Text variant="heading">{t('studio.episode.youreOnAir')}</Text>
                <Text className="text-sm text-muted">{t('studio.episode.onAirBody')}</Text>
                {state.error ? <Text className="text-sm text-danger">{state.error}</Text> : null}
                <View className="flex-row gap-2">
                  <Button
                    title={state.micOn ? t('common.mute') : t('studio.episode.unmute')}
                    kind="secondary"
                    className="flex-1"
                    onPress={() => setMic(!state.micOn)}
                  />
                  <Button title={t('studio.episode.hangUp')} kind="danger" className="flex-1" onPress={() => withdraw.mutate()} />
                </View>
              </View>
            ) : null}
            {isHost ? (
              <View className="gap-3">
                <View className="flex-row gap-2">
                  <Pressable
                    onPress={() => setMic(!state.micOn)}
                    accessibilityLabel={state.micOn ? t('studio.episode.muteMic') : t('studio.episode.unmuteMic')}
                    className={`h-12 flex-1 flex-row items-center justify-center gap-2 rounded-xl ${state.micOn ? 'bg-raised' : 'bg-danger'}`}
                  >
                    {state.micOn ? <Mic size={18} color={colors.ink} /> : <MicOff size={18} color="#fff" />}
                    <Text className={state.micOn ? 'text-ink' : 'text-white'}>{state.micOn ? t('studio.episode.micOn') : t('studio.episode.micMuted')}</Text>
                  </Pressable>
                  <Button title={t('studio.episode.endEpisode')} kind="danger" className="flex-1" onPress={confirmEnd} />
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
                    <Text className="text-xs text-muted underline">{t('studio.episode.reportCaller')}</Text>
                  </Pressable>
                ) : null}
                {!ep.my_call_in ? (
                  <Button title={t('studio.episode.callIn')} onPress={() => setCalling(true)} />
                ) : ep.my_call_in.status === 'queued' ? (
                  <View className="gap-2 rounded-2xl border border-line bg-surface p-4">
                    <Text variant="label">{t('studio.episode.inQueue')}</Text>
                    <Text className="text-sm text-muted">“{ep.my_call_in.topic}”</Text>
                    <Button title={t('studio.episode.leaveQueue')} kind="secondary" onPress={() => del(`/v1/episodes/${id}/call-in`).catch(() => {}).finally(refresh)} />
                  </View>
                ) : null}
                <Reactions ep={ep} floaters={floaters} />
                <AskBox ep={ep} />
              </View>
            )}
            <Button
              title={t('common.leave')}
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
            <Button title={t('studio.episode.goLive')} disabled={scheduledMs - now > 15 * 60_000} loading={action.isPending} onPress={() => action.mutate('start', { onSuccess: () => join() })} />
            {scheduledMs - now > 15 * 60_000 ? <Text variant="caption" className="text-center">{t('studio.episode.goLiveHint')}</Text> : null}
          </View>
        ) : null}

        {ended ? (
          <View className="items-center gap-2 py-6">
            <Headphones size={32} color={colors.muted} />
            <Text className="text-muted">{t('studio.episode.ended')}</Text>
          </View>
        ) : null}
      </ScrollView>
      <CallInSheet ep={ep} visible={calling} onClose={() => setCalling(false)} onMasked={setMasked} />
    </SafeAreaView>
  );
}
