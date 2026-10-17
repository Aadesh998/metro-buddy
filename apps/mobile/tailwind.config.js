/** @type {import('tailwindcss').Config} */
const token = (name) => `rgb(var(--c-${name}) / <alpha-value>)`;

module.exports = {
  content: ['./src/**/*.{js,jsx,ts,tsx}'],
  presets: [require('nativewind/preset')],
  theme: {
    extend: {
      // Values come from src/lib/theme.ts (light and dark palettes), set as CSS variables at the root.
      colors: {
        bg: token('bg'),
        surface: token('surface'),
        raised: token('raised'),
        line: token('line'),
        ink: token('ink'),
        muted: token('muted'),
        faint: token('faint'),
        accent: token('accent'),
        'accent-ink': token('accent-ink'),
        brand: token('brand'),
        'brand-soft': token('brand-soft'),
        mint: token('mint'),
        danger: token('danger'),
        sky: token('sky'),
      },
      borderRadius: {
        xl: '14px',
        '2xl': '20px',
      },
    },
  },
  plugins: [],
};
