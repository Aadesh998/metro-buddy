// Illustrated avatars, generated once with scripts/gen-avatars.py and bundled so they load offline.
export const AVATARS = {
  'img-f01': require('../../assets/avatars/f01.png'),
  'img-f02': require('../../assets/avatars/f02.png'),
  'img-f03': require('../../assets/avatars/f03.png'),
  'img-f04': require('../../assets/avatars/f04.png'),
  'img-f05': require('../../assets/avatars/f05.png'),
  'img-f06': require('../../assets/avatars/f06.png'),
  'img-f07': require('../../assets/avatars/f07.png'),
  'img-f08': require('../../assets/avatars/f08.png'),
  'img-f09': require('../../assets/avatars/f09.png'),
  'img-f10': require('../../assets/avatars/f10.png'),
  'img-m01': require('../../assets/avatars/m01.png'),
  'img-m02': require('../../assets/avatars/m02.png'),
  'img-m03': require('../../assets/avatars/m03.png'),
  'img-m04': require('../../assets/avatars/m04.png'),
  'img-m05': require('../../assets/avatars/m05.png'),
  'img-m06': require('../../assets/avatars/m06.png'),
  'img-m07': require('../../assets/avatars/m07.png'),
  'img-m08': require('../../assets/avatars/m08.png'),
  'img-m09': require('../../assets/avatars/m09.png'),
  'img-m10': require('../../assets/avatars/m10.png'),
} as const;

export type AvatarId = keyof typeof AVATARS;
export const AVATAR_IDS = Object.keys(AVATARS) as AvatarId[];

export const isIllustrated = (seed: string): seed is AvatarId => seed in AVATARS;
