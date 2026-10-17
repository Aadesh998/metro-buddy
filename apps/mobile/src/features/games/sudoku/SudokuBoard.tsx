import { useState } from 'react';
import { Pressable, useWindowDimensions, View } from 'react-native';

import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';
import { useLive } from '../api';
import type { BoardProps } from '../types';

export type SudokuView = {
  givens: number[];
  board: number[];
  filled_by: number[];
  mistakes: number;
  max_mistakes: number;
  ends_at: string;
  solved: boolean;
  failed: boolean;
  solution?: number[];
};

// Seat colours for who placed a cell; plum first to match the brand.
const SEAT = ['text-brand', 'text-mint', 'text-sky', 'text-danger', 'text-brand', 'text-mint', 'text-sky', 'text-danger'];

export function SudokuBoard({ room, view, spectator, move, now }: BoardProps<SudokuView>) {
  const { t } = useT();
  const { width } = useWindowDimensions();
  const [sel, setSel] = useState<number | null>(null);
  const [flash, setFlash] = useState<{ cell: number; ok: boolean } | null>(null);
  useLive<{ cell: number; correct: boolean }>(room.id, 'cell', (d) => {
    setFlash({ cell: d.cell, ok: d.correct });
    setTimeout(() => setFlash(null), 700);
  });
  // Fit the 9×9 grid to the screen with a 16px gutter on each side.
  const side = Math.min(width - 32, 420);
  const cell = Math.floor(side / 9);
  const over = view.solved || view.failed;
  const left = Math.max(0, Math.ceil((new Date(view.ends_at).getTime() - now) / 1000));
  const shown = over && view.solution ? view.solution : view.board;
  const sameRowColBox = (i: number) =>
    sel !== null && (Math.floor(i / 9) === Math.floor(sel / 9) || i % 9 === sel % 9 || (Math.floor(i / 27) === Math.floor(sel / 27) && Math.floor((i % 9) / 3) === Math.floor((sel % 9) / 3)));

  return (
    <View className="items-center gap-4">
      <View className="w-full flex-row items-center justify-between px-4">
        <Text className={`text-sm font-semibold ${view.mistakes >= view.max_mistakes - 2 ? 'text-danger' : 'text-muted'}`}>
          {t('games.sudoku.mistakes', { n: view.mistakes, max: view.max_mistakes })}
        </Text>
        {!over ? <Text className="font-mono text-sm text-muted">{`${Math.floor(left / 60)}:${String(left % 60).padStart(2, '0')}`}</Text> : null}
      </View>

      <View style={{ width: cell * 9 + 4, height: cell * 9 + 4 }} className="flex-row flex-wrap border-2 border-ink bg-surface">
        {shown.map((v, i) => {
          const r = Math.floor(i / 9);
          const c = i % 9;
          const given = view.givens[i] !== 0;
          const filled = view.board[i] !== 0;
          const selected = sel === i;
          const fl = flash?.cell === i ? (flash.ok ? 'bg-mint/30' : 'bg-danger/30') : '';
          return (
            <Pressable
              key={i}
              disabled={spectator || over || filled}
              onPress={() => setSel(i)}
              accessibilityLabel={`row ${r + 1} column ${c + 1}${v ? `, ${v}` : ''}`}
              style={{ width: cell, height: cell }}
              className={`items-center justify-center border-line ${c % 3 === 2 && c < 8 ? 'border-r-2 border-r-ink' : 'border-r'} ${r % 3 === 2 && r < 8 ? 'border-b-2 border-b-ink' : 'border-b'} ${selected ? 'bg-brand/25' : sameRowColBox(i) ? 'bg-raised' : ''} ${fl}`}
            >
              {v ? (
                <Text
                  style={{ fontSize: cell * 0.5 }}
                  className={given ? 'font-semibold text-ink' : filled ? SEAT[Math.max(0, view.filled_by[i] - 1) % SEAT.length] : 'text-faint'}
                >
                  {v}
                </Text>
              ) : null}
            </Pressable>
          );
        })}
      </View>

      {over ? (
        <Text variant="heading" className={view.solved ? 'text-mint' : 'text-danger'}>
          {view.solved ? t('games.sudoku.solved') : t('games.sudoku.failed')}
        </Text>
      ) : spectator ? null : (
        <View className="w-full gap-2 px-4">
          <Text variant="caption" className="text-center">
            {t('games.sudoku.pickCell')}
          </Text>
          <View className="flex-row justify-between">
            {[1, 2, 3, 4, 5, 6, 7, 8, 9].map((n) => (
              <Pressable
                key={n}
                disabled={sel === null}
                onPress={() => {
                  if (sel === null) return;
                  move({ cell: sel, value: n });
                  setSel(null);
                }}
                style={{ width: Math.min(40, (side - 8 * 4) / 9), height: 48 }}
                className={`items-center justify-center rounded-xl border border-line bg-surface ${sel === null ? 'opacity-40' : 'active:bg-raised'}`}
                accessibilityRole="button"
              >
                <Text className="text-xl font-semibold">{n}</Text>
              </Pressable>
            ))}
          </View>
        </View>
      )}
    </View>
  );
}
