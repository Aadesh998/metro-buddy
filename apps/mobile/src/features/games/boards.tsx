import type { ComponentType } from 'react';

import type { GameName } from './api';
import { BreakWallBoard } from './breakwall/BreakWallBoard';
import { ChessBoard } from './chess/ChessBoard';
import { ImposterBoard } from './imposter/ImposterBoard';
import { ScribbleBoard } from './scribble/ScribbleBoard';
import { WordChainBoard } from './wordchain/WordChainBoard';
import { SudokuBoard } from './sudoku/SudokuBoard';
import type { BoardProps } from './types';

// One board per game; a game missing here shows "update the app".
export const BOARDS: Partial<Record<GameName, ComponentType<BoardProps<any>>>> = {
  sudoku: SudokuBoard,
  chess: ChessBoard,
  scribble: ScribbleBoard,
  imposter: ImposterBoard,
  breakwall: BreakWallBoard,
  wordchain: WordChainBoard,
};
