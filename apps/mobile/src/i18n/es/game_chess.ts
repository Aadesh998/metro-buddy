import type { Messages } from '..';

const chess: Messages['game_chess'] = {
  white: 'Blancas',
  black: 'Negras',
  you: 'Tú',
  check: '¡Jaque!',
  waitingFor: 'Esperando a {name}',
  moves: 'Jugadas',
  noMoves: 'Las blancas mueven primero',
  promoteTo: 'Coronar a',
  pieces: {
    q: 'Dama',
    r: 'Torre',
    b: 'Alfil',
    n: 'Caballo',
  },
  resign: 'Rendirse',
  resignTitle: '¿Rendirte en esta partida?',
  resignBody: 'Tu rival gana.',
  offerDraw: 'Ofrecer tablas',
  drawOffered: 'Tablas ofrecidas',
  drawOfferFrom: '{name} ofrece tablas',
  accept: 'Aceptar',
  decline: 'Rechazar',
  result: {
    white: 'Ganan las blancas',
    black: 'Ganan las negras',
    draw: 'Tablas',
  },
  reason: {
    checkmate: 'Jaque mate',
    stalemate: 'Ahogado',
    repetition: 'Triple repetición',
    fifty_moves: 'Regla de los 50 movimientos',
    insufficient_material: 'Material insuficiente para dar mate',
    resign: 'Abandono',
    flag: 'Sin tiempo',
    agreement: 'Tablas acordadas',
    draw: 'Tablas',
  },
};

export default chess;
