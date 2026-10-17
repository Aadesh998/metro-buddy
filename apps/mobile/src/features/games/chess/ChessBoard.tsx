import { useState } from 'react';
import { Alert, Pressable, ScrollView, useWindowDimensions, View } from 'react-native';

import type { PersonaRef } from '@/api/types';
import { Avatar } from '@/components/Avatar';
import { PersonaName } from '@/components/PersonaBadge';
import { Button } from '@/components/ui/Button';
import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';
import { useLive } from '../api';
import type { BoardProps } from '../types';
import { ChessPiece } from './pieces';

type Color = 'white' | 'black';

export type ChessView = {
  fen: string;
  moves: string[];
  uci: string[];
  last: [string, string] | null;
  you: Color | 'spectator';
  turn: Color;
  check: boolean;
  legal: string[];
  clocks: { white_ms: number; black_ms: number; running_since: string | null };
  draw_offer: Color | null;
  result: Color | 'draw' | null;
  reason: Reason | null;
  white: string;
  black: string;
};

type Reason = 'checkmate' | 'stalemate' | 'repetition' | 'fifty_moves' | 'insufficient_material' | 'resign' | 'flag' | 'agreement' | 'draw';

type LiveMove = { uci: string; san: string; fen: string; ply: number };

const PROMOS = ['q', 'r', 'b', 'n'] as const;
const FILES = 'abcdefgh';

// Piece letters by square ("e4" → "P"), from the placement field of a FEN.
function parseFen(fen: string) {
  const out: Record<string, string> = {};
  fen
    .split(' ')[0]
    .split('/')
    .forEach((row, i) => {
      let f = 0;
      for (const ch of row) {
        if (ch >= '1' && ch <= '8') f += Number(ch);
        else out[`${FILES[f++]}${8 - i}`] = ch;
      }
    });
  return out;
}

