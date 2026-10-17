import { BadgeCheck, Mic } from 'lucide-react-native';
import { View } from 'react-native';

import type { PersonaRef } from '@/api/types';
import { Avatar } from '@/components/Avatar';
import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';
import { colors } from '@/lib/theme';

export function PersonaName({ p, size = 'sm' }: { p: Pick<PersonaRef, 'alias' | 'verified' | 'is_host'>; size?: 'sm' | 'lg' }) {
  const { t } = useT();
  const icon = size === 'lg' ? 18 : 13;
  return (
    <View className="flex-row items-center gap-1">
      <Text className={size === 'lg' ? 'text-2xl font-bold text-ink' : 'text-sm font-semibold text-ink'}>{p.alias}</Text>
      {p.verified ? <BadgeCheck color={colors.sky} size={icon} accessibilityLabel={t('bowls.persona.verified')} /> : null}
      {p.is_host ? <Mic color={colors.brand} size={icon} accessibilityLabel={t('bowls.persona.host')} /> : null}
    </View>
  );
}

export function PersonaRow({ p, subtitle }: { p: PersonaRef; subtitle?: string }) {
  return (
    <View className="flex-row items-center gap-2.5">
      <Avatar seed={p.avatar_seed} size={32} deleted={p.alias === '[deleted]'} />
      <View>
        <PersonaName p={p} />
        {subtitle ? <Text variant="caption">{subtitle}</Text> : null}
      </View>
    </View>
  );
}
