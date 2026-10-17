import type { Messages } from '..';

const settings: Messages['settings'] = {
  blocked: {
    title: 'ブロック・ミュート',
    blocked: 'ブロック中',
    muted: 'ミュート中',
    empty: '誰もいません。',
    unblock: 'ブロック解除',
    unmute: 'ミュート解除',
    explainer: 'ブロックした人は、どこでもあなたを見ることも連絡することもできません。ミュートした人はフィードに表示されなくなるだけです。',
  },
  contacts: {
    title: '緊急連絡先',
    intro: 'SOSを押すと、最後に通過した駅がわかるライブリンクがこの人たちにSMSで届きます。チャットの内容が見られることはありません。',
    remove: '{name}を削除',
    name: '名前',
    namePlaceholder: 'お姉ちゃん',
    mobile: '携帯番号',
    add: '連絡先を追加',
    failed: '失敗しました',
  },
  privacy: {
    title: 'プライバシーとデータ',
    allowed: '許可している内容',
    corePause: 'オフにするとアカウントが一時停止されます。',
  },
  consent: {
    core_service: {
      title: 'アカウントの運用',
      body: 'アプリが動くように、電話番号（暗号化）、匿名ペルソナ、投稿内容を保存します。必須です。',
    },
    travel_matching: {
      title: '通勤仲間マッチング',
      body: '通勤の駅と時間を使って、同じルートの人をおすすめします。表示されるのは移動の共通区間だけです。',
    },
    ai_processing: {
      title: 'AIによる安全チェック',
      body: 'メッセージが共有される前に、隠れた電話番号やSNSのIDがないかAIが再確認します。AIが勝手に何かを実行することはありません。',
    },
    analytics: {
      title: '匿名の利用統計',
      body: 'どの機能が使われているかを、あなたやペルソナと結びつけずに集計します。',
    },
    marketing_push: {
      title: 'お知らせ',
      body: '新機能や、あなたの街のイベントについてときどき通知します。',
    },
  },
  profile: {
    avatar: 'アバターを変更',
    avatarSaved: 'アバターを更新しました',
    title: 'プロフィールを編集',
    saved: 'プロフィールを保存しました',
    becomeTitle: '{alias}になりますか？',
    becomeBody: 'エイリアスは30日に1回変更できます。',
    change: '変更',
    aliasFailed: 'エイリアスを変更できませんでした',
    newAlias: 'ユーザー名を変更',
    aliasChanged: 'ユーザー名を変更しました',
    aliasAvailable: '{date}に新しいエイリアスに変更できます',
    bio: '自己紹介',
    hereFor: '目的',
    stations: '駅（非公開）',
    nearHome: '自宅の最寄り',
    nearWork: '職場・学校の最寄り',
    notSet: '未設定',
    vibes: '雰囲気（{count}）',
  },
  reveal: {
    title: '公開カード',
    saved: '非公開で保存しました',
    intro:
      'これらは暗号化され、非表示のまま保管されます。1対1のチャットでは、あなたと相手がそれぞれ一歩ずつ公開できます。最初は名前、次に写真、最後に連絡先。お互いが同意するまで何も表示されず、いつでも取り消せます。',
    firstName: '名前',
    firstNamePlaceholder: '下の名前だけ',
    photo: '写真',
    changePhoto: '写真を変更',
    addPhoto: '写真を追加',
    contact: '連絡先',
    contactPlaceholder: '電話番号またはSNSのID',
  },
};

export default settings;
