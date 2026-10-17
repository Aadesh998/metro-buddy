import { Heart, SendHorizontal } from 'lucide-react-native';
import { useState } from 'react';
import { Pressable, View } from 'react-native';

import { Avatar } from '@/components/Avatar';
import { PersonaName } from '@/components/PersonaBadge';
import { FontTextInput } from '@/components/ui/FontTextInput';
import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';
import { colors } from '@/lib/theme';
import { secondsLeft, useLive } from '../api';
import type { BoardProps } from '../types';

type Link = { word: string; by: string };
type RefusalCode = 'letter' | 'used' | 'unknown' | 'letters' | 'short' | 'long';

export type WordChainView = {
  players: string[];
  chain: Link[];
  letter: string;
  turn: string;
  turn_ends_at: string;
  turn_seconds: number;
  round: number;
  lives: Record<string, number>;
  max_lives: number;
  out: string[];
  used: number;
  max_words: number;
  score: Record<string, number>;
  over: boolean;
  winner: string | null;
  // Only for the player whose turn it is.
  strikes?: number;
  max_strikes?: number;
  refusal?: string;
  refusal_code?: RefusalCode;
};

// Seat colours, plum first to match the brand; they repeat past four players.
const SEAT = [
  { pill: 'border-brand/40 bg-brand/10', text: 'text-brand', bar: 'bg-brand', dot: colors.brand },
  { pill: 'border-mint/40 bg-mint/10', text: 'text-mint', bar: 'bg-mint', dot: colors.mint },
  { pill: 'border-sky/40 bg-sky/10', text: 'text-sky', bar: 'bg-sky', dot: colors.sky },
  { pill: 'border-danger/40 bg-danger/10', text: 'text-danger', bar: 'bg-danger', dot: colors.danger },
];

