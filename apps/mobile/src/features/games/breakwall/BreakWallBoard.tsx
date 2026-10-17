import { memo, useEffect, useMemo, useRef, useState } from 'react';
import { useWindowDimensions, View } from 'react-native';
import Svg, { Circle, Rect } from 'react-native-svg';

import { Avatar } from '@/components/Avatar';
import { PersonaName } from '@/components/PersonaBadge';
import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';
import { colors } from '@/lib/theme';
import { useLive } from '../api';
import type { BoardProps } from '../types';
import { breakwallLayout, breakwallScore } from './layout';

type Entry = { persona: string; bricks: number; score: number; final: boolean };

export type BreakWallView = {
  round: number;
  rounds: number;
  phase: 'countdown' | 'play' | 'results';
  phase_ends_at: string;
  play_starts_at: string;
  play_ends_at: string;
  total: number;
  players: string[];
  board: Entry[];
  round_scores: Record<string, number>[];
  totals: Record<string, number>;
  done: boolean;
  // Only while the wall is being counted down or played.
  seed?: number;
  layout?: number;
  rows?: number;
  cols?: number;
};

// The wall is simulated in a fixed 300×400 world and scaled to the screen.
const W = 300;
const H = 400;
const TOP = 24;
const SIDE = 6;
const GAP = 3;
const BRICK_H = 14;
const PADDLE_Y = 370;
const PADDLE_W = 56;
const PADDLE_H = 8;
const R = 5;
const SPEED = 250;
const STEP = 1 / 120;
const LIVES = 3;
const SERVE = 0.8;
const REPORT_MS = 2000;

type Rect4 = { x: number; y: number; w: number; h: number };
type Finish = 'balls' | 'clear' | 'time';

function brickRects(cols: number, cells: number[]): Rect4[] {
  const w = (W - SIDE * 2 - GAP * (cols - 1)) / cols;
  return cells.map((i) => ({
    x: SIDE + (i % cols) * (w + GAP),
    y: TOP + Math.floor(i / cols) * (BRICK_H + GAP),
    w,
    h: BRICK_H,
  }));
}

// Rows cycle through the theme accents so the wall reads as stripes.
const rowColor = (row: number) => [colors.brand, colors.mint, colors.sky, colors.danger, colors.ink][row % 5];

// The bricks only change when one breaks, so they don't redraw with every ball frame.
const Bricks = memo(function Bricks({ rects, alive, cells, cols }: { rects: Rect4[]; alive: boolean[]; cells: number[]; cols: number }) {
  return (
    <>
      {rects.map((b, i) =>
        alive[i] ? <Rect key={i} x={b.x} y={b.y} width={b.w} height={b.h} rx={3} fill={rowColor(Math.floor(cells[i] / cols))} fillOpacity={0.85} /> : null,
      )}
    </>
  );
});

type Snap = {
  alive: boolean[];
  bx: number;
  by: number;
  px: number;
  lives: number;
  bricks: number;
  score: number;
  ended: Finish | null;
  bonus: number;
};

type WallProps = {
  round: number;
  seed: number;
  layout: number;
  playStarts: number;
  playEnds: number;
  already?: Entry;
  move: (m: unknown) => void;
  now: number;
  onScore: (score: number) => void;
};

