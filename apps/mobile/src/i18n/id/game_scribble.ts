import type { Messages } from '..';

const x: Messages['game_scribble'] = {
  round: 'Ronde {n}/{max}',
  choosing: '{name} sedang memilih kata…',
  pickWord: 'Pilih kata untuk digambar',
  drawThis: 'Gambar ini:',
  drawing: '{name} sedang menggambar',
  youGotIt: 'Kamu benar!',
  guessPlaceholder: 'Tebakanmu (bahasa Inggris)',
  chatPlaceholder: 'Ngobrol dengan yang sudah benar',
  send: 'Kirim tebakan',
  guessedIt: '{name} berhasil menebak!',
  soClose: '“{text}” hampir benar!',
  privateNote: 'hanya yang sudah benar yang melihat ini',
  onlyYou: 'hanya kamu yang melihat ini',
  wordWas: 'Katanya adalah',
  noGuesses: 'Tebakan muncul di sini.',
  letters_one: '{count} huruf',
  letters_other: '{count} huruf',
  undo: 'Urungkan',
  clear: 'Hapus',
  colour: 'Warna {n}',
  brush: 'Ukuran kuas {n}',
  canvas: 'Kanvas gambar',
  you: 'Kamu',
};

export default x;
