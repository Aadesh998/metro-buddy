import type { Messages } from '..';

const wordchain: Messages['game_wordchain'] = {
  next: 'Следующий',
  startsWith: 'На букву {letter}',
  placeholder: 'Английское слово на {letter}',
  send: 'Отправить слово',
  thinking: '{name} думает…',
  you: 'Ты',
  out: 'Выбыл',
  livesLeft_one: 'Жизней: {count}',
  livesLeft_other: 'Жизней: {count}',
  attempts: 'Попытка {n} из {max}',
  used: 'Слов: {n}/{max}',
  chain: 'Цепочка',
  empty: 'Слов пока нет. Первое начинается на {letter}.',
  winner: '{name} остаётся последним!',
  youWon: 'Ты остался последним!',
  chainDone: 'Цепочка собрана. Побеждает лучший счёт.',
  playedBy: '{word}, от {name}',
  refusal: {
    letter: 'Должно начинаться на {letter}',
    used: 'Уже было',
    unknown: 'Нет в нашем английском словаре',
    letters: 'Только буквы от a до z',
    short: 'Минимум 3 буквы',
    long: 'Слишком длинное слово',
  },
};

export default wordchain;
