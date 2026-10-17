import type { Messages } from '..';

const game_breakwall: Messages['game_breakwall'] = {
  round: 'Раунд {n}/{max}',
  getReady: 'Приготовься',
  play: 'Вперёд!',
  roundResults: 'Итоги раунда',
  finalStandings: 'Итоговая таблица',
  score: 'Счёт',
  balls: 'Мячи',
  time: 'Время',
  dragHint: 'Веди пальцем по стене, чтобы двигать платформу',
  outOfBalls: 'Мячи закончились',
  cleared: 'Стена разбита! Бонус +{n}',
  timeUp: 'Время вышло',
  waiting: 'Ждём остальных…',
  thisRound: 'Этот раунд',
  total: 'Всего',
  done: 'Готово',
  playing: 'Играет',
  canvas: 'Кирпичная стена. Веди пальцем, чтобы двигать платформу.',
};

export default game_breakwall;
