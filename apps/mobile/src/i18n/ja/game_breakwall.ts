import type { Messages } from '..';

const game_breakwall: Messages['game_breakwall'] = {
  round: 'ラウンド {n}/{max}',
  getReady: '準備して',
  play: 'スタート！',
  roundResults: 'ラウンド結果',
  finalStandings: '最終順位',
  score: 'スコア',
  balls: 'ボール',
  time: '時間',
  dragHint: '画面のどこでもドラッグしてパドルを動かそう',
  outOfBalls: 'ボールがなくなりました',
  cleared: '全ブロック破壊！ボーナス+{n}',
  timeUp: 'タイムアップ',
  waiting: 'ほかの人を待っています…',
  thisRound: 'このラウンド',
  total: '合計',
  done: '完了',
  playing: 'プレイ中',
  canvas: 'ブロックの壁。ドラッグでパドルを移動。',
};

export default game_breakwall;
