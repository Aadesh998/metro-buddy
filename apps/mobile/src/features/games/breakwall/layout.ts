// The wall for a round, built from the server's seed exactly as the server builds it
// (server/internal/games/breakwall.go), so both agree on how many bricks there are.
//
// Checked against the Go test (breakwall_test.go):
//   mulberry32(1) → 2693262067, 11749833, 2265367787
//   breakwallLayout(12345, 0) → 5×8, 34 bricks; breakwallLayout(12345, 2) → 6×7, 31 bricks

// Same table as the server: rows, columns and how dense (percent) the wall is.
const LAYOUTS = [
  { rows: 5, cols: 8, density: 85 },
  { rows: 6, cols: 8, density: 75 },
  { rows: 6, cols: 7, density: 80 },
  { rows: 7, cols: 8, density: 65 },
];

export const BRICK_POINTS = 10;
export const CLEAR_BONUS = 50;

// Points for a run: per brick, plus a bonus and one point per second left on a full clear.
export function breakwallScore(bricks: number, total: number, secondsLeft: number) {
  return bricks * BRICK_POINTS + (bricks === total ? CLEAR_BONUS + secondsLeft : 0);
}

export function mulberry32(seed: number) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) | 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return (t ^ (t >>> 14)) >>> 0;
  };
}

// Cells (row*cols+col, ascending) that hold a brick; mirrored left to right.
export function breakwallLayout(seed: number, layout: number) {
  const n = LAYOUTS.length;
  const { rows, cols, density } = LAYOUTS[((layout % n) + n) % n];
  const next = mulberry32(seed);
  const grid: boolean[] = new Array(rows * cols).fill(false);
  const half = Math.ceil(cols / 2);
  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < half; c++) {
      const on = next() % 100 < density;
      grid[r * cols + c] = on;
      grid[r * cols + cols - 1 - c] = on;
    }
  }
  let cells = grid.flatMap((on, i) => (on ? [i] : []));
  if (cells.length === 0) cells = Array.from({ length: cols }, (_, c) => c);
  return { rows, cols, cells };
}
