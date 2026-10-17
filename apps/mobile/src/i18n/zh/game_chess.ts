import type { Messages } from '..';

const chess: Messages['game_chess'] = {
  white: '白方',
  black: '黑方',
  you: '你',
  check: '将军！',
  waitingFor: '等待 {name}',
  moves: '着法',
  noMoves: '白方先走',
  promoteTo: '升变为',
  pieces: {
    q: '后',
    r: '车',
    b: '象',
    n: '马',
  },
  resign: '认输',
  resignTitle: '要认输吗？',
  resignBody: '对手将获胜。',
  offerDraw: '提议和棋',
  drawOffered: '已提议和棋',
  drawOfferFrom: '{name} 提议和棋',
  accept: '接受',
  decline: '拒绝',
  result: {
    white: '白方胜',
    black: '黑方胜',
    draw: '和棋',
  },
  reason: {
    checkmate: '将杀',
    stalemate: '逼和',
    repetition: '三次重复局面',
    fifty_moves: '五十步规则',
    insufficient_material: '子力不足以将杀',
    resign: '认输',
    flag: '超时',
    agreement: '协议和棋',
    draw: '和棋',
  },
};

export default chess;