// One round's wall, played locally; mounted fresh per round (keyed), reporting to the server as it goes.
function Wall({ round, seed, layout, playStarts, playEnds, already, move, now, onScore }: WallProps) {
  const { t } = useT();
  const { width } = useWindowDimensions();
  const side = Math.min(width - 32, 420);
  const scale = side / W;
  const wall = useMemo(() => {
    const l = breakwallLayout(seed, layout);
    return { ...l, rects: brickRects(l.cols, l.cells) };
  }, [seed, layout]);
  const total = wall.cells.length;
  const [snap, setSnap] = useState<Snap>(() => ({
    alive: wall.cells.map(() => true),
    bx: W / 2,
    by: PADDLE_Y - R - 1,
    px: W / 2,
    lives: LIVES,
    bricks: already?.bricks ?? 0,
    score: already?.score ?? 0,
    ended: already?.final ? 'time' : null,
    bonus: 0,
  }));

  const target = useRef(W / 2);
  const moveRef = useRef(move);
  const scoreRef = useRef(onScore);
  useEffect(() => {
    moveRef.current = move;
    scoreRef.current = onScore;
  }, [move, onScore]);

  // The paddle follows the finger's x; the wall keeps the touch so the screen doesn't scroll mid-rally.
  // (These are the responder props PanResponder wraps; using them directly keeps refs out of render.)
  const follow = (x: number) => {
    target.current = x / scale;
  };

  // The game loop: fixed 120 Hz physics steps, drawn at most ~60 fps through one state update.
  useEffect(() => {
    if (already?.final) return;
    const s = {
      alive: wall.cells.map(() => true),
      bx: W / 2,
      by: PADDLE_Y - R - 1,
      vx: 0,
      vy: 0,
      px: W / 2,
      lives: LIVES,
      bricks: 0,
      serving: SERVE,
      dir: 1,
      ended: null as Finish | null,
      bonus: 0,
      dirty: true,
    };
    // A rejoin mid-round starts a fresh wall but never reports less than the server already has.
    const floor = already?.score ?? 0;
    let lastSent = 0;
    let sentBricks = already?.bricks ?? 0;
    let lastDraw = 0;
    let acc = 0;
    let prevT = 0;
    let raf = 0;

    const score = () => breakwallScore(s.bricks, total, s.bonus);
    const report = (final: boolean) => {
      const sc = score();
      lastSent = Date.now();
      sentBricks = s.bricks;
      // Too late (the app was in the background) or nothing new: the server would only refuse it.
      if (sc < floor || (!final && s.bricks === 0) || lastSent > playEnds + 2500) return;
      moveRef.current({
        type: 'score',
        round,
        bricks: s.bricks,
        score: sc,
        final,
      });
      scoreRef.current(sc);
    };
    const finish = (kind: Finish) => {
      if (s.ended) return;
      s.ended = kind;
      if (kind === 'clear') s.bonus = Math.max(0, Math.floor((playEnds - Date.now()) / 1000));
      s.dirty = true;
      report(true);
    };

    const step = () => {
      s.px = Math.min(W - PADDLE_W / 2, Math.max(PADDLE_W / 2, target.current));
      if (s.serving > 0) {
        s.bx = s.px;
        s.by = PADDLE_Y - R - 1;
        s.serving -= STEP;
        if (s.serving <= 0) {
          const a = 0.35 * s.dir;
          s.dir = -s.dir;
          s.vx = SPEED * Math.sin(a);
          s.vy = -SPEED * Math.cos(a);
        }
        return;
      }
      const ox = s.bx;
      const oy = s.by;
      s.bx += s.vx * STEP;
      s.by += s.vy * STEP;
      if (s.bx < R) {
        s.bx = R;
        s.vx = Math.abs(s.vx);
      } else if (s.bx > W - R) {
        s.bx = W - R;
        s.vx = -Math.abs(s.vx);
      }
      if (s.by < R) {
        s.by = R;
        s.vy = Math.abs(s.vy);
      }
      // Paddle: the further from the centre it hits, the steeper it flies off (up to ~60°).
      if (s.vy > 0 && s.by + R >= PADDLE_Y && s.by - R <= PADDLE_Y + PADDLE_H && Math.abs(s.bx - s.px) <= PADDLE_W / 2 + R) {
        const a = Math.max(-1, Math.min(1, (s.bx - s.px) / (PADDLE_W / 2))) * 1.05;
        s.vx = SPEED * Math.sin(a);
        s.vy = -SPEED * Math.cos(a);
        s.by = PADDLE_Y - R;
      }
      // One brick per step: whichever side the ball came from decides the bounce.
      for (let i = 0; i < wall.rects.length; i++) {
        if (!s.alive[i]) continue;
        const b = wall.rects[i];
        const cx = Math.max(b.x, Math.min(s.bx, b.x + b.w));
        const cy = Math.max(b.y, Math.min(s.by, b.y + b.h));
        if ((s.bx - cx) ** 2 + (s.by - cy) ** 2 > R * R) continue;
        const fromSide = (ox < b.x || ox > b.x + b.w) && oy >= b.y && oy <= b.y + b.h;
        if (fromSide) s.vx = -s.vx;
        else s.vy = -s.vy;
        s.bx = ox;
        s.by = oy;
        s.alive = s.alive.slice();
        s.alive[i] = false;
        s.bricks++;
        s.dirty = true;
        if (s.bricks === total) finish('clear');
        break;
      }
      if (s.by - R > H) {
        s.lives--;
        s.dirty = true;
        if (s.lives <= 0) finish('balls');
        else s.serving = SERVE;
      }
    };

    const frame = (ts: number) => {
      raf = requestAnimationFrame(frame);
      const dt = prevT ? Math.min(0.1, (ts - prevT) / 1000) : 0;
      prevT = ts;
      const clock = Date.now();
      if (!s.ended && clock >= playStarts) {
        if (clock >= playEnds) finish('time');
        acc += dt;
        while (acc >= STEP && !s.ended) {
          step();
          acc -= STEP;
        }
        if (!s.ended && clock - lastSent >= REPORT_MS && s.bricks !== sentBricks && clock >= playStarts + 1000) report(false);
      } else if (!s.ended) {
        // Before the start the paddle moves, the ball rides on it.
        s.px = Math.min(W - PADDLE_W / 2, Math.max(PADDLE_W / 2, target.current));
        s.bx = s.px;
      }
      if (ts - lastDraw >= 16 || s.dirty) {
        lastDraw = ts;
        s.dirty = false;
        setSnap({
          alive: s.alive,
          bx: s.bx,
          by: s.by,
          px: s.px,
          lives: s.lives,
          bricks: s.bricks,
          score: score(),
          ended: s.ended,
          bonus: s.bonus,
        });
      }
      if (s.ended) cancelAnimationFrame(raf);
    };
    raf = requestAnimationFrame(frame);
    return () => cancelAnimationFrame(raf);
    // Each wall plays once; the component is re-keyed for the next round.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const countdown = Math.ceil((playStarts - now) / 1000);
  const left = Math.max(0, Math.ceil((playEnds - now) / 1000));
  const ended =
    snap.ended === 'clear'
      ? t('game_breakwall.cleared', { n: snap.bonus })
      : snap.ended === 'balls'
        ? t('game_breakwall.outOfBalls')
        : snap.ended
          ? t('game_breakwall.timeUp')
          : null;

  return (
    <View className="items-center gap-2">
      <View style={{ width: side }} className="flex-row items-end justify-between">
        <View>
          <Text variant="caption">{t('game_breakwall.score')}</Text>
          <Text className="font-mono text-xl font-bold">{snap.score}</Text>
        </View>
        <View className="items-center">
          <Text variant="caption">{t('game_breakwall.balls')}</Text>
          <View className="h-7 flex-row items-center gap-1">
            {Array.from({ length: LIVES }, (_, i) => (
              <View key={i} className={`h-3 w-3 rounded-full ${i < snap.lives ? 'bg-ink' : 'bg-line'}`} />
            ))}
          </View>
        </View>
        <View className="items-end">
          <Text variant="caption">{t('game_breakwall.time')}</Text>
          <Text className={`font-mono text-xl font-bold ${left <= 10 && countdown <= 0 ? 'text-danger' : ''}`}>
            {t('games.timeLeft', {
              s: countdown > 0 ? Math.round((playEnds - playStarts) / 1000) : left,
            })}
          </Text>
        </View>
      </View>

      <View
        onStartShouldSetResponder={() => true}
        onStartShouldSetResponderCapture={() => true}
        onMoveShouldSetResponder={() => true}
        onMoveShouldSetResponderCapture={() => true}
        onResponderTerminationRequest={() => false}
        onResponderGrant={(e) => follow(e.nativeEvent.locationX)}
        onResponderMove={(e) => follow(e.nativeEvent.locationX)}
        style={{ width: side, height: (side * H) / W }}
        className="overflow-hidden rounded-2xl border border-line bg-surface"
        accessibilityLabel={t('game_breakwall.canvas')}
      >
        <Svg pointerEvents="none" width={side} height={(side * H) / W} viewBox={`0 0 ${W} ${H}`}>
          <Bricks rects={wall.rects} alive={snap.alive} cells={wall.cells} cols={wall.cols} />
          <Rect x={snap.px - PADDLE_W / 2} y={PADDLE_Y} width={PADDLE_W} height={PADDLE_H} rx={4} fill={colors.accent} />
          {snap.ended ? null : <Circle cx={snap.bx} cy={snap.by} r={R} fill={colors.brand} />}
        </Svg>
        {countdown > 0 || ended ? (
          <View pointerEvents="none" className="absolute inset-0 items-center justify-center bg-bg/60 px-6">
            {countdown > 0 ? (
              countdown > 3 ? (
                <Text variant="heading">{t('game_breakwall.getReady')}</Text>
              ) : (
                <>
                  <Text className="font-mono text-6xl font-bold text-ink">{countdown}</Text>
                  <Text className="mt-2 text-center text-sm text-muted">{t('game_breakwall.dragHint')}</Text>
                </>
              )
            ) : (
              <>
                <Text variant="heading" className={snap.ended === 'clear' ? 'text-mint' : 'text-ink'}>
                  {ended}
                </Text>
                <Text className="mt-2 text-sm text-muted">{t('game_breakwall.waiting')}</Text>
              </>
            )}
          </View>
        ) : null}
      </View>
    </View>
  );
}

export function BreakWallBoard({ room, view, spectator, myId, move, now }: BoardProps<BreakWallView>) {
  const { t } = useT();
  // Scores pushed live between refetches, for this round only.
  const [live, setLive] = useState<{
    round: number;
    scores: Record<string, { score: number; final: boolean }>;
  }>({ round: view.round, scores: {} });
  useLive<{ persona: string; round: number; score: number; final: boolean }>(room.id, 'score', (d) =>
    setLive((prev) => {
      const base = prev.round === d.round ? prev.scores : {};
      return {
        round: d.round,
        scores: { ...base, [d.persona]: { score: d.score, final: d.final } },
      };
    }),
  );
  const [mine, setMine] = useState<{ round: number; score: number } | null>(null);

  const people = useMemo(() => new Map(room.players.map((p) => [p.persona.id, p.persona])), [room.players]);
  const liveScores = live.round === view.round ? live.scores : {};
  const rows = view.board.map((e) => {
    const l = liveScores[e.persona];
    let score = Math.max(e.score, l?.score ?? 0);
    if (e.persona === myId && mine?.round === view.round) score = Math.max(score, mine.score);
    const final = e.final || !!l?.final;
    // Totals already include what the server has for this round.
    const total = (view.totals[e.persona] ?? 0) - e.score + score;
    return { persona: e.persona, score, final, total };
  });
  const ranked =
    view.phase === 'results' ? [...rows].sort((a, b) => (view.done ? b.total - a.total : b.score - a.score)) : [...rows].sort((a, b) => b.total - a.total);
  const me = view.board.find((e) => e.persona === myId);
  const playing = !spectator && !!me && view.seed !== undefined && view.layout !== undefined && view.phase !== 'results';
  const title = view.done
    ? t('game_breakwall.finalStandings')
    : view.phase === 'results'
      ? t('game_breakwall.roundResults')
      : view.phase === 'countdown'
        ? t('game_breakwall.getReady')
        : t('game_breakwall.play');

  return (
    <View className="gap-4 px-4">
      <View className="flex-row items-center justify-between">
        <Text variant="label">{t('game_breakwall.round', { n: view.round, max: view.rounds })}</Text>
        <Text variant="caption">{title}</Text>
      </View>

      {playing ? (
        <Wall
          key={`${view.round}-${view.seed}`}
          round={view.round}
          seed={view.seed!}
          layout={view.layout!}
          playStarts={new Date(view.play_starts_at).getTime()}
          playEnds={new Date(view.play_ends_at).getTime()}
          already={me}
          move={move}
          now={now}
          onScore={(score) => setMine({ round: view.round, score })}
        />
      ) : null}

      <View className="rounded-2xl border border-line bg-surface px-3 py-1">
        <View className="flex-row items-center gap-3 border-b border-line py-2">
          <Text variant="caption" className="flex-1">
            {view.done ? t('game_breakwall.finalStandings') : view.phase === 'results' ? t('game_breakwall.roundResults') : t('game_breakwall.thisRound')}
          </Text>
          <Text variant="caption" className="w-14 text-right">
            {t('game_breakwall.thisRound')}
          </Text>
          <Text variant="caption" className="w-14 text-right">
            {t('game_breakwall.total')}
          </Text>
        </View>
        {ranked.map((r, i) => {
          const p = people.get(r.persona);
          return (
            <View key={r.persona} className="flex-row items-center gap-3 py-2">
              <Text className="w-4 text-sm font-semibold text-muted">{i + 1}</Text>
              <Avatar seed={p?.avatar_seed ?? ''} size={28} deleted={!p} />
              <View className="min-w-0 flex-1">
                {p ? <PersonaName p={p} /> : <Text className="text-sm text-faint">—</Text>}
                {view.phase === 'play' ? (
                  <Text className={`text-xs ${r.final ? 'text-mint' : 'text-faint'}`}>{r.final ? t('game_breakwall.done') : t('game_breakwall.playing')}</Text>
                ) : null}
              </View>
              <Text className={`w-14 text-right font-mono text-sm ${r.persona === myId ? 'font-bold text-brand' : 'text-ink'}`}>{r.score}</Text>
              <Text className="w-14 text-right font-mono text-sm font-semibold">{r.total}</Text>
            </View>
          );
        })}
      </View>
    </View>
  );
}
