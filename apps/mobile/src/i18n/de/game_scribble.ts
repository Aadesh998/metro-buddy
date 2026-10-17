import type { Messages } from '..';

const scribble: Messages['game_scribble'] = {
  round: 'Runde {n}/{max}',
  choosing: '{name} wählt ein Wort…',
  pickWord: 'Wähle ein Wort zum Zeichnen',
  drawThis: 'Zeichne:',
  drawing: '{name} zeichnet',
  youGotIt: 'Richtig erraten!',
  guessPlaceholder: 'Dein Tipp',
  chatPlaceholder: 'Chatte mit denen, die es haben',
  send: 'Tipp senden',
  guessedIt: '{name} hat es erraten!',
  soClose: '„{text}“ ist ganz nah dran!',
  privateNote: 'nur für die, die es erraten haben',
  onlyYou: 'nur du siehst das',
  wordWas: 'Das Wort war',
  noGuesses: 'Tipps erscheinen hier.',
  letters_one: '{count} Buchstabe',
  letters_other: '{count} Buchstaben',
  undo: 'Rückgängig',
  clear: 'Leeren',
  colour: 'Farbe {n}',
  brush: 'Pinselgröße {n}',
  canvas: 'Zeichenfläche',
  you: 'Du',
};

export default scribble;
