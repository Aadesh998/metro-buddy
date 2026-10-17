import { createElement } from 'react';
import { BrickWall, Brush, Crown, Grid3x3, Link2, VenetianMask, type LucideIcon } from 'lucide-react-native';

import type { GameName } from './api';

// Display order and icon for each game; the server's /v1/games list decides what is playable.
export const GAMES: { name: GameName; icon: LucideIcon }[] = [
  { name: 'scribble', icon: Brush },
  { name: 'imposter', icon: VenetianMask },
  { name: 'wordchain', icon: Link2 },
  { name: 'sudoku', icon: Grid3x3 },
  { name: 'chess', icon: Crown },
  { name: 'breakwall', icon: BrickWall },
];

const icons = Object.fromEntries(GAMES.map((g) => [g.name, g.icon])) as Record<GameName, LucideIcon>;

export function GameIcon({ name, size, color }: { name: GameName; size: number; color: string }) {
  return createElement(icons[name] ?? Grid3x3, { size, color });
}
