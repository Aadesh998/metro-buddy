import type { Messages } from '..';

const chess: Messages['game_chess'] = {
  white: 'Brancas',
  black: 'Pretas',
  you: 'Você',
  check: 'Xeque!',
  waitingFor: 'Esperando {name}',
  moves: 'Lances',
  noMoves: 'As brancas jogam primeiro',
  promoteTo: 'Promover a',
  pieces: {
    q: 'Dama',
    r: 'Torre',
    b: 'Bispo',
    n: 'Cavalo',
  },
  resign: 'Desistir',
  resignTitle: 'Desistir desta partida?',
  resignBody: 'Seu adversário vence.',
  offerDraw: 'Propor empate',
  drawOffered: 'Empate proposto',
  drawOfferFrom: '{name} propõe empate',
  accept: 'Aceitar',
  decline: 'Recusar',
  result: {
    white: 'Brancas vencem',
    black: 'Pretas vencem',
    draw: 'Empate',
  },
  reason: {
    checkmate: 'Xeque-mate',
    stalemate: 'Afogamento',
    repetition: 'Tripla repetição',
    fifty_moves: 'Regra dos 50 lances',
    insufficient_material: 'Peças insuficientes para dar mate',
    resign: 'Desistência',
    flag: 'Tempo esgotado',
    agreement: 'Empate combinado',
    draw: 'Empate',
  },
};

export default chess;
