import type { Messages } from '..';

const x: Messages['game_scribble'] = {
  round: 'ラウンド {n}/{max}',
  choosing: '{name}がお題を選んでいます…',
  pickWord: '描く英単語を選んでね',
  drawThis: 'これを描いて:',
  drawing: '{name}が描いています',
  youGotIt: '正解！',
  guessPlaceholder: '答えを英語で入力',
  chatPlaceholder: '正解した人とチャット',
  send: '回答を送信',
  guessedIt: '{name}が正解！',
  soClose: '「{text}」は惜しい！',
  privateNote: '正解者だけに表示',
  onlyYou: 'あなただけに表示',
  wordWas: '答えは',
  noGuesses: '回答はここに表示されます。',
  letters_one: '{count}文字',
  letters_other: '{count}文字',
  undo: '元に戻す',
  clear: '全消去',
  colour: '色 {n}',
  brush: 'ブラシサイズ {n}',
  canvas: 'お絵かきキャンバス',
  you: 'あなた',
};

export default x;
