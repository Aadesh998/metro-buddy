import type { Messages } from '..';

const auth: Messages['auth'] = {
  tagline: '通勤路上的匿名伙伴。吐槽、聊天，还能在你的线路上找到旅伴。',
  google: '使用 Google 继续',
  or: '或',
  email: '邮箱',
  emailPlaceholder: 'you@example.com',
  password: '密码',
  passwordPlaceholder: '至少 8 个字符',
  confirmPassword: '确认密码',
  showPassword: '显示密码',
  hidePassword: '隐藏密码',
  logIn: '登录',
  createAccount: '创建账号',
  haveAccount: '已经有账号了？',
  noAccount: '第一次来？',
  switchToLogIn: '登录',
  switchToSignUp: '创建账号',
  passwordRule: '至少 8 个字符，需包含字母和数字。',
  mismatch: '两次输入的密码不一致',
  privateNote: '你的邮箱不会在 Metro Buddy 上向任何人显示。大家只能看到你的匿名昵称。',
  termsBefore: '仅限 18 岁以上。继续即表示你同意',
  terms: '服务条款',
  termsMiddle: '和',
  privacy: '隐私政策',
  termsAfter: '。',
};

export default auth;
