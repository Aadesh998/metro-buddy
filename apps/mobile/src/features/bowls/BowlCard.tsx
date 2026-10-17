import { router } from 'expo-router';
import { Building2, GraduationCap, Hash, MapPin, TrainFront, Users } from 'lucide-react-native';
import { Pressable, View } from 'react-native';

import type { Bowl } from '@/api/types';
import { Button } from '@/components/ui/Button';
import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';
import { colors } from '@/lib/theme';
import { useJoin } from './hooks';

export function BowlIcon({ b, size = 20 }: { b: Bowl; size?: number }) {
  const color = b.line?.color ?? colors.accent;
  const Icon = { city: Building2, line: TrainFront, station: MapPin, office_hub: Building2, college: GraduationCap, topic: Hash }[b.kind] ?? Users;
  return (
    <View className="h-10 w-10 items-center justify-center rounded-xl" style={{ backgroundColor: `${color}22` }}>
      <Icon size={size} color={color} />
    </View>
  );
}

export function BowlCard({ b }: { b: Bowl }) {
  const { t, tp } = useT();
  const join = useJoin();
  return (
    <Pressable onPress={() => router.push(`/bowl/${b.id}`)} className="flex-row items-center gap-3 rounded-2xl border border-line bg-surface p-3 active:bg-raised">
      <BowlIcon b={b} />
      <View className="flex-1">
        <Text variant="label" numberOfLines={1}>
          {b.name}
        </Text>
        <Text variant="caption" numberOfLines={1}>
          {b.reason ? `${b.reason} · ` : ''}
          {tp('bowls.members', b.member_count)}
        </Text>
      </View>
      <Button
        title={b.joined ? t('common.joined') : t('common.join')}
        kind={b.joined ? 'secondary' : 'primary'}
        className="h-9 px-4"
        onPress={() => join.mutate({ id: b.id, join: !b.joined })}
      />
    </Pressable>
  );
}
