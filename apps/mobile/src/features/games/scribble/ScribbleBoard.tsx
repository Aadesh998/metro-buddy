import { Check, Eraser, Pencil, Send, Undo2 } from 'lucide-react-native';
import { useEffect, useMemo, useRef, useState } from 'react';
import { Keyboard, Pressable, useWindowDimensions, View, type GestureResponderEvent } from 'react-native';
import Svg, { Path, Rect } from 'react-native-svg';

import type { PersonaRef } from '@/api/types';
import { Avatar } from '@/components/Avatar';
import { FontTextInput } from '@/components/ui/FontTextInput';
import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';
import { colors } from '@/lib/theme';
import { secondsLeft, useLive } from '../api';
import type { BoardProps } from '../types';

type Point = [number, number];
export type ScribbleStroke = { id: number; points: Point[]; color: string; width: number; cont?: boolean };
type FeedItem = { id: number; kind: 'guess' | 'correct' | 'close' | 'chat'; by: string; text?: string };

export type ScribbleView = {
  phase: 'choosing' | 'drawing' | 'reveal' | 'over';
  drawer: string;
  players: string[];
  gone: Record<string, boolean>;
  score: Record<string, number>;
  round: number;
  rounds: number;
  turn: number;
  turns: number;
  ends_at: string | null;
  strokes: ScribbleStroke[];
  feed: FeedItem[];
  guessed: string[];
  turn_points: Record<string, number>;
  palette: string[];
  widths: number[];
  draw_seconds: number;
  hint?: string;
  word?: string;
  choices?: string[];
};

type DrawEvent = { turn: number; op: 'stroke' | 'undo' | 'clear'; stroke?: ScribbleStroke; ids?: number[] };
// A stroke the drawer has sent but not yet seen come back from the server.
type Pending = Omit<ScribbleStroke, 'id'> & { at: number };

// Brush widths are in screen-ish pixels on a phone; the canvas works in 0..1000 units.
const UNIT = 3;
const FLUSH_MS = 250;
const MAX_POINTS = 200;

function pathOf(points: Point[]) {
  if (!points.length) return '';
  // A tap is a dot: nudge the second point so the round cap renders everywhere.
  const pts = points.length === 1 ? [points[0], [points[0][0] + 1, points[0][1]] as Point] : points;
  return pts.map((p, i) => `${i ? 'L' : 'M'}${p[0]} ${p[1]}`).join(' ');
}

function StrokePath({ s }: { s: Pick<ScribbleStroke, 'points' | 'color' | 'width'> }) {
  return <Path d={pathOf(s.points)} stroke={s.color} strokeWidth={s.width * UNIT} strokeLinecap="round" strokeLinejoin="round" fill="none" />;
}

