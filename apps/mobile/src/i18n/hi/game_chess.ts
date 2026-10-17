import type { Messages } from '..';

const chess: Messages['game_chess'] = {
  white: 'सफ़ेद',
  black: 'काला',
  you: 'आप',
  check: 'शह!',
  waitingFor: '{name} की चाल का इंतज़ार',
  moves: 'चालें',
  noMoves: 'पहली चाल सफ़ेद की',
  promoteTo: 'प्यादा बनेगा',
  pieces: {
    q: 'वज़ीर',
    r: 'हाथी',
    b: 'ऊँट',
    n: 'घोड़ा',
  },
  resign: 'हार मानें',
  resignTitle: 'यह गेम छोड़कर हार मानें?',
  resignBody: 'आपका विरोधी जीत जाएगा।',
  offerDraw: 'ड्रॉ का प्रस्ताव',
  drawOffered: 'ड्रॉ का प्रस्ताव भेजा',
  drawOfferFrom: '{name} ड्रॉ चाहते हैं',
  accept: 'मानें',
  decline: 'मना करें',
  result: {
    white: 'सफ़ेद जीता',
    black: 'काला जीता',
    draw: 'ड्रॉ',
  },
  reason: {
    checkmate: 'शह और मात',
    stalemate: 'स्टेलमेट',
    repetition: 'तीन बार एक ही स्थिति',
    fifty_moves: 'पचास चालों का नियम',
    insufficient_material: 'मात देने लायक मोहरे नहीं बचे',
    resign: 'हार मान ली',
    flag: 'समय ख़त्म',
    agreement: 'आपसी सहमति से ड्रॉ',
    draw: 'ड्रॉ',
  },
};

export default chess;
