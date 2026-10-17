import { extendTailwindMerge } from 'tailwind-merge';

// Teach the merger our custom colours so `text-accent-ink` correctly overrides `text-ink`.
export const cn = extendTailwindMerge({
  extend: {
    theme: {
      colors: ['bg', 'surface', 'raised', 'line', 'ink', 'muted', 'faint', 'accent', 'accent-ink', 'mint', 'danger', 'sky', 'brand', 'brand-soft'],
    },
  },
});
