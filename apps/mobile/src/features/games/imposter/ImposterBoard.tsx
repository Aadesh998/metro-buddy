import * as Haptics from 'expo-haptics';
import { Eye, EyeOff, Send, VenetianMask } from 'lucide-react-native';
import { useEffect, useRef, useState, type RefObject } from 'react';
import { Keyboard, Platform, Pressable, useWindowDimensions, View, type ViewInstance } from 'react-native';
import Animated, { FadeIn, ZoomIn } from 'react-native-reanimated';

import { Avatar } from '@/components/Avatar';
import { FontTextInput } from '@/components/ui/FontTextInput';
import { Text } from '@/components/ui/Text';
import { useT, type Key } from '@/i18n';
import { colors } from '@/lib/theme';
import { secondsLeft, useLive } from '../api';
import type { BoardProps } from '../types';

type Phase = 'clue' | 'discuss' | 'vote' | 'guess' | 'reveal' | 'over';
type Line = { by: string; text: string };

export type ImposterView = {
  phase: Phase;
  deadline?: string;
  round: number;
  rounds: number;
  category: string;
  players: string[];
  order: string[];
  turn?: string;
  clues: { by: string; text: string; skipped?: boolean }[];
  chat: Line[];
  voted: string[];
  score: Record<string, number>;
  imposter_count: number;
  // Only for the viewer themselves.
  role?: 'crew' | 'imposter';
  partners?: string[];
  my_vote?: string;
  // The crew always; everyone from the reveal on.
  word?: string;
  // From the reveal on (result also during the last guess, with out and out_role only).
  imposters?: string[];
  votes?: Record<string, string>;
  result?: {
    out: string | null;
    out_role?: 'crew' | 'imposter';
    guess?: string;
    guess_right?: boolean;
    winner?: 'crew' | 'imposter';
    reason?: 'voted' | 'imposter_left';
    points?: Record<string, number>;
  };
};

const CLUE_MAX = 30;
const SAY_MAX = 140;
const CHAT_SHOWN = 15;

// The game screen is a plain ScrollView, so an input low on the page can end up under the
// keyboard. When that happens the input row slides up just enough to sit above it.
function useKeyboardLift(ref: RefObject<ViewInstance | null>, focused: boolean) {
  const [lift, setLift] = useState(0);
  useEffect(() => {
    if (!focused) return;
    const show = Keyboard.addListener(Platform.OS === 'ios' ? 'keyboardWillShow' : 'keyboardDidShow', (e) => {
      const top = e.endCoordinates.screenY;
      ref.current?.measureInWindow((_x, y, _w, h) => setLift((cur) => Math.max(0, y + cur + h + 8 - top)));
    });
    const hide = Keyboard.addListener(Platform.OS === 'ios' ? 'keyboardWillHide' : 'keyboardDidHide', () => setLift(0));
    return () => {
      show.remove();
      hide.remove();
      setLift(0);
    };
  }, [ref, focused]);
  return lift;
}

function Composer({ placeholder, max, onSend, label }: { placeholder: string; max: number; onSend: (text: string) => void; label: string }) {
  const [text, setText] = useState('');
  const row = useRef<ViewInstance>(null);
  const [focused, setFocused] = useState(false);
  const lift = useKeyboardLift(row, focused);
  const send = () => {
    const v = text.trim();
    if (!v) return;
    onSend(v);
    setText('');
  };
  return (
    <View ref={row} style={{ transform: [{ translateY: -lift }] }} className="flex-row items-center gap-2 bg-bg py-1">
      <FontTextInput
        value={text}
        onChangeText={setText}
        maxLength={max}
        placeholder={placeholder}
        placeholderTextColor={colors.faint}
        returnKeyType="send"
        submitBehavior="submit"
        onSubmitEditing={send}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        accessibilityLabel={placeholder}
        className="min-h-12 min-w-0 flex-1 rounded-full border border-line bg-surface px-4 py-2.5 text-base text-ink"
      />
      <Pressable
        onPress={send}
        disabled={!text.trim()}
        accessibilityRole="button"
        accessibilityLabel={label}
        className={`h-12 w-12 items-center justify-center rounded-full bg-accent ${text.trim() ? 'active:opacity-80' : 'opacity-40'}`}
      >
        <Send size={18} color={colors.accentInk} />
      </Pressable>
    </View>
  );
}

