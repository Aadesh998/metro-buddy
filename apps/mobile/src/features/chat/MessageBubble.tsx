import { Image } from 'expo-image';
import { AlertCircle, Clock, EyeOff, MapPin, Sparkles, Timer } from 'lucide-react-native';
import { memo } from 'react';
import { Pressable, View } from 'react-native';

import type { Message } from '@/api/types';
import { Avatar } from '@/components/Avatar';
import { Text } from '@/components/ui/Text';
import { dateLocale, useT } from '@/i18n';
import { colors } from '@/lib/theme';
import { StickerArt } from './ExpressionPanel';

function time(iso: string) {
  return new Date(iso).toLocaleTimeString(dateLocale(), { hour: 'numeric', minute: '2-digit' });
}

export function sameDay(a: string, b: string) {
  return new Date(a).toDateString() === new Date(b).toDateString();
}

// A centred "Today" / "Yesterday" / "Fri, 17 Oct" pill above the first message of each day.
export function DaySeparator({ iso }: { iso: string }) {
  const { t } = useT();
  const d = new Date(iso);
  const today = new Date();
  const yesterday = new Date(today.getFullYear(), today.getMonth(), today.getDate() - 1);
  const label =
    d.toDateString() === today.toDateString()
      ? t('chat.today')
      : d.toDateString() === yesterday.toDateString()
        ? t('chat.yesterday')
        : d.toLocaleDateString(dateLocale(), { weekday: 'short', day: 'numeric', month: 'short', ...(d.getFullYear() !== today.getFullYear() && { year: 'numeric' }) });
  return (
    <View className="my-3 items-center" accessibilityRole="header">
      <View className="rounded-full bg-raised px-3 py-1">
        <Text className="text-xs font-semibold text-muted">{label}</Text>
      </View>
    </View>
  );
}

