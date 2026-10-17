import { Image } from 'expo-image';
import { memo } from 'react';
import Svg, { Circle, G, Path, Rect } from 'react-native-svg';

import { AVATARS, isIllustrated } from '@/lib/avatars';

const BG = ['#C9C6E6', '#EADCD4', '#CDAE9F', '#A688AB', '#8FC4BE', '#E7B4A8', '#D9CBE3', '#B9D3C6'];
const FG = ['#1A1300', '#0E1116', '#1F2630', '#2A1A00'];

function fnv1a(s: string) {
  let h = 0x811c9dc5;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return h >>> 0;
}

function rng(seed: string) {
  let a = fnv1a(seed);
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

export type AvatarParts = { bg: string; fg: string; shape: number; eyes: number; mouth: number; tilt: number };

// Pure function of the seed so every client draws the same avatar.
export function avatarParts(seed: string): AvatarParts {
  const r = rng(seed || 'deleted');
  const pick = <T,>(xs: T[]) => xs[Math.floor(r() * xs.length)];
  return {
    bg: pick(BG),
    fg: pick(FG),
    shape: Math.floor(r() * 3),
    eyes: Math.floor(r() * 3),
    mouth: Math.floor(r() * 3),
    tilt: Math.round((r() - 0.5) * 20),
  };
}

export const Avatar = memo(function Avatar({ seed, size = 40, deleted }: { seed: string; size?: number; deleted?: boolean }) {
  if (deleted || !seed) {
    return (
      <Svg width={size} height={size} viewBox="0 0 100 100">
        <Circle cx={50} cy={50} r={50} fill="#2A3340" />
        <Path d="M30 50h40" stroke="#5B6675" strokeWidth={8} strokeLinecap="round" />
      </Svg>
    );
  }
  if (isIllustrated(seed)) {
    return (
      <Image
        source={AVATARS[seed]}
        style={{ width: size, height: size, borderRadius: size / 2 }}
        contentFit="cover"
        transition={0}
        accessibilityRole="image"
      />
    );
  }
  const p = avatarParts(seed);
  const body =
    p.shape === 0 ? (
      <Circle cx={50} cy={58} r={30} fill={p.fg} opacity={0.18} />
    ) : p.shape === 1 ? (
      <Rect x={22} y={30} width={56} height={52} rx={16} fill={p.fg} opacity={0.18} />
    ) : (
      <Path d="M50 22 L82 80 H18 Z" fill={p.fg} opacity={0.18} />
    );
  const eyes =
    p.eyes === 0 ? (
      <>
        <Circle cx={38} cy={50} r={5} fill={p.fg} />
        <Circle cx={62} cy={50} r={5} fill={p.fg} />
      </>
    ) : p.eyes === 1 ? (
      <>
        <Rect x={32} y={47} width={12} height={5} rx={2.5} fill={p.fg} />
        <Rect x={56} y={47} width={12} height={5} rx={2.5} fill={p.fg} />
      </>
    ) : (
      <>
        <Circle cx={38} cy={50} r={7} fill="#fff" />
        <Circle cx={62} cy={50} r={7} fill="#fff" />
        <Circle cx={39} cy={51} r={3.5} fill={p.fg} />
        <Circle cx={63} cy={51} r={3.5} fill={p.fg} />
      </>
    );
  const mouth =
    p.mouth === 0 ? (
      <Path d="M40 64 Q50 72 60 64" stroke={p.fg} strokeWidth={4} fill="none" strokeLinecap="round" />
    ) : p.mouth === 1 ? (
      <Path d="M42 66 H58" stroke={p.fg} strokeWidth={4} strokeLinecap="round" />
    ) : (
      <Circle cx={50} cy={66} r={4} fill={p.fg} />
    );
  return (
    <Svg width={size} height={size} viewBox="0 0 100 100" accessibilityRole="image">
      <Circle cx={50} cy={50} r={50} fill={p.bg} />
      {body}
      <G rotation={p.tilt} origin="50, 50">
        {eyes}
        {mouth}
      </G>
    </Svg>
  );
});
