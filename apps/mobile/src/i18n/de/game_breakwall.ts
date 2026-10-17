import type { Messages } from '..';

const breakwall: Messages['game_breakwall'] = {
  round: 'Runde {n}/{max}',
  getReady: 'Mach dich bereit',
  play: 'Los!',
  roundResults: 'Rundenergebnis',
  finalStandings: 'Endstand',
  score: 'Punkte',
  balls: 'Bälle',
  time: 'Zeit',
  dragHint: 'Zieh irgendwo auf der Mauer, um den Schläger zu bewegen',
  outOfBalls: 'Keine Bälle mehr',
  cleared: 'Mauer geschafft! +{n} Bonus',
  timeUp: 'Zeit ist um',
  waiting: 'Warte auf die anderen…',
  thisRound: 'Diese Runde',
  total: 'Gesamt',
  done: 'Fertig',
  playing: 'Spielt',
  canvas: 'Ziegelmauer. Ziehen, um den Schläger zu bewegen.',
};

export default breakwall;
