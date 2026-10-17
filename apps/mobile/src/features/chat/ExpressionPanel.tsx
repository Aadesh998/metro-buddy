import { useState } from 'react';
import { Pressable, ScrollView, View } from 'react-native';

import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';
import { EMOJI, STICKERS, stickerLabel } from './stickers';

export function StickerArt({ id, size = 120 }: { id: string; size?: number }) {
  const { t } = useT();
  const s = STICKERS.find((x) => x.id === id);
  if (!s) return null;
  return (
    <View style={{ width: size, height: size, backgroundColor: s.bg }} className="items-center justify-center gap-1 rounded-3xl p-2">
      <Text style={{ fontSize: size * 0.32, lineHeight: size * 0.4 }}>{s.emoji}</Text>
      <Text numberOfLines={2} className="text-center text-[11px] font-semibold text-white">
        {t(stickerLabel(s.id))}
      </Text>
    </View>
  );
}

// Sits where the keyboard would be: emoji go into the text box, stickers send straight away.
export function ExpressionPanel({ onEmoji, onSticker }: { onEmoji: (e: string) => void; onSticker: (id: string) => void }) {
  const { t } = useT();
  const [tab, setTab] = useState<'emoji' | 'stickers'>('emoji');
  return (
    <View className="h-64 border-t border-line bg-surface">
      <View className="flex-row gap-2 px-3 pt-2">
        {(['emoji', 'stickers'] as const).map((k) => (
          <Pressable key={k} onPress={() => setTab(k)} className={`rounded-full px-3 py-1.5 ${tab === k ? 'bg-brand/15' : ''}`}>
            <Text className={`text-sm font-medium ${tab === k ? 'text-brand' : 'text-muted'}`}>{k === 'emoji' ? t('chat.emoji') : t('chat.stickers_tab')}</Text>
          </Pressable>
        ))}
      </View>
      <ScrollView contentContainerClassName="flex-row flex-wrap px-2 py-2" keyboardShouldPersistTaps="always">
        {tab === 'emoji'
          ? EMOJI.map((e) => (
              <Pressable key={e} onPress={() => onEmoji(e)} className="h-11 w-[12.5%] items-center justify-center active:opacity-60" accessibilityLabel={e}>
                <Text className="text-2xl">{e}</Text>
              </Pressable>
            ))
          : STICKERS.map((s) => (
              <Pressable key={s.id} onPress={() => onSticker(s.id)} className="w-1/4 items-center p-1.5 active:opacity-70" accessibilityLabel={t(stickerLabel(s.id))}>
                <StickerArt id={s.id} size={76} />
              </Pressable>
            ))}
      </ScrollView>
    </View>
  );
}
