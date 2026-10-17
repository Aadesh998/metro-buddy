import { memo } from 'react';
import Svg, { Circle, Line, Path, Rect } from 'react-native-svg';

// Chess pieces as simple vector shapes, so they look the same on every phone
// (Unicode chess glyphs depend on a system font that some devices lack).
const WHITE = { fill: '#FAF7F3', stroke: '#1B1A19', detail: '#1B1A19' };
const BLACK = { fill: '#1B1A19', stroke: '#6E6863', detail: '#FAF7F3' };

type Kind = 'k' | 'q' | 'r' | 'b' | 'n' | 'p';

export const ChessPiece = memo(function ChessPiece({ kind, white, size }: { kind: Kind; white: boolean; size: number }) {
  const c = white ? WHITE : BLACK;
  const s = { fill: c.fill, stroke: c.stroke, strokeWidth: 3, strokeLinejoin: 'round' as const };
  const base = <Rect x={24} y={80} width={52} height={10} rx={4} {...s} />;
  let body;
  switch (kind) {
    case 'p':
      body = (
        <>
          <Path d="M40 48 H60 L68 80 H32 Z" {...s} />
          <Circle cx={50} cy={36} r={13} {...s} />
        </>
      );
      break;
    case 'r':
      body = (
        <>
          <Path d="M34 44 H66 L68 80 H32 Z" {...s} />
          <Path d="M28 18 H38 V26 H45 V18 H55 V26 H62 V18 H72 V44 H28 Z" {...s} />
        </>
      );
      break;
    case 'b':
      body = (
        <>
          <Path d="M40 60 H60 L66 80 H34 Z" {...s} />
          <Path d="M50 18 C64 26 66 44 60 60 H40 C34 44 36 26 50 18 Z" {...s} />
          <Circle cx={50} cy={14} r={5} {...s} />
          <Line x1={45} y1={42} x2={56} y2={30} stroke={c.detail} strokeWidth={3.5} strokeLinecap="round" />
        </>
      );
      break;
    case 'n':
      body = (
        <>
          <Path d="M32 80 L36 58 C30 46 34 30 48 22 L50 12 L58 22 C72 28 78 46 74 60 L64 54 L56 60 L66 80 Z" {...s} />
          <Circle cx={55} cy={33} r={3.5} fill={c.detail} />
        </>
      );
      break;
    case 'q':
      body = (
        <>
          <Path d="M24 30 L36 58 L42 24 L50 56 L58 24 L64 58 L76 30 L70 80 H30 Z" {...s} />
          {[24, 42, 58, 76].map((x) => (
            <Circle key={x} cx={x} cy={x === 24 || x === 76 ? 26 : 20} r={5} {...s} />
          ))}
        </>
      );
      break;
    case 'k':
      body = (
        <>
          <Path d="M32 80 C24 56 34 36 50 34 C66 36 76 56 68 80 Z" {...s} />
          <Rect x={46} y={6} width={8} height={26} rx={2} {...s} />
          <Rect x={39} y={13} width={22} height={8} rx={2} {...s} />
        </>
      );
      break;
  }
  return (
    <Svg width={size} height={size} viewBox="0 0 100 100">
      {body}
      {base}
    </Svg>
  );
});
