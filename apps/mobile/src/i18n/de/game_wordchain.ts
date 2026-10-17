import type { Messages } from '..';

const wordchain: Messages['game_wordchain'] = {
  next: 'Als Nächstes',
  startsWith: 'Beginnt mit {letter}',
  placeholder: 'Ein Wort mit {letter}',
  send: 'Wort senden',
  thinking: '{name} überlegt…',
  you: 'Du',
  out: 'Raus',
  livesLeft_one: 'Noch {count} Leben',
  livesLeft_other: 'Noch {count} Leben',
  attempts: 'Versuch {n} von {max}',
  used: '{n}/{max} Wörter',
  chain: 'Die Kette',
  empty: 'Noch keine Wörter. Das erste beginnt mit {letter}.',
  winner: '{name} bleibt als Letzte(r) übrig!',
  youWon: 'Du bist als Letzte(r) übrig!',
  chainDone: 'Die Kette ist fertig. Die meisten Punkte gewinnen.',
  playedBy: '{word}, gespielt von {name}',
  refusal: {
    letter: 'Muss mit {letter} beginnen',
    used: 'Schon verwendet',
    unknown: 'Nicht in unserer Wortliste',
    letters: 'Nur Buchstaben a bis z',
    short: 'Mindestens 3 Buchstaben',
    long: 'Das Wort ist zu lang',
  },
};

export default wordchain;