export function ScribbleBoard({ room, view, spectator, myId, move, now }: BoardProps<ScribbleView>) {
  const { t, tp } = useT();
  const { width, height } = useWindowDimensions();
  const isDrawer = !spectator && view.drawer === myId;
  const iGuessed = !!myId && view.guessed.includes(myId);
  const drawing = view.phase === 'drawing';
  const canDraw = isDrawer && drawing;
  const left = secondsLeft(view.ends_at, now);
  const people = useMemo(() => new Map<string, PersonaRef>(room.players.map((p) => [p.persona.id, p.persona])), [room.players]);
  const nameOf = (id: string) => (id === myId ? t('game_scribble.you') : (people.get(id)?.alias ?? '…'));

  // Strokes: the server's copy, with live events applied on top until the next refetch.
  const [strokes, setStrokes] = useState(view.strokes);
  const [pending, setPending] = useState<Pending[]>([]);
  const [current, setCurrent] = useState<Point[]>([]);
  const [base, setBase] = useState({ strokes: view.strokes, turn: view.turn });
  if (base.strokes !== view.strokes || base.turn !== view.turn) {
    // A fresh copy from the server replaces the local one; unacknowledged pieces linger briefly.
    setBase({ strokes: view.strokes, turn: view.turn });
    setStrokes(view.strokes);
    setPending((p) => (base.turn !== view.turn ? [] : p.filter((x) => now - x.at < 4000)));
  }
  useLive<DrawEvent>(room.id, 'draw', (d) => {
    if (d.turn !== view.turn) return;
    if (d.op === 'stroke' && d.stroke) {
      const st = d.stroke;
      setStrokes((cur) => (cur.some((x) => x.id === st.id) ? cur : [...cur, st]));
      if (isDrawer) setPending((p) => p.slice(1));
    } else if (d.op === 'undo') {
      setStrokes((cur) => cur.filter((x) => !d.ids?.includes(x.id)));
    } else if (d.op === 'clear') {
      setStrokes([]);
    }
  });

  const [color, setColor] = useState(view.palette[0]);
  const [brush, setBrush] = useState(view.widths[1] ?? view.widths[0]);

  // Shrink the canvas while the keyboard is open so the guess box stays in view.
  const [kb, setKb] = useState(0);
  useEffect(() => {
    const a = Keyboard.addListener('keyboardDidShow', (e) => setKb(e.endCoordinates.height));
    const b = Keyboard.addListener('keyboardDidHide', () => setKb(0));
    return () => {
      a.remove();
      b.remove();
    };
  }, []);
  const full = Math.min(width - 32, 480);
  const side = kb ? Math.max(120, Math.min(full, height - kb - 330)) : full;

  // The drag in progress: points since the last send, and whether it continues an earlier piece.
  // Pieces go out every FLUSH_MS so others watch the line grow; undo removes the whole drag.
  const drag = useRef<{ pts: Point[]; cont: boolean; last: number } | null>(null);
  const at = (e: GestureResponderEvent): Point => {
    const clamp = (v: number) => Math.max(0, Math.min(1000, Math.round((v * 1000) / side)));
    return [clamp(e.nativeEvent.locationX), clamp(e.nativeEvent.locationY)];
  };
  const flush = (final: boolean) => {
    const d = drag.current;
    if (!d) return;
    // Mid-drag pieces need two points; a final lone tap is sent as a dot.
    if (d.pts.length >= 2 || (final && !d.cont && d.pts.length === 1)) {
      const s = { points: d.pts, color, width: brush, cont: d.cont };
      move({ type: 'stroke', ...s });
      setPending((p) => [...p, { ...s, at: Date.now() }]);
      drag.current = { pts: [d.pts[d.pts.length - 1]], cont: true, last: Date.now() };
    }
    if (final) drag.current = null;
    setCurrent(drag.current ? drag.current.pts : []);
  };
  const touch = canDraw
    ? {
        onStartShouldSetResponder: () => true,
        onStartShouldSetResponderCapture: () => true,
        onMoveShouldSetResponderCapture: () => true,
        onResponderTerminationRequest: () => false,
        onResponderGrant: (e: GestureResponderEvent) => {
          const p = at(e);
          drag.current = { pts: [p], cont: false, last: Date.now() };
          setCurrent([p]);
        },
        onResponderMove: (e: GestureResponderEvent) => {
          const d = drag.current;
          if (!d) return;
          const p = at(e);
          const q = d.pts[d.pts.length - 1];
          if (Math.abs(p[0] - q[0]) + Math.abs(p[1] - q[1]) < 6) return;
          d.pts.push(p);
          if (Date.now() - d.last >= FLUSH_MS || d.pts.length >= MAX_POINTS) flush(false);
          else setCurrent([...d.pts]);
        },
        onResponderRelease: () => flush(true),
        onResponderTerminate: () => flush(true),
      }
    : {};

  const tool = (m: { type: 'undo' | 'clear' }) => {
    move(m);
    setPending([]);
  };

  const [text, setText] = useState('');
  const send = () => {
    const g = text.trim();
    if (!g) return;
    move({ type: 'guess', text: g.slice(0, 40) });
    setText('');
  };

  const drawerName = nameOf(view.drawer);
  const hintLetters = view.hint ? view.hint.replace(/ /g, '').length : 0;

  return (
    <View className="gap-3 px-4">
      {/* Round, status and timer */}
      <View className="flex-row items-center justify-between">
        <Text variant="caption" className="font-semibold">
          {t('game_scribble.round', { n: view.round, max: view.rounds })}
        </Text>
        {left !== null && view.phase !== 'over' ? (
          <Text className={`font-mono text-sm font-semibold ${drawing && left <= 10 ? 'text-danger' : 'text-muted'}`}>{t('games.timeLeft', { s: left })}</Text>
        ) : null}
      </View>

      {/* Scores; the drawer has a pencil, people who got it a tick */}
      <View className="flex-row flex-wrap justify-center gap-x-2 gap-y-1">
        {view.players.map((id) => {
          const p = people.get(id);
          const got = view.guessed.includes(id);
          return (
            <View key={id} className={`w-[38px] items-center ${view.gone[id] ? 'opacity-30' : ''}`}>
              <View>
                <Avatar seed={p?.avatar_seed ?? ''} size={30} deleted={!p} />
                {id === view.drawer || got ? (
                  <View className={`absolute -bottom-1 -right-1 h-4 w-4 items-center justify-center rounded-full ${got ? 'bg-mint' : 'bg-brand'}`}>
                    {got ? <Check size={10} color={colors.surface} /> : <Pencil size={9} color={colors.surface} />}
                  </View>
                ) : null}
              </View>
              <Text numberOfLines={1} className={`text-[11px] font-semibold ${id === myId ? 'text-brand' : 'text-ink'}`}>
                {view.score[id] ?? 0}
              </Text>
            </View>
          );
        })}
      </View>

      {/* What to draw / what to guess */}
      <View className="min-h-12 items-center justify-center">
        {view.phase === 'choosing' ? (
          isDrawer ? (
            <View className="w-full gap-2">
              <Text variant="label" className="text-center">
                {t('game_scribble.pickWord')}
              </Text>
              <View className="flex-row flex-wrap justify-center gap-2">
                {(view.choices ?? []).map((w, i) => (
                  <Pressable
                    key={w}
                    onPress={() => move({ type: 'choose', index: i })}
                    className="rounded-full border border-accent bg-surface px-4 py-2.5 active:bg-raised"
                    accessibilityRole="button"
                  >
                    <Text className="font-semibold">{w}</Text>
                  </Pressable>
                ))}
              </View>
            </View>
          ) : (
            <Text className="text-center text-muted">{t('game_scribble.choosing', { name: drawerName })}</Text>
          )
        ) : view.phase === 'reveal' || view.phase === 'over' ? (
          <View className="items-center">
            <Text variant="caption">{t('game_scribble.wordWas')}</Text>
            <Text variant="title" className="text-center text-brand">
              {view.word}
            </Text>
          </View>
        ) : isDrawer ? (
          <Text className="text-center text-lg">
            {t('game_scribble.drawThis')} <Text className="text-lg font-bold text-brand">{view.word}</Text>
          </Text>
        ) : iGuessed ? (
          <Text className="text-center text-lg font-semibold text-mint">
            {t('game_scribble.youGotIt')} {view.word}
          </Text>
        ) : (
          <View className="items-center">
            <Text className="text-center font-mono text-2xl font-bold tracking-[4px]" accessibilityLabel={tp('game_scribble.letters', hintLetters)}>
              {(view.hint ?? '').split('').map((ch) => (ch === ' ' ? '  ' : ch)).join(' ')}
            </Text>
            <Text variant="caption">
              {tp('game_scribble.letters', hintLetters)} · {t('game_scribble.drawing', { name: drawerName })}
            </Text>
          </View>
        )}
      </View>

      {/* Canvas: a white sheet in 0..1000 units, scaled to fit */}
      <View
        {...touch}
        style={{ width: side, height: side }}
        className="self-center overflow-hidden rounded-2xl border border-line"
        accessibilityLabel={t('game_scribble.canvas')}
      >
        <Svg width={side} height={side} viewBox="0 0 1000 1000" pointerEvents="none">
          <Rect x={0} y={0} width={1000} height={1000} fill="#FFFFFF" />
          {strokes.map((s) => (
            <StrokePath key={s.id} s={s} />
          ))}
          {pending.map((s, i) => (
            <StrokePath key={`p${i}`} s={s} />
          ))}
          {current.length ? <StrokePath s={{ points: current, color, width: brush }} /> : null}
        </Svg>
      </View>

      {canDraw ? (
        <View className="gap-2">
          <View className="flex-row justify-between">
            {view.palette.map((c, i) => (
              <Pressable
                key={c}
                onPress={() => setColor(c)}
                accessibilityRole="button"
                accessibilityState={{ selected: color === c }}
                accessibilityLabel={t('game_scribble.colour', { n: i + 1 })}
                hitSlop={4}
                className={`h-9 w-9 items-center justify-center rounded-full border-2 ${color === c ? 'border-accent' : 'border-transparent'}`}
              >
                <View style={{ backgroundColor: c }} className="h-7 w-7 rounded-full border border-line" />
              </Pressable>
            ))}
          </View>
          <View className="flex-row items-center justify-between">
            <View className="flex-row gap-2">
              {view.widths.map((w) => (
                <Pressable
                  key={w}
                  onPress={() => setBrush(w)}
                  accessibilityRole="button"
                  accessibilityState={{ selected: brush === w }}
                  accessibilityLabel={t('game_scribble.brush', { n: w })}
                  className={`h-10 w-10 items-center justify-center rounded-xl border ${brush === w ? 'border-accent bg-raised' : 'border-line bg-surface'}`}
                >
                  <View style={{ width: w * 2, height: w * 2, borderRadius: w }} className="bg-ink" />
                </Pressable>
              ))}
            </View>
            <View className="flex-row gap-2">
              <Pressable
                onPress={() => tool({ type: 'undo' })}
                accessibilityRole="button"
                accessibilityLabel={t('game_scribble.undo')}
                className="h-10 w-10 items-center justify-center rounded-xl border border-line bg-surface active:bg-raised"
              >
                <Undo2 size={18} color={colors.ink} />
              </Pressable>
              <Pressable
                onPress={() => tool({ type: 'clear' })}
                accessibilityRole="button"
                accessibilityLabel={t('game_scribble.clear')}
                className="h-10 w-10 items-center justify-center rounded-xl border border-line bg-surface active:bg-raised"
              >
                <Eraser size={18} color={colors.danger} />
              </Pressable>
            </View>
          </View>
        </View>
      ) : null}

      {!spectator && !isDrawer && drawing ? (
        <View className="flex-row items-center gap-2">
          <FontTextInput
            value={text}
            onChangeText={setText}
            onSubmitEditing={send}
            submitBehavior="submit"
            returnKeyType="send"
            maxLength={40}
            autoCorrect={false}
            autoCapitalize="none"
            placeholder={iGuessed ? t('game_scribble.chatPlaceholder') : t('game_scribble.guessPlaceholder')}
            placeholderTextColor={colors.faint}
            accessibilityLabel={iGuessed ? t('game_scribble.chatPlaceholder') : t('game_scribble.guessPlaceholder')}
            className="min-h-12 min-w-0 flex-1 rounded-xl border border-line bg-surface px-4 py-3 text-base text-ink"
          />
          <Pressable
            onPress={send}
            disabled={!text.trim()}
            accessibilityRole="button"
            accessibilityLabel={t('game_scribble.send')}
            className={`h-12 w-12 items-center justify-center rounded-xl bg-accent ${text.trim() ? '' : 'opacity-40'}`}
          >
            <Send size={18} color={colors.accentInk} />
          </Pressable>
        </View>
      ) : null}

      {/* Feed, newest first so it sits right under the guess box */}
      {view.phase !== 'choosing' ? (
        <View className="gap-1.5 rounded-2xl border border-line bg-surface p-3">
          {view.feed.length === 0 ? (
            <Text variant="caption">{t('game_scribble.noGuesses')}</Text>
          ) : (
            [...view.feed].reverse().map((m) =>
              m.kind === 'correct' ? (
                <Text key={m.id} className="text-sm font-semibold text-mint">
                  {t('game_scribble.guessedIt', { name: nameOf(m.by) })}
                </Text>
              ) : m.kind === 'close' ? (
                <Text key={m.id} className="text-sm font-semibold text-brand">
                  {t('game_scribble.soClose', { text: m.text ?? '' })}
                </Text>
              ) : (
                <Text key={m.id} className={`text-sm ${m.kind === 'chat' ? 'text-mint' : 'text-ink'}`}>
                  <Text className="text-sm font-semibold">{nameOf(m.by)}: </Text>
                  {m.text}
                  {m.kind === 'chat' ? <Text variant="caption"> · {t('game_scribble.privateNote')}</Text> : null}
                </Text>
              ),
            )
          )}
        </View>
      ) : null}
    </View>
  );
}
