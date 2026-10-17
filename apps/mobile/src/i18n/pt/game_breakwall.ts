import type { Messages } from '..';

const breakwall: Messages['game_breakwall'] = {
  round: 'Rodada {n}/{max}',
  getReady: 'Prepare-se',
  play: 'Vai!',
  roundResults: 'Resultado da rodada',
  finalStandings: 'Classificação final',
  score: 'Pontos',
  balls: 'Bolas',
  time: 'Tempo',
  dragHint: 'Arraste em qualquer lugar do muro para mover a raquete',
  outOfBalls: 'Acabaram as bolas',
  cleared: 'Muro derrubado! +{n} de bônus',
  timeUp: 'Acabou o tempo',
  waiting: 'Esperando os outros…',
  thisRound: 'Esta rodada',
  total: 'Total',
  done: 'Pronto',
  playing: 'Jogando',
  canvas: 'Muro de tijolos. Arraste para mover a raquete.',
};

export default breakwall;
