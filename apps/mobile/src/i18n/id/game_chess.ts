import type { Messages } from '..';

const chess: Messages['game_chess'] = {
  white: 'Putih',
  black: 'Hitam',
  you: 'Kamu',
  check: 'Skak!',
  waitingFor: 'Menunggu {name}',
  moves: 'Langkah',
  noMoves: 'Putih jalan duluan',
  promoteTo: 'Promosi jadi',
  pieces: {
    q: 'Ratu',
    r: 'Benteng',
    b: 'Gajah',
    n: 'Kuda',
  },
  resign: 'Menyerah',
  resignTitle: 'Menyerah di game ini?',
  resignBody: 'Lawanmu akan menang.',
  offerDraw: 'Tawarkan remis',
  drawOffered: 'Remis ditawarkan',
  drawOfferFrom: '{name} menawarkan remis',
  accept: 'Terima',
  decline: 'Tolak',
  result: {
    white: 'Putih menang',
    black: 'Hitam menang',
    draw: 'Remis',
  },
  reason: {
    checkmate: 'Skakmat',
    stalemate: 'Pat',
    repetition: 'Pengulangan tiga kali',
    fifty_moves: 'Aturan 50 langkah',
    insufficient_material: 'Bidak tidak cukup untuk skakmat',
    resign: 'Menyerah',
    flag: 'Waktu habis',
    agreement: 'Remis disepakati',
    draw: 'Remis',
  },
};

export default chess;
