import type { Messages } from '..';

const game_breakwall: Messages['game_breakwall'] = {
  round: 'Manche {n}/{max}',
  getReady: 'Prépare-toi',
  play: 'Go !',
  roundResults: 'Résultats de la manche',
  finalStandings: 'Classement final',
  score: 'Score',
  balls: 'Balles',
  time: 'Temps',
  dragHint: 'Fais glisser n’importe où sur le mur pour bouger la raquette',
  outOfBalls: 'Plus de balles',
  cleared: 'Mur détruit ! +{n} de bonus',
  timeUp: 'Temps écoulé',
  waiting: 'En attente des autres…',
  thisRound: 'Cette manche',
  total: 'Total',
  done: 'Terminé',
  playing: 'En jeu',
  canvas: 'Mur de briques. Fais glisser pour bouger la raquette.',
};

export default game_breakwall;
