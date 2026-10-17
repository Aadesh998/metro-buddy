import type { Messages } from '..';

const chess: Messages['game_chess'] = {
  white: 'Белые',
  black: 'Чёрные',
  you: 'Ты',
  check: 'Шах!',
  waitingFor: 'Ждём: {name}',
  moves: 'Ходы',
  noMoves: 'Белые ходят первыми',
  promoteTo: 'Превратить в',
  pieces: {
    q: 'Ферзь',
    r: 'Ладья',
    b: 'Слон',
    n: 'Конь',
  },
  resign: 'Сдаться',
  resignTitle: 'Сдаться в этой партии?',
  resignBody: 'Победа достанется сопернику.',
  offerDraw: 'Предложить ничью',
  drawOffered: 'Ничья предложена',
  drawOfferFrom: '{name} предлагает ничью',
  accept: 'Принять',
  decline: 'Отклонить',
  result: {
    white: 'Белые победили',
    black: 'Чёрные победили',
    draw: 'Ничья',
  },
  reason: {
    checkmate: 'Мат',
    stalemate: 'Пат',
    repetition: 'Троекратное повторение',
    fifty_moves: 'Правило 50 ходов',
    insufficient_material: 'Недостаточно фигур для мата',
    resign: 'Сдался',
    flag: 'Время вышло',
    agreement: 'Ничья по согласию',
    draw: 'Ничья',
  },
};

export default chess;
