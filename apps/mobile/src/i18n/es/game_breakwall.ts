import type { Messages } from '..';

const game_breakwall: Messages['game_breakwall'] = {
  round: 'Ronda {n}/{max}',
  getReady: 'Prepárate',
  play: '¡Ya!',
  roundResults: 'Resultados de la ronda',
  finalStandings: 'Clasificación final',
  score: 'Puntos',
  balls: 'Bolas',
  time: 'Tiempo',
  dragHint: 'Arrastra en cualquier parte del muro para mover la paleta',
  outOfBalls: 'Sin bolas',
  cleared: '¡Muro despejado! +{n} de bonus',
  timeUp: '¡Se acabó el tiempo!',
  waiting: 'Esperando a los demás…',
  thisRound: 'Esta ronda',
  total: 'Total',
  done: 'Terminado',
  playing: 'Jugando',
  canvas: 'Muro de ladrillos. Arrastra para mover la paleta.',
};

export default game_breakwall;
