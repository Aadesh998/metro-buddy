import type { Messages } from '..';

const wordchain: Messages['game_wordchain'] = {
  next: '下一个',
  startsWith: '以 {letter} 开头',
  placeholder: '以 {letter} 开头的英文单词',
  send: '发送单词',
  thinking: '{name} 正在想…',
  you: '你',
  out: '出局',
  livesLeft_one: '剩余 {count} 条命',
  livesLeft_other: '剩余 {count} 条命',
  attempts: '第 {n}/{max} 次尝试',
  used: '{n}/{max} 个单词',
  chain: '单词链',
  empty: '还没有单词。第一个以 {letter} 开头的英文单词。',
  winner: '{name} 坚持到了最后！',
  youWon: '你坚持到了最后！',
  chainDone: '接龙完成，得分最高者获胜。',
  playedBy: '{word}，由 {name} 接出',
  refusal: {
    letter: '必须以 {letter} 开头',
    used: '已经用过了',
    unknown: '不在词表中',
    letters: '只能用字母 a 到 z',
    short: '至少 3 个字母',
    long: '单词太长了',
  },
};

export default wordchain;