export function ImposterBoard({ room, view, spectator, myId, move, now }: BoardProps<ImposterView>) {
  const { t, tp } = useT();
  const { width } = useWindowDimensions();
  const [peek, setPeek] = useState(false);
  const [picked, setPicked] = useState<{ round: number; target: string } | null>(null);
  // Chat lines pushed live before the next refresh; dropped once the view catches up.
  const chatKey = `${view.round}:${view.chat.length}:${view.chat.at(-1)?.text ?? ''}`;
  const [live, setLive] = useState<{ key: string; lines: Line[] }>({ key: chatKey, lines: [] });
  useLive<Line>(room.id, 'say', (line) => setLive((cur) => ({ key: chatKey, lines: cur.key === chatKey ? [...cur.lines, line] : [line] })));
  const liveLines = live.key === chatKey ? live.lines : [];

  const people = new Map(room.players.map((p) => [p.persona.id, p.persona]));
  const name = (id?: string | null) => (id === myId ? t('game_imposter.you') : (id && people.get(id)?.alias) || t('game_imposter.someone'));
  const avatar = (id: string, size: number) => <Avatar seed={people.get(id)?.avatar_seed ?? id} size={size} />;
  const category = t(`game_imposter.categories.${view.category}` as Key);
  const left = secondsLeft(view.deadline, now);
  const playing = !spectator && !!myId && view.players.includes(myId);
  const myTurn = playing && view.phase === 'clue' && view.turn === myId;
  const canChat = playing && (view.phase === 'discuss' || view.phase === 'vote' || view.phase === 'reveal');
  const myVote = picked && picked.round === view.round ? picked.target : view.my_vote;
  const cols = width >= 400 ? 4 : 3;
  const tile = Math.floor((Math.min(width, 480) - 32 - (cols - 1) * 8) / cols);

  const vote = (target: string) => {
    setPicked({ round: view.round, target });
    move({ type: 'vote', target });
  };
  const chat = [...view.chat, ...liveLines].slice(-CHAT_SHOWN);

  return (
    <View className="gap-4 px-4">
      {/* Role card: held down to peek, so the word isn't on screen for the person in the next seat. */}
      {playing && view.role && view.phase !== 'reveal' && view.phase !== 'over' ? (
        <Pressable
          onPressIn={() => {
            setPeek(true);
            Haptics.selectionAsync().catch(() => {});
          }}
          onPressOut={() => setPeek(false)}
          accessibilityRole="button"
          accessibilityLabel={t('game_imposter.holdToPeek')}
          className={`min-h-28 justify-center rounded-3xl p-5 ${peek ? (view.role === 'imposter' ? 'bg-danger' : 'bg-accent') : 'border border-dashed border-line bg-raised'}`}
        >
          {!peek ? (
            <View className="items-center gap-1.5">
              <EyeOff size={22} color={colors.muted} />
              <Text className="font-semibold">{t('game_imposter.holdToPeek')}</Text>
              <Text variant="caption">{t('game_imposter.holdHint')}</Text>
            </View>
          ) : view.role === 'imposter' ? (
            <View className="items-center gap-1">
              <VenetianMask size={26} color={colors.accentInk} />
              <Text className="text-xl font-bold text-white">{t('game_imposter.youAreImposter')}</Text>
              <Text className="text-white/90">
                {t('game_imposter.category')}: {category}
              </Text>
              {view.partners?.length ? <Text className="text-sm text-white/90">{t('game_imposter.partner', { name: view.partners.map(name).join(', ') })}</Text> : null}
              <Text className="text-xs text-white/80">{t('game_imposter.imposterHint')}</Text>
            </View>
          ) : (
            <View className="items-center gap-1">
              <Eye size={22} color={colors.accentInk} />
              <Text className="text-xs uppercase tracking-[1.5px] text-accent-ink/80">{t('game_imposter.yourWord')}</Text>
              <Text className="text-center text-3xl font-bold text-accent-ink">{view.word}</Text>
              <Text className="text-xs text-accent-ink/80">{t('game_imposter.crewHint')}</Text>
            </View>
          )}
        </Pressable>
      ) : spectator ? (
        <Text variant="caption" className="text-center">
          {t('game_imposter.watching')}
        </Text>
      ) : null}

      {/* Phase banner */}
      <View className={`flex-row items-center gap-3 rounded-2xl p-4 ${view.phase === 'vote' || view.phase === 'guess' ? 'bg-brand-soft' : 'bg-surface border border-line'}`}>
        <View className="min-w-0 flex-1 gap-0.5">
          <Text variant="caption">
            {t('game_imposter.round', { n: view.round, total: view.rounds })} · {t('game_imposter.category')}: {category}
          </Text>
          <Text variant="heading">{t(`game_imposter.phase.${view.phase}`)}</Text>
          <Text variant="caption">
            {myTurn ? t('game_imposter.clueYourTurn') : view.phase === 'clue' && view.turn ? t('game_imposter.turnOf', { name: name(view.turn) }) : t(`game_imposter.phaseHint.${view.phase}`)}
          </Text>
        </View>
        {left !== null ? (
          <Text className={`font-mono text-2xl font-bold ${left <= 5 ? 'text-danger' : 'text-ink'}`}>{t('games.timeLeft', { s: left })}</Text>
        ) : null}
      </View>

      {/* Reveal */}
      {(view.phase === 'reveal' || view.phase === 'over') && view.result ? (
        <Animated.View entering={ZoomIn.springify().damping(14)} className="items-center gap-3 rounded-3xl bg-accent p-6">
          {view.result.out ? avatar(view.result.out, 64) : null}
          <Text className="text-center text-xl font-bold text-accent-ink">
            {view.result.reason === 'imposter_left'
              ? t('game_imposter.reveal.left', { name: name(view.result.out) })
              : view.result.out
                ? t('game_imposter.reveal.out', { name: name(view.result.out) })
                : t('game_imposter.reveal.nobody')}
          </Text>
          {view.result.out && view.result.reason !== 'imposter_left' ? (
            <Text className={`font-semibold ${view.result.out_role === 'imposter' ? 'text-mint' : 'text-danger'}`}>
              {view.result.out_role === 'imposter' ? t('game_imposter.reveal.wasImposter') : t('game_imposter.reveal.wasCrew')}
            </Text>
          ) : null}
          {view.result.out_role === 'imposter' && view.result.reason === 'voted' ? (
            <Text className="text-sm text-accent-ink/80">
              {view.result.guess
                ? view.result.guess_right
                  ? t('game_imposter.reveal.guessed', { guess: view.result.guess })
                  : t('game_imposter.reveal.guessedWrong', { guess: view.result.guess })
                : t('game_imposter.reveal.noGuess')}
            </Text>
          ) : null}
          <Animated.View entering={FadeIn.delay(500)} className="items-center gap-1">
            <Text className="text-xs uppercase tracking-[1.5px] text-accent-ink/70">{t('game_imposter.reveal.theWord')}</Text>
            <Text className="text-center text-3xl font-bold text-accent-ink">{view.word}</Text>
          </Animated.View>
          {view.imposters?.length ? (
            <Animated.View entering={FadeIn.delay(900)} className="items-center gap-2">
              <Text className="text-xs uppercase tracking-[1.5px] text-accent-ink/70">
                {view.imposters.length > 1 ? t('game_imposter.reveal.imposterWere') : t('game_imposter.reveal.imposterWas')}
              </Text>
              <View className="flex-row flex-wrap justify-center gap-3">
                {view.imposters.map((id) => (
                  <View key={id} className="items-center gap-1">
                    {avatar(id, 40)}
                    <Text className="text-sm font-semibold text-accent-ink">{name(id)}</Text>
                  </View>
                ))}
              </View>
            </Animated.View>
          ) : null}
          <Animated.View entering={ZoomIn.delay(1300)} className={`rounded-full px-5 py-2 ${view.result.winner === 'crew' ? 'bg-mint' : 'bg-danger'}`}>
            <Text className="text-sm font-bold uppercase tracking-[1.5px] text-white">
              {view.result.winner === 'crew' ? t('game_imposter.reveal.crewWins') : t('game_imposter.reveal.imposterWins')}
            </Text>
          </Animated.View>
        </Animated.View>
      ) : null}

      {/* Last guess */}
      {view.phase === 'guess' && view.result?.out ? (
        <Animated.View entering={ZoomIn.springify().damping(14)} className="items-center gap-3 rounded-3xl bg-danger p-5">
          {avatar(view.result.out, 56)}
          <Text className="text-center text-lg font-bold text-white">{t('game_imposter.guessTitle', { name: name(view.result.out) })}</Text>
          {view.result.out === myId && playing ? (
            <View className="w-full gap-2">
              <Text className="text-center text-sm text-white/90">{t('game_imposter.guessYou')}</Text>
              <Composer placeholder={t('game_imposter.guessPlaceholder')} label={t('game_imposter.guess')} max={CLUE_MAX} onSend={(text) => move({ type: 'guess', text })} />
            </View>
          ) : (
            <Text className="text-sm text-white/90">{t('game_imposter.guessWait')}</Text>
          )}
        </Animated.View>
      ) : null}

      {/* Clues, in turn order */}
      <View className="gap-1">
        <Text variant="label">{t('game_imposter.clues')}</Text>
        {view.order.map((id, i) => {
          const clue = view.clues[i];
          const current = view.phase === 'clue' && view.turn === id;
          return (
            <View key={id} className={`flex-row items-center gap-3 rounded-xl px-3 py-2 ${current ? 'bg-brand-soft' : ''}`}>
              {avatar(id, 28)}
              <Text numberOfLines={1} className="w-24 text-sm font-semibold">
                {name(id)}
              </Text>
              <Text numberOfLines={2} className={`flex-1 text-base ${clue && !clue.skipped ? 'text-ink' : 'italic text-faint'}`}>
                {clue ? (clue.skipped ? t('game_imposter.skipped') : clue.text) : current ? t('game_imposter.waitingClue') : '·'}
              </Text>
            </View>
          );
        })}
        {myTurn ? <Composer placeholder={t('game_imposter.cluePlaceholder')} label={t('game_imposter.send')} max={CLUE_MAX} onSend={(text) => move({ type: 'clue', text })} /> : null}
      </View>

      {/* Vote */}
      {view.phase === 'vote' ? (
        <View className="gap-2">
          <View className="flex-row items-center justify-between">
            <Text variant="label">{t('game_imposter.vote')}</Text>
            <Text variant="caption">{tp('game_imposter.voted', view.voted.length)}</Text>
          </View>
          <View className="flex-row flex-wrap gap-2">
            {view.players
              .filter((id) => id !== myId)
              .map((id) => {
                const mine = myVote === id;
                return (
                  <Pressable
                    key={id}
                    disabled={!playing}
                    onPress={() => vote(id)}
                    accessibilityRole="button"
                    accessibilityState={{ selected: mine }}
                    accessibilityLabel={name(id)}
                    style={{ width: tile }}
                    className={`items-center gap-1 rounded-2xl border-2 p-2 ${mine ? 'border-brand bg-brand-soft' : 'border-line bg-surface'} ${playing ? 'active:opacity-80' : ''}`}
                  >
                    {avatar(id, 44)}
                    <Text numberOfLines={1} className="text-sm font-semibold">
                      {name(id)}
                    </Text>
                    <Text className={`text-xs ${view.voted.includes(id) ? 'text-mint' : 'text-faint'}`}>{view.voted.includes(id) ? t('game_imposter.hasVoted') : ' '}</Text>
                  </Pressable>
                );
              })}
          </View>
          {playing ? (
            <Pressable
              onPress={() => vote('skip')}
              accessibilityRole="button"
              accessibilityState={{ selected: myVote === 'skip' }}
              className={`h-12 items-center justify-center rounded-full border-2 ${myVote === 'skip' ? 'border-brand bg-brand-soft' : 'border-line bg-raised'} active:opacity-80`}
            >
              <Text className="font-semibold">{t('game_imposter.skip')}</Text>
            </Pressable>
          ) : null}
        </View>
      ) : null}

      {/* Chat */}
      {view.phase !== 'clue' ? (
        <View className="gap-1.5">
          <Text variant="label">{t('game_imposter.chat')}</Text>
          {chat.length === 0 ? <Text variant="caption">{t('game_imposter.chatEmpty')}</Text> : null}
          {chat.map((l, i) => (
            <Text key={i} className="text-sm">
              <Text className={`text-sm font-semibold ${l.by === myId ? 'text-brand' : 'text-ink'}`}>{name(l.by)}: </Text>
              {l.text}
            </Text>
          ))}
          {canChat ? <Composer placeholder={t('game_imposter.chatPlaceholder')} label={t('game_imposter.send')} max={SAY_MAX} onSend={(text) => move({ type: 'say', text })} /> : null}
        </View>
      ) : null}

      {/* Scores */}
      <View className="gap-1">
        <Text variant="label">{t('game_imposter.scores')}</Text>
        <View className="flex-row flex-wrap gap-2">
          {[...view.players]
            .sort((a, b) => (view.score[b] ?? 0) - (view.score[a] ?? 0))
            .map((id) => {
              const gained = view.phase === 'reveal' ? view.result?.points?.[id] : undefined;
              return (
                <View key={id} className="flex-row items-center gap-1.5 rounded-full border border-line bg-surface py-1 pl-1 pr-3">
                  {avatar(id, 22)}
                  <Text numberOfLines={1} className="max-w-24 text-sm">
                    {name(id)}
                  </Text>
                  <Text className="text-sm font-bold">{view.score[id] ?? 0}</Text>
                  {gained ? <Text className="text-xs font-bold text-mint">{t('game_imposter.reveal.points', { n: gained })}</Text> : null}
                </View>
              );
            })}
        </View>
      </View>
    </View>
  );
}
