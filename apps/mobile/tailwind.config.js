/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./src/**/*.{js,jsx,ts,tsx}'],
  presets: [require('nativewind/preset')],
  theme: {
    extend: {
      colors: {
        bg: '#0E1116',
        surface: '#161B22',
        raised: '#1F2630',
        line: '#2A3340',
        ink: '#E8EDF3',
        muted: '#8A96A6',
        faint: '#5B6675',
        accent: '#FFC83D',
        'accent-ink': '#1A1300',
        mint: '#3DDC97',
        danger: '#FF5C6C',
        sky: '#5AB0FF',
      },
      borderRadius: {
        xl: '14px',
        '2xl': '20px',
      },
    },
  },
  plugins: [],
};