export function WordChainBoard({ room, view, spectator, myId, move, moving, now }: BoardProps<WordChainView>) {
  const { t, tp } = useT();
  const [text, setText] = useState('');
  // Words and refusals pushed live, tagged with the view they arrived on so a refetch drops them.
  const [live, setLive] = useState<(Link & { at: number })[]>([]);
  // Typing again hides the last refusal until the next attempt.
  const [editing, setEditing] = useState(false);
  const [refused, setRefused] = useState<{ code: RefusalCode; at: string } | null>(null);
  const stamp = `${view.turn}:${view.used}`;
  useLive<{ word: string; by: string }>(room.id, 'word', (d) => setLive((p) => [...p, { word: d.word, by: d.by, at: view.used }]));
  useLive<{ code: RefusalCode }>(room.id, 'refused', (d) => setRefused({ code: d.code, at: stamp }));
  const pending = live.filter((l) => l.at === view.used && !view.chain.some((c) => c.word === l.word));

  const persona = (id: string) => room.players.find((p) => p.persona.id === id)?.persona;
  const seat = (id: string) => SEAT[Math.max(0, view.players.indexOf(id)) % SEAT.length];
  const alias = (id: string) => (id === myId ? t('game_wordchain.you') : (persona(id)?.alias ?? '…'));

  const chain = [...view.chain, ...pending];
  const letter = (pending.length ? pending[pending.length - 1].word.slice(-1) : view.letter).toUpperCase();
  const myTurn = !spectator && !view.over && view.turn === myId && !pending.length;
  const left = secondsLeft(view.turn_ends_at, now) ?? 0;
  const frac = Math.min(1, left / Math.max(1, view.turn_seconds));
  const turnPersona = persona(view.turn);

  const code = editing ? undefined : refused?.at === stamp ? refused.code : view.refusal_code;
  const reason = code ? t(`game_wordchain.refusal.${code}`, { letter }) : null;

  const send = () => {
    const w = text.trim();
    if (!w || moving) return;
    move({ type: 'word', text: w });
    setText('');
    setEditing(false);
  };

  return (
    <View className="gap-4 px-4">
      {view.over ? (
        <View className="items-center gap-1 rounded-2xl border border-line bg-surface p-4">
          <Text variant="heading" className="text-center">
            {view.winner
              ? view.winner === myId
                ? t('game_wordchain.youWon')
                : t('game_wordchain.winner', { name: persona(view.winner)?.alias ?? '…' })
              : t('game_wordchain.chainDone')}
          </Text>
        </View>
      ) : (
        <View className="gap-3 rounded-2xl border border-line bg-surface p-4">
          <View className="flex-row items-center gap-3">
            {turnPersona ? <Avatar seed={turnPersona.avatar_seed} size={36} /> : null}
            <View className="min-w-0 flex-1">
              {view.turn === myId ? (
                <Text className="text-sm font-semibold text-ink">{t('games.yourTurn')}</Text>
              ) : turnPersona ? (
                <PersonaName p={turnPersona} />
              ) : null}
              <Text variant="caption">{t('game_wordchain.used', { n: view.used, max: view.max_words })}</Text>
            </View>
            <Text className={`font-mono text-base font-semibold ${left <= 3 ? 'text-danger' : 'text-muted'}`}>{t('games.timeLeft', { s: left })}</Text>
          </View>
          <View className="h-1.5 overflow-hidden rounded-full bg-raised">
            <View style={{ width: `${frac * 100}%` }} className={`h-full rounded-full ${left <= 3 ? 'bg-danger' : seat(view.turn).bar}`} />
          </View>

          <View className="flex-row items-baseline justify-center gap-2 py-1">
            <Text className="text-lg text-muted">{t('game_wordchain.next')}:</Text>
            <Text accessibilityLabel={t('game_wordchain.startsWith', { letter })} className="text-5xl font-bold text-ink">
              {letter}…
            </Text>
          </View>

          {/* The input sits near the top of the screen so the keyboard never covers it. */}
          {myTurn ? (
            <View className="gap-1.5">
              <View className="flex-row items-center gap-2">
                <FontTextInput
                  value={text}
                  onChangeText={(s) => {
                    setText(s);
                    setEditing(true);
                  }}
                  onSubmitEditing={send}
                  submitBehavior="submit"
                  returnKeyType="send"
                  autoCapitalize="none"
                  autoCorrect={false}
                  autoComplete="off"
                  spellCheck={false}
                  autoFocus
                  maxLength={30}
                  placeholder={t('game_wordchain.placeholder', { letter })}
                  placeholderTextColor={colors.faint}
                  accessibilityLabel={t('game_wordchain.placeholder', {
                    letter,
                  })}
                  className={`min-h-12 min-w-0 flex-1 rounded-xl border bg-bg px-4 py-3 text-base text-ink ${reason ? 'border-danger' : 'border-line'}`}
                />
                <Pressable
                  onPress={send}
                  disabled={!text.trim() || moving}
                  accessibilityRole="button"
                  accessibilityLabel={t('game_wordchain.send')}
                  className={`h-12 w-12 items-center justify-center rounded-xl bg-accent ${!text.trim() || moving ? 'opacity-40' : 'active:opacity-80'}`}
                >
                  <SendHorizontal size={20} color={colors.accentInk} />
                </Pressable>
              </View>
              {reason ? (
                <Text accessibilityLiveRegion="polite" className="text-sm text-danger">
                  {reason}
                  {view.strikes && view.max_strikes ? ` · ${t('game_wordchain.attempts', { n: view.strikes, max: view.max_strikes })}` : ''}
                </Text>
              ) : null}
            </View>
          ) : (
            <Text className="text-center text-muted">{t('game_wordchain.thinking', { name: alias(view.turn) })}</Text>
          )}
        </View>
      )}

      <View className="gap-2">
        {view.players.map((id) => {
          const p = persona(id);
          const out = view.out.includes(id);
          const lives = view.lives[id] ?? 0;
          return (
            <View key={id} className={`flex-row items-center gap-2.5 ${out ? 'opacity-40' : ''}`}>
              <View style={{ backgroundColor: seat(id).dot }} className="h-2.5 w-2.5 rounded-full" />
              {p ? <Avatar seed={p.avatar_seed} size={24} /> : null}
              <Text numberOfLines={1} className={`min-w-0 flex-1 text-sm ${id === view.turn && !view.over ? 'font-semibold text-ink' : 'text-ink'}`}>
                {alias(id)}
              </Text>
              {out ? (
                <Text className="text-xs font-semibold text-muted">{t('game_wordchain.out')}</Text>
              ) : (
                <View className="flex-row gap-0.5" accessibilityLabel={tp('game_wordchain.livesLeft', lives)}>
                  {Array.from({ length: view.max_lives }, (_, i) => (
                    <Heart key={i} size={14} color={i < lives ? colors.danger : colors.faint} fill={i < lives ? colors.danger : 'transparent'} />
                  ))}
                </View>
              )}
              <Text className="w-12 text-right font-mono text-sm font-semibold text-ink">{view.score[id] ?? 0}</Text>
            </View>
          );
        })}
      </View>

      <View className="gap-2">
        <Text variant="label">{t('game_wordchain.chain')}</Text>
        {chain.length ? (
          <View className="flex-row flex-wrap gap-1.5">
            {chain.map((l, i) => {
              const newest = i === chain.length - 1;
              const s = seat(l.by);
              return (
                <View
                  key={`${i}-${l.word}`}
                  accessibilityLabel={t('game_wordchain.playedBy', { word: l.word, name: alias(l.by) })}
                  className={`rounded-full border ${s.pill} ${newest ? 'border-2 px-3.5 py-1.5' : 'px-2.5 py-1'}`}
                >
                  <Text className={`${newest ? 'text-lg font-bold' : 'text-sm font-medium'} ${s.text}`}>{l.word}</Text>
                </View>
              );
            })}
          </View>
        ) : (
          <Text variant="caption">{t('game_wordchain.empty', { letter })}</Text>
        )}
      </View>
    </View>
  );
}
