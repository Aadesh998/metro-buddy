import type { Messages } from '..';

const x: Messages['game_scribble'] = {
  round: 'Ronda {n}/{max}',
  choosing: '{name} está eligiendo una palabra…',
  pickWord: 'Elige una palabra para dibujar',
  drawThis: 'Dibuja esto:',
  drawing: '{name} está dibujando',
  youGotIt: '¡Acertaste!',
  guessPlaceholder: 'Escribe tu respuesta',
  chatPlaceholder: 'Chatea con quienes acertaron',
  send: 'Enviar respuesta',
  guessedIt: '¡{name} acertó!',
  soClose: '¡“{text}” está muy cerca!',
  privateNote: 'solo lo ven quienes acertaron',
  onlyYou: 'solo tú lo ves',
  wordWas: 'La palabra era',
  noGuesses: 'Las respuestas aparecen aquí.',
  letters_one: '{count} letra',
  letters_other: '{count} letras',
  undo: 'Deshacer',
  clear: 'Borrar',
  colour: 'Color {n}',
  brush: 'Pincel tamaño {n}',
  canvas: 'Lienzo de dibujo',
  you: 'Tú',
};

export default x;
