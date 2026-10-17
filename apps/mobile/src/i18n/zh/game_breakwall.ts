import type { Messages } from '..';

const game_breakwall: Messages['game_breakwall'] = {
  round: '第 {n}/{max} 轮',
  getReady: '准备',
  play: '开始！',
  roundResults: '本轮结果',
  finalStandings: '最终排名',
  score: '得分',
  balls: '球',
  time: '时间',
  dragHint: '在砖墙任意位置拖动来移动挡板',
  outOfBalls: '球用完了',
  cleared: '砖墙清空！奖励 +{n}',
  timeUp: '时间到',
  waiting: '等待其他人…',
  thisRound: '本轮',
  total: '总分',
  done: '完成',
  playing: '进行中',
  canvas: '砖墙。拖动来移动挡板。',
};

export default game_breakwall;
