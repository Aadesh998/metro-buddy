import type { Messages } from '..';

const x: Messages['game_scribble'] = {
  round: 'Manche {n}/{max}',
  choosing: '{name} choisit un mot…',
  pickWord: 'Choisis un mot à dessiner',
  drawThis: 'Dessine ceci :',
  drawing: '{name} dessine',
  youGotIt: 'Trouvé !',
  guessPlaceholder: 'Écris ta réponse',
  chatPlaceholder: 'Discute avec ceux qui ont trouvé',
  send: 'Envoyer',
  guessedIt: '{name} a trouvé !',
  soClose: '« {text} », c’est tout près !',
  privateNote: 'visible seulement par ceux qui ont trouvé',
  onlyYou: 'visible par toi seul',
  wordWas: 'Le mot était',
  noGuesses: 'Les réponses s’affichent ici.',
  letters_one: '{count} lettre',
  letters_other: '{count} lettres',
  undo: 'Annuler',
  clear: 'Effacer',
  colour: 'Couleur {n}',
  brush: 'Pinceau taille {n}',
  canvas: 'Zone de dessin',
  you: 'Toi',
};

export default x;
