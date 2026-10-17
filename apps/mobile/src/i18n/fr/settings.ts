import type { Messages } from '..';

const settings: Messages['settings'] = {
  blocked: {
    title: 'Bloqués et masqués',
    blocked: 'Bloqués',
    muted: 'Masqués',
    empty: 'Personne ici.',
    unblock: 'Débloquer',
    unmute: 'Réafficher',
    explainer: 'Les personnes bloquées ne peuvent ni te voir ni te contacter, nulle part. Les personnes masquées disparaissent juste de tes fils.',
  },
  contacts: {
    title: 'Contacts de confiance',
    intro: 'Quand tu appuies sur SOS, ces personnes reçoivent un SMS avec un lien en direct vers la dernière station où tu es passé·e. Elles ne voient jamais tes chats.',
    remove: 'Retirer {name}',
    name: 'Nom',
    namePlaceholder: 'Didi',
    mobile: 'Portable',
    add: 'Ajouter un contact',
    failed: 'Échec',
  },
  privacy: {
    title: 'Confidentialité et données',
    allowed: 'Ce que tu as autorisé',
    corePause: 'Désactiver ceci met ton compte en pause.',
  },
  consent: {
    core_service: {
      title: 'Faire tourner mon compte',
      body: 'Stocker mon numéro (chiffré), mon profil anonyme et ce que je publie pour que l’appli fonctionne. Obligatoire.',
    },
    travel_matching: {
      title: 'Matching Travel Buddy',
      body: 'Utiliser mes stations et horaires de trajet pour suggérer des gens sur le même itinéraire. Seule la partie commune d’un trajet est montrée.',
    },
    ai_processing: {
      title: 'Vérifications de sécurité par IA',
      body: 'Laisser une IA vérifier mes messages pour repérer des numéros ou pseudos cachés avant leur envoi. Elle n’agit jamais seule.',
    },
    analytics: {
      title: 'Stats d’utilisation anonymes',
      body: 'Compter quelles fonctionnalités sont utilisées, sans lien avec moi ou mon profil.',
    },
    marketing_push: {
      title: 'Nouveautés',
      body: 'Des notifications de temps en temps sur les nouvelles fonctionnalités et les événements dans ma ville.',
    },
  },
  profile: {
    avatar: 'Changer d’avatar',
    avatarSaved: 'Avatar mis à jour',
    title: 'Modifier le profil',
    saved: 'Profil enregistré',
    becomeTitle: 'Devenir {alias} ?',
    becomeBody: 'Tu peux changer de pseudo une fois tous les 30 jours.',
    change: 'Changer',
    aliasFailed: 'Impossible de changer de pseudo',
    newAlias: 'Changer de pseudo',
    aliasChanged: 'Pseudo modifié',
    aliasAvailable: 'Nouveau pseudo possible le {date}',
    bio: 'Bio',
    hereFor: 'Là pour',
    stations: 'Stations (privé)',
    nearHome: 'Près de chez moi',
    nearWork: 'Près du boulot / de la fac',
    notSet: 'Non défini',
    vibes: 'Vibes ({count})',
  },
  reveal: {
    title: 'Carte de révélation',
    saved: 'Enregistré en privé',
    intro:
      'Ces infos restent chiffrées et cachées. Dans un chat à deux, chacun·e peut choisir de se révéler une étape à la fois : d’abord le prénom, puis la photo, puis le contact. Rien n’est montré tant que vous n’êtes pas d’accord tous les deux, et tu peux revenir en arrière quand tu veux.',
    firstName: 'Prénom',
    firstNamePlaceholder: 'Seulement ton prénom',
    photo: 'Photo',
    changePhoto: 'Changer la photo',
    addPhoto: 'Ajouter une photo',
    contact: 'Contact',
    contactPlaceholder: 'Numéro de téléphone ou pseudo de réseau social',
  },
};

export default settings;
