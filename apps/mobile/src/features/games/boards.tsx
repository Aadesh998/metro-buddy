import type { ComponentType } from 'react';

import type { GameName } from './api';
import { SudokuBoard } from './sudoku/SudokuBoard';
import type { BoardProps } from './types';

// One board per game; a game missing here shows "update the app".
export const BOARDS: Partial<Record<GameName, ComponentType<BoardProps<any>>>> = {
  sudoku: SudokuBoard,
};
