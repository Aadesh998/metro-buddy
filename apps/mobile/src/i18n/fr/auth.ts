import type { Messages } from '..';

const auth: Messages['auth'] = {
  tagline: 'De la compagnie anonyme pour ton trajet quotidien. Vide ton sac, discute et trouve un·e partenaire de trajet sur ta ligne.',
  google: 'Continuer avec Google',
  or: 'ou',
  email: 'E-mail',
  emailPlaceholder: 'you@example.com',
  password: 'Mot de passe',
  passwordPlaceholder: 'Au moins 8 caractères',
  confirmPassword: 'Confirmer le mot de passe',
  showPassword: 'Afficher le mot de passe',
  hidePassword: 'Masquer le mot de passe',
  logIn: 'Se connecter',
  createAccount: 'Créer un compte',
  haveAccount: 'Déjà un compte ?',
  noAccount: 'Nouveau ici ?',
  switchToLogIn: 'Se connecter',
  switchToSignUp: 'Créer un compte',
  passwordRule: '8 caractères min., avec des lettres et des chiffres.',
  mismatch: 'Les mots de passe ne correspondent pas',
  privateNote: 'Ton e-mail n’est jamais montré sur Metro Buddy. Les autres voient seulement ton pseudo anonyme.',
  termsBefore: '18+ uniquement. En continuant, tu acceptes les ',
  terms: 'Conditions',
  termsMiddle: ' et la ',
  privacy: 'Politique de confidentialité',
  termsAfter: '.',
};

export default auth;