function clockText(ms: number) {
  const s = Math.max(0, Math.ceil(ms / 1000));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

export function ChessBoard({ room, view, spectator, move, moving, now }: BoardProps<ChessView>) {
  const { t } = useT();
  const { width } = useWindowDimensions();
  const [sel, setSel] = useState<string | null>(null);
  const [promo, setPromo] = useState<(typeof PROMOS)[number]>('q');
  // A live move shows up before the refetch; it wins only while it's newer than the view.
  const [live, setLive] = useState<LiveMove | null>(null);
  useLive<LiveMove>(room.id, 'move', (d) => {
    setLive(d);
    setSel(null);
  });
  const fresh = live && live.ply > view.uci.length ? live : null;
  const fen = fresh ? fresh.fen : view.fen;
  const last = fresh ? [fresh.uci.slice(0, 2), fresh.uci.slice(2, 4)] : view.last;
  const turn: Color = fresh ? (fen.split(' ')[1] === 'w' ? 'white' : 'black') : view.turn;
  const over = view.result !== null;

  const me = view.you === 'spectator' ? null : view.you;
  const bottom: Color = me ?? 'white';
  const top: Color = bottom === 'white' ? 'black' : 'white';
  const myTurn = !spectator && !over && !fresh && me === turn;
  const legal = myTurn ? view.legal : [];
  const pieces = parseFen(fen);

  const side = Math.min(width - 32, 420);
  const sq = Math.floor(side / 8);
  const ranks = bottom === 'white' ? [8, 7, 6, 5, 4, 3, 2, 1] : [1, 2, 3, 4, 5, 6, 7, 8];
  const files = bottom === 'white' ? FILES.split('') : FILES.split('').reverse();

  const targets = new Set(sel ? legal.filter((m) => m.startsWith(sel)).map((m) => m.slice(2, 4)) : []);
  const promoting = sel !== null && legal.some((m) => m.startsWith(sel) && m.length === 5);
  const kingInCheck = (fresh ? fresh.san.endsWith('+') : view.check) ? Object.keys(pieces).find((s) => pieces[s] === (turn === 'white' ? 'K' : 'k')) : undefined;

  const tap = (s: string) => {
    if (!myTurn) return;
    if (sel && targets.has(s)) {
      const uci = `${sel}${s}`;
      move({ type: 'move', uci: legal.includes(uci) ? uci : `${uci}${promo}` });
      setSel(null);
      return;
    }
    const p = pieces[s];
    const mine = p && (p === p.toUpperCase()) === (me === 'white');
    setSel(mine && s !== sel ? s : null);
  };

  const player = (c: Color) => room.players.find((p) => p.persona.id === (c === 'white' ? view.white : view.black))?.persona;
  const remaining = (c: Color) => {
    const ms = c === 'white' ? view.clocks.white_ms : view.clocks.black_ms;
    const since = view.clocks.running_since;
    return !over && since && c === view.turn ? ms - (now - new Date(since).getTime()) : ms;
  };

  const bar = (c: Color) => {
    const p: PersonaRef | undefined = player(c);
    const running = !over && c === turn;
    const ms = remaining(c);
    return (
      <View className="w-full flex-row items-center gap-2.5 px-4">
        {p ? <Avatar seed={p.avatar_seed} size={32} /> : null}
        <View className="flex-1">
          {p ? <PersonaName p={p} /> : null}
          <Text variant="caption">{c === me ? `${t(`game_chess.${c}`)} · ${t('game_chess.you')}` : t(`game_chess.${c}`)}</Text>
        </View>
        <View className={`rounded-lg px-2.5 py-1 ${running ? (ms < 30_000 ? 'bg-danger' : 'bg-accent') : 'bg-raised'}`}>
          <Text className={`font-mono text-base font-semibold ${running ? (ms < 30_000 ? 'text-white' : 'text-accent-ink') : 'text-muted'}`}>{clockText(ms)}</Text>
        </View>
      </View>
    );
  };

  const resign = () =>
    Alert.alert(t('game_chess.resignTitle'), t('game_chess.resignBody'), [
      { text: t('common.cancel'), style: 'cancel' },
      { text: t('game_chess.resign'), style: 'destructive', onPress: () => move({ type: 'resign' }) },
    ]);

  const opponent = me ? player(me === 'white' ? 'black' : 'white') : undefined;
  const offeredToMe = me !== null && view.draw_offer !== null && view.draw_offer !== me;

  return (
    <View className="items-center gap-3">
      {bar(top)}

      <View style={{ width: sq * 8, height: sq * 8 }} className="flex-row flex-wrap overflow-hidden rounded-md bg-raised">
        {ranks.map((r, ri) =>
          files.map((f, fi) => {
            const s = `${f}${r}`;
            const p = pieces[s];
            const isDark = (ri + fi) % 2 === 1;
            const isWhite = p !== undefined && p === p.toUpperCase();
            const hl = s === sel ? 'bg-brand/45' : s === kingInCheck ? 'bg-danger/50' : last && (s === last[0] || s === last[1]) ? 'bg-brand/25' : '';
            return (
              <Pressable
                key={s}
                onPress={() => tap(s)}
                disabled={!myTurn}
                accessibilityLabel={p ? `${s} ${p}` : s}
                style={{ width: sq, height: sq }}
                className={isDark ? 'bg-faint/45' : ''}
              >
                <View className={`flex-1 items-center justify-center ${hl}`}>
                  {targets.has(s) ? (
                    p ? (
                      <View className="absolute inset-0.5 rounded-full border-[3px] border-ink/30" />
                    ) : (
                      <View style={{ width: sq * 0.28, height: sq * 0.28 }} className="rounded-full bg-ink/30" />
                    )
                  ) : null}
                  {p ? <ChessPiece kind={p.toLowerCase() as 'k' | 'q' | 'r' | 'b' | 'n' | 'p'} white={isWhite} size={sq * 0.86} /> : null}
                  {fi === 0 ? <Text className="absolute left-0.5 top-0 text-[9px] text-muted">{r}</Text> : null}
                  {ri === 7 ? <Text className="absolute bottom-0 right-0.5 text-[9px] text-muted">{f}</Text> : null}
                </View>
              </Pressable>
            );
          }),
        )}
      </View>

      {bar(bottom)}

      {over ? (
        <View className="items-center">
          <Text variant="heading">{t(`game_chess.result.${view.result ?? 'draw'}`)}</Text>
          {view.reason ? <Text variant="caption">{t(`game_chess.reason.${view.reason}`)}</Text> : null}
        </View>
      ) : (
        <Text className={`text-sm font-semibold ${kingInCheck ? 'text-danger' : myTurn ? 'text-brand' : 'text-muted'}`}>
          {kingInCheck ? t('game_chess.check') : myTurn ? t('games.yourTurn') : t('game_chess.waitingFor', { name: player(turn)?.alias ?? t(`game_chess.${turn}`) })}
        </Text>
      )}

      {promoting ? (
        <View className="flex-row items-center gap-2">
          <Text variant="caption">{t('game_chess.promoteTo')}</Text>
          {PROMOS.map((k) => (
            <Pressable
              key={k}
              onPress={() => setPromo(k)}
              accessibilityRole="button"
              accessibilityState={{ selected: promo === k }}
              accessibilityLabel={t(`game_chess.pieces.${k}`)}
              className={`h-11 w-11 items-center justify-center rounded-xl border ${promo === k ? 'border-brand bg-brand-soft' : 'border-line bg-surface'}`}
            >
              <ChessPiece kind={k} white={me !== 'black'} size={32} />
            </Pressable>
          ))}
        </View>
      ) : null}

      <View className="w-full gap-1.5">
        <Text variant="caption" className="px-4">
          {t('game_chess.moves')}
        </Text>
        <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerClassName="gap-3 px-4">
          {view.moves.length === 0 ? <Text className="text-sm text-faint">{t('game_chess.noMoves')}</Text> : null}
          {view.moves.map((san, i) =>
            i % 2 === 0 ? (
              <Text key={i} className="font-mono text-sm text-ink">
                <Text className="font-mono text-sm text-faint">{`${i / 2 + 1}. `}</Text>
                {san}
                {view.moves[i + 1] ? ` ${view.moves[i + 1]}` : ''}
              </Text>
            ) : null,
          )}
        </ScrollView>
      </View>

      {me && !over ? (
        offeredToMe ? (
          <View className="w-full gap-2 px-4">
            <Text className="text-center text-sm font-semibold text-ink">{t('game_chess.drawOfferFrom', { name: opponent?.alias ?? '' })}</Text>
            <View className="flex-row gap-2">
              <Button title={t('game_chess.accept')} className="flex-1" disabled={moving} onPress={() => move({ type: 'draw_accept' })} />
              <Button title={t('game_chess.decline')} kind="secondary" className="flex-1" disabled={moving} onPress={() => move({ type: 'draw_decline' })} />
            </View>
          </View>
        ) : (
          <View className="w-full flex-row gap-2 px-4">
            <Button
              title={view.draw_offer === me ? t('game_chess.drawOffered') : t('game_chess.offerDraw')}
              kind="secondary"
              className="flex-1 px-3"
              disabled={moving || view.draw_offer === me}
              onPress={() => move({ type: 'draw_offer' })}
            />
            <Button title={t('game_chess.resign')} kind="secondary" className="flex-1 px-3" disabled={moving} onPress={resign} />
          </View>
        )
      ) : null}
    </View>
  );
}
