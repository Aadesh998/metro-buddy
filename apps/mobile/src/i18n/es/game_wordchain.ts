import type { Messages } from '..';

const wordchain: Messages['game_wordchain'] = {
  next: 'Siguiente',
  startsWith: 'Empieza con {letter}',
  placeholder: 'Una palabra que empiece con {letter}',
  send: 'Enviar palabra',
  thinking: '{name} está pensando…',
  you: 'Tú',
  out: 'Fuera',
  livesLeft_one: 'Queda {count} vida',
  livesLeft_other: 'Quedan {count} vidas',
  attempts: 'Intento {n} de {max}',
  used: '{n}/{max} palabras',
  chain: 'La cadena',
  empty: 'Aún no hay palabras. La primera empieza con {letter}.',
  winner: '¡{name} es el último en pie!',
  youWon: '¡Eres el último en pie!',
  chainDone: 'La cadena está completa. Gana la mayor puntuación.',
  playedBy: '{word}, de {name}',
  refusal: {
    letter: 'Debe empezar con {letter}',
    used: 'Ya se usó',
    unknown: 'No está en nuestra lista',
    letters: 'Solo letras de la a a la z',
    short: 'Al menos 3 letras',
    long: 'Esa palabra es demasiado larga',
  },
};

export default wordchain;
