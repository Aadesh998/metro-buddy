import type { Messages } from '..';

const settings: Messages['settings'] = {
  blocked: {
    title: 'Blockiert & stumm',
    blocked: 'Blockiert',
    muted: 'Stumm',
    empty: 'Niemand hier.',
    unblock: 'Entblocken',
    unmute: 'Stumm aus',
    explainer: 'Blockierte Personen können dich nirgends sehen oder kontaktieren. Stummgeschaltete verschwinden einfach aus deinen Feeds.',
  },
  contacts: {
    title: 'Vertrauenskontakte',
    intro: 'Wenn du SOS drückst, bekommen diese Leute eine SMS mit einem Live-Link, der zeigt, welche Station du zuletzt passiert hast. Deine Chats sehen sie nie.',
    remove: '{name} entfernen',
    name: 'Name',
    namePlaceholder: 'Didi',
    mobile: 'Handy',
    add: 'Hinzufügen',
    failed: 'Fehlgeschlagen',
  },
  privacy: {
    title: 'Datenschutz & deine Daten',
    allowed: 'Was du erlaubt hast',
    corePause: 'Wenn du das ausschaltest, wird dein Konto pausiert.',
  },
  consent: {
    core_service: {
      title: 'Mein Konto betreiben',
      body: 'Meine Telefonnummer (verschlüsselt), meine anonyme Persona und meine Posts speichern, damit die App funktioniert. Erforderlich.',
    },
    travel_matching: {
      title: 'Reisebuddy-Matching',
      body: 'Meine Pendelstationen und -zeiten nutzen, um Leute auf derselben Strecke vorzuschlagen. Gezeigt wird immer nur der gemeinsame Teil der Fahrt.',
    },
    ai_processing: {
      title: 'KI-Sicherheitscheck',
      body: 'Eine KI prüft meine Nachrichten vor dem Senden auf versteckte Telefonnummern oder Handles. Sie handelt nie eigenständig.',
    },
    analytics: {
      title: 'Anonyme Nutzungsstatistik',
      body: 'Zählen, welche Funktionen genutzt werden, ohne Verbindung zu mir oder meiner Persona.',
    },
    marketing_push: {
      title: 'Produkt-Updates',
      body: 'Ab und zu Benachrichtigungen zu neuen Funktionen und Events in meiner Stadt.',
    },
  },
  profile: {
    avatar: 'Avatar ändern',
    avatarSaved: 'Avatar aktualisiert',
    title: 'Profil bearbeiten',
    saved: 'Profil gespeichert',
    becomeTitle: 'Zu {alias} werden?',
    becomeBody: 'Du kannst deinen Alias einmal alle 30 Tage ändern.',
    change: 'Ändern',
    aliasFailed: 'Alias konnte nicht geändert werden',
    newAlias: 'Username ändern',
    aliasChanged: 'Username geändert',
    aliasAvailable: 'Neuer Alias ab {date} möglich',
    bio: 'Bio',
    hereFor: 'Hier für',
    stations: 'Stationen (privat)',
    nearHome: 'Nahe Zuhause',
    nearWork: 'Nahe Arbeit / Uni',
    notSet: 'Nicht festgelegt',
    vibes: 'Vibes ({count})',
  },
  reveal: {
    title: 'Reveal-Karte',
    saved: 'Privat gespeichert',
    intro:
      'Diese Angaben bleiben verschlüsselt und verborgen. In einem Einzelchat könnt ihr beide Schritt für Schritt etwas preisgeben: erst den Vornamen, dann ein Foto, dann einen Kontakt. Nichts wird gezeigt, bis ihr beide zustimmt, und du kannst jederzeit einen Schritt zurück.',
    firstName: 'Vorname',
    firstNamePlaceholder: 'Nur dein Vorname',
    photo: 'Foto',
    changePhoto: 'Foto ändern',
    addPhoto: 'Foto hinzufügen',
    contact: 'Kontakt',
    contactPlaceholder: 'Telefonnummer oder Social-Handle',
  },
};

export default settings;
