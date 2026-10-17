import type { Messages } from '..';

const wordchain: Messages['game_wordchain'] = {
  next: 'Suivant',
  startsWith: 'Commence par {letter}',
  placeholder: 'Un mot qui commence par {letter}',
  send: 'Envoyer le mot',
  thinking: '{name} réfléchit…',
  you: 'Toi',
  out: 'Éliminé',
  livesLeft_one: '{count} vie restante',
  livesLeft_other: '{count} vies restantes',
  attempts: 'Essai {n} sur {max}',
  used: '{n}/{max} mots',
  chain: 'La chaîne',
  empty: 'Aucun mot pour l’instant. Le premier commence par {letter}.',
  winner: '{name} est le dernier en lice !',
  youWon: 'Tu es le dernier en lice !',
  chainDone: 'La chaîne est complète. Le meilleur score gagne.',
  playedBy: '{word}, joué par {name}',
  refusal: {
    letter: 'Doit commencer par {letter}',
    used: 'Déjà utilisé',
    unknown: 'Pas dans notre liste',
    letters: 'Lettres de a à z seulement',
    short: 'Au moins 3 lettres',
    long: 'Ce mot est trop long',
  },
};

export default wordchain;