export const MessageBubble = memo(function MessageBubble({
  m,
  showSender,
  seen,
  onLongPress,
  onRetry,
  onPickOpener,
  onOpenViewOnce,
}: {
  m: Message;
  showSender: boolean;
  seen?: boolean;
  onLongPress: (m: Message) => void;
  onRetry: (m: Message) => void;
  onPickOpener?: (text: string) => void;
  onOpenViewOnce?: (m: Message) => void;
}) {
  const { t } = useT();
  if (m.kind === 'icebreaker') {
    return (
      <View className="mx-4 my-2 gap-2 rounded-2xl border border-accent/30 bg-brand/5 p-4">
        <View className="flex-row items-center gap-2">
          <Sparkles size={16} color={colors.brand} />
          <Text variant="label">{t('chat.bubble.openerTitle')}</Text>
        </View>
        {m.body.split('\n').map((line) => (
          <Pressable key={line} onPress={() => onPickOpener?.(line)} className="rounded-xl bg-raised px-3 py-2.5 active:opacity-70" accessibilityRole="button">
            <Text className="text-sm">{line}</Text>
          </Pressable>
        ))}
      </View>
    );
  }
  if (m.kind === 'system') {
    return (
      <View className="my-2 items-center px-8">
        <Text variant="caption" className="text-center">
          {m.body}
        </Text>
      </View>
    );
  }
  if (m.kind === 'meet_point') {
    return (
      <View className="mx-4 my-2 flex-row items-center gap-3 rounded-2xl border border-mint/40 bg-mint/10 p-4">
        <MapPin size={20} color={colors.mint} />
        <Text className="flex-1 leading-5">{m.body}</Text>
      </View>
    );
  }
  const mine = m.is_mine;
  if (m.kind === 'sticker' && !m.deleted) {
    return (
      <View className={`my-1 flex-row px-3 ${mine ? 'justify-end' : 'justify-start'}`}>
        {!mine ? <View className="mr-2 w-7">{showSender && m.sender ? <Avatar seed={m.sender.avatar_seed} size={28} /> : null}</View> : null}
        <Pressable onLongPress={() => onLongPress(m)} delayLongPress={300} className="items-end gap-0.5">
          <StickerArt id={m.body} />
          <Text className="text-[10px] text-faint">{time(m.created_at)}</Text>
        </Pressable>
      </View>
    );
  }
  return (
    <View className={`my-0.5 flex-row px-3 ${mine ? 'justify-end' : 'justify-start'}`}>
      {!mine ? <View className="mr-2 w-7">{showSender && m.sender ? <Avatar seed={m.sender.avatar_seed} size={28} /> : null}</View> : null}
      <Pressable onLongPress={() => onLongPress(m)} onPress={() => m.local === 'failed' && onRetry(m)} delayLongPress={300} className="max-w-[78%]">
        {showSender && !mine && m.sender ? <Text className="mb-0.5 ml-1 text-xs font-semibold text-muted">{m.sender.alias}</Text> : null}
        <View className={`rounded-2xl px-3.5 py-2 ${mine ? 'rounded-br-md bg-accent' : 'rounded-bl-md bg-raised'} ${m.local ? 'opacity-70' : ''}`}>
          {m.deleted ? (
            <Text className={`italic ${mine ? 'text-accent-ink/70' : 'text-muted'}`}>{t('chat.messageRemoved')}</Text>
          ) : (
            <>
              {m.view_once ? (
                <Pressable
                  onPress={() => !mine && !m.viewed_by_me && m.media.length > 0 && onOpenViewOnce?.(m)}
                  disabled={mine || m.viewed_by_me || m.media.length === 0}
                  className="flex-row items-center gap-2 py-1"
                  accessibilityRole="button"
                >
                  <View className={`h-7 w-7 items-center justify-center rounded-full border-2 ${mine ? 'border-accent-ink' : 'border-accent'}`}>
                    <Text className={`text-xs font-bold ${mine ? 'text-accent-ink' : 'text-brand'}`}>1</Text>
                  </View>
                  <View>
                    <Text className={`text-sm font-semibold ${mine ? 'text-accent-ink' : 'text-ink'}`}>{t('chat.viewOnce')}</Text>
                    <Text className={`text-xs ${mine ? 'text-accent-ink/70' : 'text-muted'}`}>
                      {m.viewed_by_me || m.media.length === 0 ? t('chat.opened') : mine ? t('chat.viewOnceNote') : t('chat.tapToView')}
                    </Text>
                  </View>
                </Pressable>
              ) : null}
              {(m.view_once ? [] : m.media).map((im, i) =>
                im.url && !im.pending ? (
                  <Image key={i} source={{ uri: im.url, cacheKey: im.id }} cachePolicy="memory-disk" recyclingKey={im.id} style={{ width: 220, aspectRatio: im.w / im.h, borderRadius: 12, marginBottom: 4 }} />
                ) : (
                  <Text key={i} variant="caption">
                    {t('chat.bubble.imageReview')}
                  </Text>
                ),
              )}
              {m.body ? <Text className={`text-[15px] leading-5 ${mine ? 'text-accent-ink' : 'text-ink'}`}>{m.body}</Text> : null}
            </>
          )}
          <View className="mt-0.5 flex-row items-center justify-end gap-1">
            {m.pii_hidden ? <EyeOff size={10} color={mine ? colors.accentInk : colors.faint} /> : null}
            {m.local === 'sending' ? <Clock size={10} color={colors.accentInk} /> : null}
            {m.edited_at ? <Text className={`text-[10px] ${mine ? 'text-accent-ink/60' : 'text-faint'}`}>{t('chat.edited')}</Text> : null}
            {m.expires_at ? <Timer size={10} color={mine ? colors.accentInk : colors.faint} accessibilityLabel={t('chat.bubble.disappearing')} /> : null}
            <Text className={`text-[10px] ${mine ? 'text-accent-ink/60' : 'text-faint'}`}>{time(m.created_at)}</Text>
          </View>
        </View>
        {m.local === 'failed' ? (
          <View className="mt-1 flex-row items-center justify-end gap-1">
            <AlertCircle size={12} color={colors.danger} />
            <Text className="text-xs text-danger">{t('chat.bubble.notSent')}</Text>
          </View>
        ) : null}
        {seen ? <Text className="mr-1 mt-0.5 text-right text-[11px] text-muted">{t('chat.bubble.seen')}</Text> : null}
      </Pressable>
    </View>
  );
});
