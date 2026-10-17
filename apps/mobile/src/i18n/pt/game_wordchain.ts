import type { Messages } from '..';

const wordchain: Messages['game_wordchain'] = {
  next: 'Próximo',
  startsWith: 'Começa com {letter}',
  placeholder: 'Uma palavra com {letter}',
  send: 'Enviar palavra',
  thinking: '{name} está pensando…',
  you: 'Você',
  out: 'Fora',
  livesLeft_one: '{count} vida restante',
  livesLeft_other: '{count} vidas restantes',
  attempts: 'Tentativa {n} de {max}',
  used: '{n}/{max} palavras',
  chain: 'A corrente',
  empty: 'Nenhuma palavra ainda. A primeira começa com {letter}.',
  winner: '{name} é o último que sobrou!',
  youWon: 'Você é o último que sobrou!',
  chainDone: 'A corrente está completa. Quem fizer mais pontos vence.',
  playedBy: '{word}, jogada por {name}',
  refusal: {
    letter: 'Precisa começar com {letter}',
    used: 'Já foi usada',
    unknown: 'Não está na nossa lista',
    letters: 'Só letras de a a z',
    short: 'Pelo menos 3 letras',
    long: 'Essa palavra é longa demais',
  },
};

export default wordchain;
