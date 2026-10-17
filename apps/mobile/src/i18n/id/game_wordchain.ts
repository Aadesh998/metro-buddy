import type { Messages } from '..';

const wordchain: Messages['game_wordchain'] = {
  next: 'Berikutnya',
  startsWith: 'Berawalan {letter}',
  placeholder: 'Kata Inggris berawalan {letter}',
  send: 'Kirim kata',
  thinking: '{name} sedang berpikir…',
  you: 'Kamu',
  out: 'Gugur',
  livesLeft_one: 'Sisa {count} nyawa',
  livesLeft_other: 'Sisa {count} nyawa',
  attempts: 'Percobaan {n} dari {max}',
  used: '{n}/{max} kata',
  chain: 'Rantai kata',
  empty: 'Belum ada kata. Kata pertama berawalan {letter}.',
  winner: '{name} bertahan sampai akhir!',
  youWon: 'Kamu bertahan sampai akhir!',
  chainDone: 'Rantai selesai. Skor tertinggi menang.',
  playedBy: '{word}, dari {name}',
  refusal: {
    letter: 'Harus berawalan {letter}',
    used: 'Sudah dipakai',
    unknown: 'Tidak ada di daftar kata Inggris kami',
    letters: 'Hanya huruf a sampai z',
    short: 'Minimal 3 huruf',
    long: 'Kata itu terlalu panjang',
  },
};

export default wordchain;
