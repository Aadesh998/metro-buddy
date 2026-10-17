import type { Messages } from '..';

const chess: Messages['game_chess'] = {
  white: '白',
  black: '黒',
  you: 'あなた',
  check: 'チェック！',
  waitingFor: '{name}を待っています',
  moves: '棋譜',
  noMoves: '白が先手です',
  promoteTo: '昇格先',
  pieces: {
    q: 'クイーン',
    r: 'ルーク',
    b: 'ビショップ',
    n: 'ナイト',
  },
  resign: '投了',
  resignTitle: 'この対局を投了する？',
  resignBody: '相手の勝ちになります。',
  offerDraw: '引き分けを提案',
  drawOffered: '引き分けを提案中',
  drawOfferFrom: '{name}が引き分けを提案しています',
  accept: '受ける',
  decline: '断る',
  result: {
    white: '白の勝ち',
    black: '黒の勝ち',
    draw: '引き分け',
  },
  reason: {
    checkmate: 'チェックメイト',
    stalemate: 'ステイルメイト',
    repetition: '同形3回',
    fifty_moves: '50手ルール',
    insufficient_material: 'メイトに必要な駒がない',
    resign: '投了',
    flag: '時間切れ',
    agreement: '合意による引き分け',
    draw: '引き分け',
  },
};

export default chess;
