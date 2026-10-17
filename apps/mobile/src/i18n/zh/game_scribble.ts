import type { Messages } from '..';

const x: Messages['game_scribble'] = {
  round: '第 {n}/{max} 轮',
  choosing: '{name} 正在选词…',
  pickWord: '选一个要画的英文单词',
  drawThis: '请画:',
  drawing: '{name} 正在画',
  youGotIt: '猜对了！',
  guessPlaceholder: '输入你猜的英文单词',
  chatPlaceholder: '和猜对的人聊天',
  send: '发送答案',
  guessedIt: '{name} 猜对了！',
  soClose: '“{text}”很接近了！',
  privateNote: '仅猜对的人可见',
  onlyYou: '仅你可见',
  wordWas: '答案是',
  noGuesses: '猜测会显示在这里。',
  letters_one: '{count} 个字母',
  letters_other: '{count} 个字母',
  undo: '撤销',
  clear: '清空',
  colour: '颜色 {n}',
  brush: '笔刷大小 {n}',
  canvas: '画布',
  you: '你',
};

export default x;
