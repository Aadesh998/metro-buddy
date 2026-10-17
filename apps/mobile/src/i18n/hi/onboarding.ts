import type { Messages } from '..';

const onboarding: Messages['onboarding'] = {
  age: {
    title: 'आपकी उम्र क्या है?',
    subtitle: 'Metro Buddy सिर्फ़ बालिगों के लिए है। हम सिर्फ़ आपके जन्म का साल रखते हैं, पूरी तारीख़ कभी नहीं।',
    badDate: 'यह तारीख़ सही नहीं लग रही।',
    under18: 'Metro Buddy सिर्फ़ 18 साल और उससे ज़्यादा उम्र के लोगों के लिए है।',
    dd: 'दिन',
    mm: 'महीना',
    yyyy: 'साल',
    day: 'जन्म का दिन',
    month: 'जन्म का महीना',
    year: 'जन्म का साल',
  },
  consents: {
    title: 'आपका डेटा, आपका फ़ैसला',
    subtitle: 'चुनें कि Metro Buddy क्या कर सकता है। आप इनमें से कुछ भी बाद में सेटिंग्स में बदल सकते हैं।',
    required: 'ज़रूरी',
    readBefore: 'पूरी ',
    policy: 'प्राइवेसी पॉलिसी',
    readAfter: ' पढ़ें (वर्ज़न {version})।',
    turnOnCore: 'आगे बढ़ने के लिए “मेरा अकाउंट चलाएँ” चालू करें।',
    agree: 'सहमत हूँ, आगे बढ़ें',
  },
  persona: {
    title: 'अपनी गुमनाम पहचान चुनें',
    subtitle: 'सब आपको इसी रूप में देखेंगे। कोई असली नाम नहीं, कोई फ़ोटो नहीं।',
    choose: '{alias} चुनें',
    reroll: 'नए नाम दिखाएँ',
    hereFor: 'आप यहाँ किसलिए हैं?',
    bioLabel: 'छोटा परिचय (वैकल्पिक)',
    bioPlaceholder: 'नाइट-शिफ़्ट कोडर, कॉफ़ी से ज़्यादा चाय, येलो लाइन का रोज़ का यात्री',
    bioNote: 'फ़ोन नंबर, हैंडल और लिंक अपने आप छिप जाते हैं। {count}/160',
    continueAs: '{alias} के रूप में आगे बढ़ें',
  },
  homeMetro: {
    title: 'आपकी रोज़ की मेट्रो',
    subtitle: 'इनसे हम आपको बाउल और ट्रैवल बडी सुझाते हैं। आपका घर या ऑफ़िस वाला स्टेशन कोई कभी नहीं देखता।',
    choose: 'स्टेशन चुनें',
    nearHome: 'घर के पास का स्टेशन',
    nearWorkOptional: 'ऑफ़िस या कॉलेज के पास का स्टेशन (वैकल्पिक)',
    nearWork: 'ऑफ़िस के पास का स्टेशन',
  },
  vibes: {
    title: 'आपका वाइब क्या है?',
    subtitle: '{min} से {max} चुनें। इनसे आपको ऐसे लोग मिलते हैं जिनसे आपकी सच में बनेगी।',
    selected: '{count}/{max} चुने गए',
    finish: 'पूरा करें',
  },
  stationPicker: {
    search: 'स्टेशन खोजें',
    loading: 'स्टेशन लोड हो रहे हैं…',
    noMatch: '“{q}” से कोई स्टेशन नहीं मिला।',
    interchange: 'इंटरचेंज',
  },
  vibeCategories: {
    talk_style: 'आप कैसे बात करते हैं',
    music: 'संगीत',
    sport: 'खेल',
    hobby: 'शौक',
    work: 'काम',
    study: 'पढ़ाई',
    food: 'खाना',
    wellbeing: 'सेहत और सुकून',
  },
};

export default onboarding;
