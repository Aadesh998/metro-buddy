import type { Key } from '@/i18n';

// Must match the server's sticker list (server/internal/chat/expression.go).
export const STICKERS: { id: string; emoji: string; bg: string }[] = [
  { id: 'chai', emoji: '☕', bg: '#7A4E2D' },
  { id: 'late-again', emoji: '🚇😩', bg: '#3B4A6B' },
  { id: 'rush-hour', emoji: '🏃‍♀️🚉', bg: '#8A3B3B' },
  { id: 'window-seat', emoji: '🪟✨', bg: '#2F6B5E' },
  { id: 'mind-the-gap', emoji: '⚠️🚆', bg: '#8A6A1F' },
  { id: 'on-my-way', emoji: '🛤️💨', bg: '#33507A' },
  { id: 'same', emoji: '🙋', bg: '#6B3B7A' },
  { id: 'hug', emoji: '🤗', bg: '#7A3B5E' },
  { id: 'lol', emoji: '😂', bg: '#7A6A1F' },
  { id: 'facepalm', emoji: '🤦', bg: '#5E4A3B' },
  { id: 'party', emoji: '🥳', bg: '#3B6B7A' },
  { id: 'thank-you', emoji: '🙏', bg: '#4A6B3B' },
  { id: 'good-morning', emoji: '🌅', bg: '#8A5A2F' },
  { id: 'good-night', emoji: '🌙', bg: '#2F3B6B' },
  { id: 'monsoon', emoji: '🌧️☔', bg: '#2F5E6B' },
  { id: 'dosa', emoji: '🥞😋', bg: '#7A5A2F' },
];

export const stickerLabel = (id: string) => `chat.stickers.${id.replace(/-(\w)/g, (_, c: string) => c.toUpperCase())}` as Key;

export const EMOJI = [
  '😀', '😂', '🥲', '😊', '😍', '🥰', '😘', '😎', '🤔', '😴', '😭', '😤', '😡', '🥺', '😱', '🤯',
  '👍', '👎', '👏', '🙏', '💪', '🙌', '🤝', '👋', '✌️', '🤞', '🫶', '❤️', '💔', '🔥', '✨', '💯',
  '🎉', '🥳', '🎂', '☕', '🍵', '🍕', '🥞', '🍦', '🚇', '🚆', '🚉', '🛤️', '🏃', '🚶', '🧳', '⏰',
  '🌧️', '☀️', '🌙', '⭐', '🌈', '🎧', '🎵', '📚', '💼', '💻', '🏏', '⚽', '🎮', '🙈', '💀', '👀',
];
