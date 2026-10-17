import type { Messages } from '..';

const wordchain: Messages['game_wordchain'] = {
  next: 'अगला',
  startsWith: '{letter} से शुरू',
  placeholder: '{letter} से शुरू होने वाला शब्द',
  send: 'शब्द भेजें',
  thinking: '{name} सोच रहे हैं…',
  you: 'आप',
  out: 'बाहर',
  livesLeft_one: '{count} जान बाकी',
  livesLeft_other: '{count} जानें बाकी',
  attempts: 'कोशिश {n}/{max}',
  used: '{n}/{max} शब्द',
  chain: 'शब्दों की कड़ी',
  empty: 'अभी कोई शब्द नहीं। पहला शब्द {letter} से शुरू होगा।',
  winner: '{name} आख़िर तक टिके रहे!',
  youWon: 'आप आख़िर तक टिके रहे!',
  chainDone: 'कड़ी पूरी हुई। सबसे ज़्यादा अंक वाला जीता।',
  playedBy: '{word}, {name} ने खेला',
  refusal: {
    letter: '{letter} से शुरू होना चाहिए',
    used: 'पहले ही इस्तेमाल हो चुका',
    unknown: 'हमारी शब्द सूची में नहीं है',
    letters: 'सिर्फ़ a से z तक के अक्षर',
    short: 'कम से कम 3 अक्षर',
    long: 'यह शब्द बहुत लंबा है',
  },
};

export default wordchain;
