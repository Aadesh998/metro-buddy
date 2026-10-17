import type { Messages } from '..';

const chess: Messages['game_chess'] = {
  white: 'Weiß',
  black: 'Schwarz',
  you: 'Du',
  check: 'Schach!',
  waitingFor: 'Warte auf {name}',
  moves: 'Züge',
  noMoves: 'Weiß zieht zuerst',
  promoteTo: 'Umwandeln in',
  pieces: {
    q: 'Dame',
    r: 'Turm',
    b: 'Läufer',
    n: 'Springer',
  },
  resign: 'Aufgeben',
  resignTitle: 'Partie aufgeben?',
  resignBody: 'Dein Gegner gewinnt.',
  offerDraw: 'Remis bieten',
  drawOffered: 'Remis angeboten',
  drawOfferFrom: '{name} bietet Remis an',
  accept: 'Annehmen',
  decline: 'Ablehnen',
  result: {
    white: 'Weiß gewinnt',
    black: 'Schwarz gewinnt',
    draw: 'Remis',
  },
  reason: {
    checkmate: 'Schachmatt',
    stalemate: 'Patt',
    repetition: 'Dreifache Wiederholung',
    fifty_moves: '50-Züge-Regel',
    insufficient_material: 'Zu wenig Material für Matt',
    resign: 'Aufgegeben',
    flag: 'Zeit abgelaufen',
    agreement: 'Remis vereinbart',
    draw: 'Remis',
  },
};

export default chess;
