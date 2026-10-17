import type { Messages } from '..';

const game_breakwall: Messages['game_breakwall'] = {
  round: 'राउंड {n}/{max}',
  getReady: 'तैयार हो जाइए',
  play: 'शुरू!',
  roundResults: 'राउंड के नतीजे',
  finalStandings: 'अंतिम स्कोर',
  score: 'स्कोर',
  balls: 'गेंदें',
  time: 'समय',
  dragHint: 'पैडल चलाने के लिए दीवार पर कहीं भी उंगली खींचिए',
  outOfBalls: 'गेंदें ख़त्म',
  cleared: 'दीवार साफ़! +{n} बोनस',
  timeUp: 'समय ख़त्म',
  waiting: 'बाकी खिलाड़ियों का इंतज़ार…',
  thisRound: 'इस राउंड',
  total: 'कुल',
  done: 'पूरा',
  playing: 'खेल रहे हैं',
  canvas: 'ईंटों की दीवार। पैडल चलाने के लिए खींचिए।',
};
export default game_breakwall;
