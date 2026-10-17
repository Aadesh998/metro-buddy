import type { Messages } from '..';

const scribble: Messages['game_scribble'] = {
  round: 'Rodada {n}/{max}',
  choosing: '{name} está escolhendo uma palavra…',
  pickWord: 'Escolha uma palavra para desenhar',
  drawThis: 'Desenhe isto:',
  drawing: '{name} está desenhando',
  youGotIt: 'Você acertou!',
  guessPlaceholder: 'Digite seu palpite',
  chatPlaceholder: 'Converse com quem acertou',
  send: 'Enviar palpite',
  guessedIt: '{name} acertou!',
  soClose: '“{text}” está quase!',
  privateNote: 'só quem acertou vê isto',
  onlyYou: 'só você vê isto',
  wordWas: 'A palavra era',
  noGuesses: 'Os palpites aparecem aqui.',
  letters_one: '{count} letra',
  letters_other: '{count} letras',
  undo: 'Desfazer',
  clear: 'Limpar',
  colour: 'Cor {n}',
  brush: 'Tamanho do pincel {n}',
  canvas: 'Área de desenho',
  you: 'Você',
};

export default scribble;
