import type { Messages } from '..';

const wordchain: Messages['game_wordchain'] = {
  next: '次',
  startsWith: '{letter}から始まる',
  placeholder: '{letter}で始まる英単語',
  send: '単語を送信',
  thinking: '{name}が考え中…',
  you: 'あなた',
  out: '脱落',
  livesLeft_one: '残りライフ {count}',
  livesLeft_other: '残りライフ {count}',
  attempts: '{n}/{max}回目',
  used: '{n}/{max}語',
  chain: 'つながった単語',
  empty: 'まだ単語がないよ。最初は{letter}で始まる英単語。',
  winner: '{name}が最後まで生き残った！',
  youWon: 'あなたが最後まで生き残った！',
  chainDone: 'しりとり完成！最高スコアの人が勝ち。',
  playedBy: '{word}（{name}）',
  refusal: {
    letter: '{letter}で始めてね',
    used: 'もう使われています',
    unknown: '単語リストにありません',
    letters: '英字 a〜z のみ',
    short: '3文字以上',
    long: '単語が長すぎます',
  },
};

export default wordchain;
