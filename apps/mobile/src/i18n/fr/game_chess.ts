import type { Messages } from '..';

const chess: Messages['game_chess'] = {
  white: 'Blancs',
  black: 'Noirs',
  you: 'Toi',
  check: 'Échec !',
  waitingFor: 'En attente de {name}',
  moves: 'Coups',
  noMoves: 'Les blancs jouent en premier',
  promoteTo: 'Promouvoir en',
  pieces: {
    q: 'Dame',
    r: 'Tour',
    b: 'Fou',
    n: 'Cavalier',
  },
  resign: 'Abandonner',
  resignTitle: 'Abandonner la partie ?',
  resignBody: 'Ton adversaire gagne.',
  offerDraw: 'Proposer nulle',
  drawOffered: 'Nulle proposée',
  drawOfferFrom: '{name} propose la nulle',
  accept: 'Accepter',
  decline: 'Refuser',
  result: {
    white: 'Les blancs gagnent',
    black: 'Les noirs gagnent',
    draw: 'Partie nulle',
  },
  reason: {
    checkmate: 'Échec et mat',
    stalemate: 'Pat',
    repetition: 'Triple répétition',
    fifty_moves: 'Règle des 50 coups',
    insufficient_material: 'Matériel insuffisant pour mater',
    resign: 'Abandon',
    flag: 'Temps écoulé',
    agreement: 'Nulle d’un commun accord',
    draw: 'Nulle',
  },
};

export default chess;
