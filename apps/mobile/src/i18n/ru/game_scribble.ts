import type { Messages } from '..';

const x: Messages['game_scribble'] = {
  round: 'Раунд {n}/{max}',
  choosing: '{name} выбирает слово…',
  pickWord: 'Выбери слово для рисунка',
  drawThis: 'Нарисуй:',
  drawing: '{name} рисует',
  youGotIt: 'Угадал!',
  guessPlaceholder: 'Твой вариант (на английском)',
  chatPlaceholder: 'Чат с угадавшими',
  send: 'Отправить ответ',
  guessedIt: '{name} угадывает!',
  soClose: '«{text}» — почти!',
  privateNote: 'видят только угадавшие',
  onlyYou: 'видишь только ты',
  wordWas: 'Слово было',
  noGuesses: 'Здесь появятся ответы.',
  letters_one: 'Букв: {count}',
  letters_other: 'Букв: {count}',
  undo: 'Отменить',
  clear: 'Очистить',
  colour: 'Цвет {n}',
  brush: 'Размер кисти {n}',
  canvas: 'Холст для рисования',
  you: 'Ты',
};

export default x;
