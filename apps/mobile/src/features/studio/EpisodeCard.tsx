import { router } from 'expo-router';
import { Headphones, Radio } from 'lucide-react-native';
import { Pressable, View } from 'react-native';

import type { Episode } from '@/api/types';
import { Avatar } from '@/components/Avatar';
import { Text } from '@/components/ui/Text';
import { dateLocale, t, useT } from '@/i18n';
import { colors } from '@/lib/theme';

export function whenLabel(iso: string, now: number) {
  const d = new Date(iso);
  const mins = Math.round((d.getTime() - now) / 60000);
  if (mins <= 0) return t('studio.card.startingSoon');
  if (mins < 60) return t('studio.card.inMin', { n: mins });
  const sameDay = new Date(now).toDateString() === d.toDateString();
  const time = d.toLocaleTimeString(dateLocale(), { hour: 'numeric', minute: '2-digit' });
  return sameDay
    ? t('studio.card.today', { time })
    : t('studio.card.dateTime', { date: d.toLocaleDateString(dateLocale(), { weekday: 'short', day: 'numeric', month: 'short' }), time });
}

export function EpisodeCard({ ep, now }: { ep: Episode; now: number }) {
  const { tp } = useT();
  const live = ep.status === 'live';
  return (
    <Pressable
      onPress={() => router.push(`/episode/${ep.id}`)}
      accessibilityRole="button"
      className={`gap-3 rounded-2xl border p-4 ${live ? 'border-accent/50 bg-brand/5' : 'border-line bg-surface'}`}
    >
      <View className="flex-row items-center gap-2">
        {live ? (
          <View className="flex-row items-center gap-1.5 rounded-full bg-danger px-2 py-0.5">
            <Radio size={12} color="#fff" />
            <Text className="text-xs font-bold text-white">LIVE</Text>
          </View>
        ) : (
          <Text variant="caption" className="text-brand">
            {whenLabel(ep.scheduled_at, now)}
          </Text>
        )}
        <Text variant="caption" numberOfLines={1} className="flex-1">
          {ep.show_title}
        </Text>
        {live ? (
          <View className="flex-row items-center gap-1" accessibilityLabel={tp('studio.listening', ep.listeners)}>
            <Headphones size={14} color={colors.muted} />
            <Text variant="caption">{ep.listeners}</Text>
          </View>
        ) : null}
      </View>
      <Text variant="heading" numberOfLines={2}>
        {ep.title}
      </Text>
      <View className="flex-row items-center gap-2">
        <Avatar seed={ep.host.avatar_seed} size={22} />
        <Text variant="caption">{t('studio.card.hostedBy', { alias: ep.host.alias })}</Text>
      </View>
    </Pressable>
  );
}
