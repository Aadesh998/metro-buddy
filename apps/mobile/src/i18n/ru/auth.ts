import type { Messages } from '..';

const auth: Messages['auth'] = {
  tagline: 'Анонимная компания в дороге на работу. Выговорись, поболтай и найди попутчика на своей линии.',
  google: 'Войти через Google',
  or: 'или',
  email: 'Email',
  emailPlaceholder: 'you@example.com',
  password: 'Пароль',
  passwordPlaceholder: 'Не меньше 8 символов',
  confirmPassword: 'Повтори пароль',
  showPassword: 'Показать пароль',
  hidePassword: 'Скрыть пароль',
  logIn: 'Войти',
  createAccount: 'Создать аккаунт',
  haveAccount: 'Уже есть аккаунт?',
  noAccount: 'Впервые здесь?',
  switchToLogIn: 'Войти',
  switchToSignUp: 'Создать аккаунт',
  passwordRule: 'Минимум 8 символов, буквы и цифры.',
  mismatch: 'Пароли не совпадают',
  privateNote: 'Твой email никто в Metro Buddy не увидит. Все видят только твой анонимный псевдоним.',
  termsBefore: 'Только 18+. Продолжая, ты принимаешь ',
  terms: 'Условия',
  termsMiddle: ' и ',
  privacy: 'Политику конфиденциальности',
  termsAfter: '.',
};

export default auth;
