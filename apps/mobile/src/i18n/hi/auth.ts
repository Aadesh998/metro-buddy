import type { Messages } from '..';

const auth: Messages['auth'] = {
  phone: {
    tagline: 'रोज़ के मेट्रो सफ़र का गुमनाम साथी। मन की बात कहें, चैट करें और अपनी लाइन पर ट्रैवल बडी ढूँढें।',
    label: 'आपका मोबाइल नंबर',
    a11y: 'मोबाइल नंबर',
    note: 'आपका नंबर सिर्फ़ लॉग इन के लिए है। Metro Buddy पर इसे कोई कभी नहीं देखता।',
    sendCode: 'कोड भेजें',
    termsBefore: 'सिर्फ़ 18+ के लिए। आगे बढ़कर आप ',
    terms: 'शर्तों',
    termsMiddle: ' और ',
    privacy: 'प्राइवेसी पॉलिसी',
    termsAfter: ' से सहमत होते हैं।',
  },
  otp: {
    title: 'कोड डालें',
    sentTo: '+91 {number} पर भेजा गया।',
    change: 'बदलें',
    a11y: 'वन-टाइम कोड',
    verify: 'वेरिफ़ाई करें',
    resendIn: '{n} सेकंड में कोड फिर से भेजें',
    resend: 'कोड फिर से भेजें',
  },
};

export default auth;
